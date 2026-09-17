package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chistotel/vpn_subscription_service/internal/config"
	"github.com/chistotel/vpn_subscription_service/internal/dao"
	"github.com/chistotel/vpn_subscription_service/internal/model"
)

var (
	// ErrInviteDisabled — лендинг не сконфигурирован.
	ErrInviteDisabled = errors.New("invite: disabled")
	// ErrInviteBadAnswer — некорректный answer.
	ErrInviteBadAnswer = errors.New("invite: bad answer")
	// ErrInviteBadEvent — пустой или слишком длинный event type.
	ErrInviteBadEvent = errors.New("invite: bad event")
)

const (
	maxVibeChips = 2
	maxFoodChips = 2
	maxNotesLen  = 500
	maxEventType = 64
)

var allowedEvents = map[string]struct{}{
	"gate_open":          {},
	"gate_redirect":      {},
	"page_open":          {},
	"heartbeat":          {},
	"visibility_hidden":  {},
	"visibility_visible": {},
	"title_card_shown":   {},
	"video_play":         {},
	"video_progress":     {},
	"video_ended":        {},
	"rsvp_yes":           {},
	"rsvp_no":            {},
	"form_start":         {},
	"form_submit":        {},
	"calendar_added":     {},
}

var allowedVibe = map[string]struct{}{
	"активный": {}, "релакс": {}, "тет-а-тет": {}, "бар": {},
	"прогулка": {}, "кино": {}, "кафе": {}, "сюрприз": {},
}

var allowedFood = map[string]struct{}{
	"роллы": {}, "пицца": {}, "паста": {}, "грузинская": {},
	"шашлык": {}, "десерт": {}, "на твой вкус": {}, "сюрприз": {},
}

var allowedMeet = map[string]struct{}{
	"я доеду": {}, "забери": {}, "решим на месте": {}, "": {},
}

// InviteService — бизнес-логика секретного приглашения.
type InviteService struct {
	dao *dao.InviteDAO
	cfg config.Config
}

// NewInviteService создаёт InviteService.
func NewInviteService(d *dao.InviteDAO, cfg config.Config) *InviteService {
	return &InviteService{dao: d, cfg: cfg}
}

// Enabled — лендинг включён.
func (s *InviteService) Enabled() bool {
	return s.cfg.InviteEnabled()
}

// Config возвращает копию конфига (для handler).
func (s *InviteService) Config() config.Config {
	return s.cfg
}

// RevealAt — момент открытия открытки.
func (s *InviteService) RevealAt() (time.Time, error) {
	return s.cfg.InviteRevealTime()
}

// IsRevealed — наступило ли время открытки.
func (s *InviteService) IsRevealed(now time.Time) (bool, time.Time, error) {
	reveal, err := s.RevealAt()
	if err != nil {
		return false, time.Time{}, err
	}
	return !now.Before(reveal), reveal, nil
}

// EnsureSession создаёт/обновляет сессию по cookie id или генерирует новый UUID.
func (s *InviteService) EnsureSession(ctx context.Context, sessionID, ua, ip string) (string, error) {
	if !s.Enabled() {
		return "", ErrInviteDisabled
	}
	id := strings.TrimSpace(sessionID)
	if id == "" || !isUUID(id) {
		var err error
		id, err = newUUID()
		if err != nil {
			return "", err
		}
	}
	_, err := s.dao.UpsertSession(ctx, id, ua, ip)
	if err != nil {
		return "", err
	}
	return id, nil
}

// RecordEvent пишет событие и обновляет last_seen.
func (s *InviteService) RecordEvent(ctx context.Context, sessionID, eventType string, payload map[string]any) error {
	if !s.Enabled() {
		return ErrInviteDisabled
	}
	eventType = strings.TrimSpace(eventType)
	if eventType == "" || len(eventType) > maxEventType {
		return ErrInviteBadEvent
	}
	if _, ok := allowedEvents[eventType]; !ok {
		return ErrInviteBadEvent
	}
	if err := s.dao.TouchSession(ctx, sessionID); err != nil {
		return err
	}
	return s.dao.InsertEvent(ctx, sessionID, eventType, payload)
}

// SaveRSVP валидирует и сохраняет ответ.
func (s *InviteService) SaveRSVP(ctx context.Context, sessionID string, req model.InviteRSVPRequest) (model.InviteRSVPResponse, error) {
	if !s.Enabled() {
		return model.InviteRSVPResponse{}, ErrInviteDisabled
	}
	answer := strings.ToLower(strings.TrimSpace(req.Answer))
	if answer != "yes" && answer != "no" {
		return model.InviteRSVPResponse{}, ErrInviteBadAnswer
	}

	vibe := filterChips(req.Vibe, allowedVibe, maxVibeChips)
	food := filterChips(req.Food, allowedFood, maxFoodChips)
	meet := strings.TrimSpace(req.Meet)
	if _, ok := allowedMeet[meet]; !ok {
		meet = ""
	}
	notes := strings.TrimSpace(req.Notes)
	if len(notes) > maxNotesLen {
		notes = notes[:maxNotesLen]
	}

	datePref := strings.TrimSpace(req.DatePref)
	timePref := strings.TrimSpace(req.TimePref)
	if answer == "no" {
		datePref, timePref, vibe, food, meet, notes = "", "", nil, nil, "", ""
	}

	row, err := s.dao.UpsertRSVP(ctx, dao.InviteRSVP{
		SessionID: sessionID,
		Answer:    answer,
		DatePref:  datePref,
		TimePref:  timePref,
		Vibe:      vibe,
		Food:      food,
		Meet:      meet,
		Notes:     notes,
	})
	if err != nil {
		return model.InviteRSVPResponse{}, err
	}

	eventType := "form_submit"
	if answer == "yes" {
		_ = s.dao.InsertEvent(ctx, sessionID, "rsvp_yes", map[string]any{
			"date_pref": datePref,
			"time_pref": timePref,
		})
	} else {
		eventType = "rsvp_no"
	}
	_ = s.dao.InsertEvent(ctx, sessionID, eventType, map[string]any{
		"answer": answer,
	})
	_ = s.dao.TouchSession(ctx, sessionID)

	return model.InviteRSVPResponse{
		Answer:    row.Answer,
		DatePref:  row.DatePref,
		TimePref:  row.TimePref,
		Vibe:      row.Vibe,
		Food:      row.Food,
		Meet:      row.Meet,
		Notes:     row.Notes,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

// GetRSVPForSession — RSVP текущей сессии.
func (s *InviteService) GetRSVPForSession(ctx context.Context, sessionID string) (dao.InviteRSVP, error) {
	return s.dao.GetRSVPBySession(ctx, sessionID)
}

// ResetAll очищает sessions/events/rsvp приглашения.
func (s *InviteService) ResetAll(ctx context.Context) (model.InviteResetResponse, error) {
	res, err := s.dao.ClearAll(ctx)
	if err != nil {
		return model.InviteResetResponse{}, err
	}
	return model.InviteResetResponse{
		Message:         "ok",
		SessionsDeleted: res.Sessions,
		EventsDeleted:   res.Events,
		RSVPDeleted:     res.RSVP,
	}, nil
}

// AdminStats собирает сводку для админ-страницы.
func (s *InviteService) AdminStats(ctx context.Context) (model.InviteAdminStats, error) {
	if !s.Enabled() {
		return model.InviteAdminStats{}, ErrInviteDisabled
	}
	reveal, err := s.RevealAt()
	if err != nil {
		return model.InviteAdminStats{}, err
	}
	loc := reveal.Location()
	now := time.Now().In(loc)
	revealed := !now.Before(reveal)

	countdownOpens, err := s.dao.CountPageOpenVariant(ctx, "countdown")
	if err != nil {
		return model.InviteAdminStats{}, err
	}
	cardOpens, err := s.dao.CountPageOpenVariant(ctx, "card")
	if err != nil {
		return model.InviteAdminStats{}, err
	}
	heartbeats, err := s.dao.CountEventsByType(ctx, "heartbeat")
	if err != nil {
		return model.InviteAdminStats{}, err
	}
	gateOpens, err := s.dao.CountEventsByType(ctx, "gate_open")
	if err != nil {
		return model.InviteAdminStats{}, err
	}
	sessions, err := s.dao.CountSessions(ctx)
	if err != nil {
		return model.InviteAdminStats{}, err
	}

	nearFrom := reveal.Add(-30 * time.Minute)
	nearCount, err := s.dao.CountEventsNearReveal(ctx, nearFrom, reveal)
	if err != nil {
		return model.InviteAdminStats{}, err
	}

	events, err := s.dao.ListEvents(ctx, 250)
	if err != nil {
		return model.InviteAdminStats{}, err
	}
	outEvents := make([]model.InviteEvent, 0, len(events))
	for _, e := range events {
		payload := map[string]any{}
		if len(e.Payload) > 0 {
			_ = json.Unmarshal(e.Payload, &payload)
		}
		outEvents = append(outEvents, model.InviteEvent{
			ID:        e.ID,
			SessionID: e.SessionID,
			Type:      e.EventType,
			Payload:   payload,
			CreatedAt: e.CreatedAt.In(loc),
		})
	}

	stats := model.InviteAdminStats{
		CountdownOpens:   countdownOpens,
		CardOpens:        cardOpens,
		HeartbeatCount:   heartbeats,
		HeartbeatMinutes: float64(heartbeats) * 0.5, // heartbeat каждые 30с
		NearReveal:       nearCount > 0,
		GateOpens:        gateOpens,
		Sessions:         sessions,
		RSVPs:            []model.InviteRSVPDTO{},
		Events:           outEvents,
		GateURL:          fmt.Sprintf("%s/%s", s.cfg.PublicBaseURL, s.cfg.InviteGateUID),
		PageURL:          fmt.Sprintf("%s/%s", s.cfg.PublicBaseURL, s.cfg.InvitePageUID),
		RevealAt:         reveal.Format("2006-01-02 15:04 MST"),
		Timezone:         s.cfg.InviteTZ,
		Now:              now.Format("2006-01-02 15:04:05 MST"),
		Revealed:         revealed,
	}

	rsvps, err := s.dao.ListRSVP(ctx, 100)
	if err != nil {
		return model.InviteAdminStats{}, err
	}
	for _, rsvp := range rsvps {
		stats.RSVPs = append(stats.RSVPs, model.InviteRSVPDTO{
			SessionID: rsvp.SessionID,
			Answer:    rsvp.Answer,
			DatePref:  rsvp.DatePref,
			TimePref:  rsvp.TimePref,
			Vibe:      rsvp.Vibe,
			Food:      rsvp.Food,
			Meet:      rsvp.Meet,
			Notes:     rsvp.Notes,
			CreatedAt: rsvp.CreatedAt.In(loc),
			UpdatedAt: rsvp.UpdatedAt.In(loc),
		})
	}

	return stats, nil
}

// BuildICS строит текст .ics для сохранённого yes-RSVP.
func (s *InviteService) BuildICS(ctx context.Context, sessionID string) (string, error) {
	rsvp, err := s.dao.GetRSVPBySession(ctx, sessionID)
	if err != nil {
		return "", err
	}
	if rsvp.Answer != "yes" {
		return "", ErrInviteBadAnswer
	}
	reveal, err := s.RevealAt()
	if err != nil {
		return "", err
	}
	loc := reveal.Location()

	start := parseDateTimePref(rsvp.DatePref, rsvp.TimePref, loc, reveal.Add(24*time.Hour))
	end := start.Add(3 * time.Hour)

	uid := fmt.Sprintf("invite-%s@vpn-invite", sessionID)
	stamp := time.Now().UTC().Format("20060102T150405Z")
	dtStart := start.Format("20060102T150405")
	dtEnd := end.Format("20060102T150405")

	desc := "Свидание"
	if len(rsvp.Vibe) > 0 {
		desc += " · " + strings.Join(rsvp.Vibe, ", ")
	}
	if len(rsvp.Food) > 0 {
		desc += " · " + strings.Join(rsvp.Food, ", ")
	}

	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//Invite//RU\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	b.WriteString("METHOD:PUBLISH\r\n")
	b.WriteString("BEGIN:VEVENT\r\n")
	b.WriteString("UID:" + uid + "\r\n")
	b.WriteString("DTSTAMP:" + stamp + "\r\n")
	b.WriteString("DTSTART;TZID=" + s.cfg.InviteTZ + ":" + dtStart + "\r\n")
	b.WriteString("DTEND;TZID=" + s.cfg.InviteTZ + ":" + dtEnd + "\r\n")
	b.WriteString("SUMMARY:Свидание\r\n")
	b.WriteString("DESCRIPTION:" + escapeICS(desc) + "\r\n")
	b.WriteString("END:VEVENT\r\n")
	b.WriteString("END:VCALENDAR\r\n")
	return b.String(), nil
}

func filterChips(in []string, allowed map[string]struct{}, max int) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, v := range in {
		v = strings.TrimSpace(strings.ToLower(v))
		// Preserve original casing from allowed keys by matching lower.
		matched := ""
		for k := range allowed {
			if strings.EqualFold(k, v) {
				matched = k
				break
			}
		}
		if matched == "" {
			continue
		}
		if _, ok := seen[matched]; ok {
			continue
		}
		seen[matched] = struct{}{}
		out = append(out, matched)
		if len(out) >= max {
			break
		}
	}
	return out
}

func parseDateTimePref(datePref, timePref string, loc *time.Location, fallback time.Time) time.Time {
	// Ожидаем date вида "2026-09-26" или "сб 26.09" / "26.09" — пробуем несколько форматов.
	datePref = strings.TrimSpace(datePref)
	timePref = strings.TrimSpace(timePref)
	hour, min := 19, 0
	if parts := strings.Split(timePref, ":"); len(parts) == 2 {
		var h, m int
		if _, err := fmt.Sscanf(timePref, "%d:%d", &h, &m); err == nil {
			hour, min = h, m
		}
	}

	year := fallback.Year()
	// "2026-09-26"
	if t, err := time.ParseInLocation("2006-01-02", datePref, loc); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), hour, min, 0, 0, loc)
	}
	// "26.09" or "26.09.2026"
	cleaned := datePref
	for _, prefix := range []string{"сб ", "вс ", "пн ", "вт ", "ср ", "чт ", "пт ", "Сб ", "Вс ", "Пт "} {
		cleaned = strings.TrimPrefix(cleaned, prefix)
	}
	cleaned = strings.TrimSpace(cleaned)
	if t, err := time.ParseInLocation("02.01.2006", cleaned, loc); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), hour, min, 0, 0, loc)
	}
	if t, err := time.ParseInLocation("02.01", cleaned, loc); err == nil {
		y := year
		cand := time.Date(y, t.Month(), t.Day(), hour, min, 0, 0, loc)
		if cand.Before(fallback.Add(-24 * time.Hour)) {
			cand = time.Date(y+1, t.Month(), t.Day(), hour, min, 0, 0, loc)
		}
		return cand
	}
	return time.Date(fallback.Year(), fallback.Month(), fallback.Day(), hour, min, 0, 0, loc)
}

func escapeICS(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, ";", `\;`)
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("uuid: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32], nil
}

func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}
