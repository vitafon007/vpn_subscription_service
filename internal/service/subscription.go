package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/chistotel/vpn_subscription_service/internal/dao"
	"github.com/chistotel/vpn_subscription_service/internal/model"
	"github.com/chistotel/vpn_subscription_service/internal/xui"
)

// SubscriptionService собирает публичную подписку по токену.
type SubscriptionService struct {
	users     *dao.UserDAO
	titles    *dao.TitleDAO
	announces *dao.AnnounceDAO
	xui       *xui.Client
}

// NewSubscriptionService создаёт SubscriptionService.
func NewSubscriptionService(
	users *dao.UserDAO,
	titles *dao.TitleDAO,
	announces *dao.AnnounceDAO,
	xuiClient *xui.Client,
) *SubscriptionService {
	return &SubscriptionService{
		users:     users,
		titles:    titles,
		announces: announces,
		xui:       xuiClient,
	}
}

// Get возвращает тело подписки и HTTP-заголовки по токену.
func (s *SubscriptionService) Get(ctx context.Context, token string) (model.SubscriptionResult, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return model.SubscriptionResult{}, ErrNotFound
	}
	if s.xui == nil || !s.xui.Configured() {
		return model.SubscriptionResult{}, ErrXUINotConfigured
	}

	u, err := s.users.GetByToken(ctx, token)
	if errors.Is(err, dao.ErrNotFound) {
		return model.SubscriptionResult{}, ErrNotFound
	}
	if err != nil {
		return model.SubscriptionResult{}, err
	}
	if u.XUISubID == nil || *u.XUISubID == "" {
		return model.SubscriptionResult{}, ErrNotFound
	}

	links, err := s.xui.GetSubLinks(ctx, *u.XUISubID)
	if err != nil {
		if errors.Is(err, xui.ErrNotFound) {
			return model.SubscriptionResult{}, ErrNotFound
		}
		if errors.Is(err, xui.ErrNotConfigured) {
			return model.SubscriptionResult{}, ErrXUINotConfigured
		}
		return model.SubscriptionResult{}, fmt.Errorf("sub links: %w", err)
	}

	title := s.resolveTitle(ctx, u)
	announce := s.resolveAnnounce(ctx, u)
	userinfo := s.resolveUserinfo(ctx, u)

	joined := strings.Join(links, "\n")
	body := []byte(base64.StdEncoding.EncodeToString([]byte(joined)))

	return model.SubscriptionResult{
		Body:         body,
		ProfileTitle: encodeHeaderValue(title),
		Announce:     encodeHeaderValue(announce),
		Userinfo:     userinfo,
	}, nil
}

func (s *SubscriptionService) resolveTitle(ctx context.Context, u dao.User) string {
	if t, err := s.titles.GetByUserID(ctx, u.ID); err == nil && t != "" {
		return t
	}
	def, err := s.users.GetDefault(ctx)
	if err != nil {
		return ""
	}
	if def.ID == u.ID {
		return ""
	}
	t, err := s.titles.GetByUserID(ctx, def.ID)
	if err != nil {
		return ""
	}
	return t
}

func (s *SubscriptionService) resolveAnnounce(ctx context.Context, u dao.User) string {
	if a, err := s.announces.PickNextAndMarkShown(ctx, u.ID); err == nil {
		return a.Body
	}
	def, err := s.users.GetDefault(ctx)
	if err != nil {
		return ""
	}
	if def.ID == u.ID {
		return ""
	}
	a, err := s.announces.PickNextAndMarkShown(ctx, def.ID)
	if err != nil {
		return ""
	}
	return a.Body
}

func (s *SubscriptionService) resolveUserinfo(ctx context.Context, u dao.User) string {
	email := ""
	if u.XUIEmail != nil {
		email = *u.XUIEmail
	}
	if email == "" {
		email = u.Login
	}
	tr, err := s.xui.GetClientTraffic(ctx, email)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d",
		tr.Upload, tr.Download, tr.Total, tr.ExpireUnix)
}

// encodeHeaderValue кодирует текст в формат 3x-ui: base64:<std>.
func encodeHeaderValue(text string) string {
	if text == "" {
		return ""
	}
	return "base64:" + base64.StdEncoding.EncodeToString([]byte(text))
}
