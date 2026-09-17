package handler

import (
	"errors"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/chistotel/vpn_subscription_service/internal/dao"
	"github.com/chistotel/vpn_subscription_service/internal/model"
	"github.com/chistotel/vpn_subscription_service/internal/service"
	inviteweb "github.com/chistotel/vpn_subscription_service/web/invite"
	"github.com/gin-gonic/gin"
)

const (
	inviteCookieName = "invite_sid"
	inviteCookieDays = 90
)

// InviteHandler — HTTP для секретного приглашения.
type InviteHandler struct {
	svc       *service.InviteService
	templates *template.Template
}

// NewInviteHandler создаёт handler и парсит embed-шаблоны.
func NewInviteHandler(svc *service.InviteService) (*InviteHandler, error) {
	tmpl, err := template.New("invite").ParseFS(inviteweb.FS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &InviteHandler{svc: svc, templates: tmpl}, nil
}

// StaticFS отдаёт CSS/JS.
func (h *InviteHandler) StaticFS() http.FileSystem {
	sub, err := fs.Sub(inviteweb.FS, "static")
	if err != nil {
		return http.FS(inviteweb.FS)
	}
	return http.FS(sub)
}

func (h *InviteHandler) ensureSession(c *gin.Context) (string, error) {
	cookie, _ := c.Cookie(inviteCookieName)
	ua := c.Request.UserAgent()
	ip := c.ClientIP()
	id, err := h.svc.EnsureSession(c.Request.Context(), cookie, ua, ip)
	if err != nil {
		return "", err
	}
	cfg := h.svc.Config()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		inviteCookieName,
		id,
		inviteCookieDays*24*3600,
		"/",
		"",
		cfg.InviteSecureCookie(),
		true,
	)
	return id, nil
}

func (h *InviteHandler) setQuietHeaders(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Robots-Tag", "noindex, nofollow")
	c.Header("Referrer-Policy", "no-referrer")
}

// Gate — QR-вход: cookie + редирект на page UID.
func (h *InviteHandler) Gate(c *gin.Context) {
	if !h.svc.Enabled() {
		c.Status(http.StatusNotFound)
		return
	}
	h.setQuietHeaders(c)
	sid, err := h.ensureSession(c)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	_ = h.svc.RecordEvent(c.Request.Context(), sid, "gate_open", map[string]any{})
	_ = h.svc.RecordEvent(c.Request.Context(), sid, "gate_redirect", map[string]any{})
	cfg := h.svc.Config()
	c.Redirect(http.StatusFound, "/"+cfg.InvitePageUID)
}

// Page — countdown или открытка.
func (h *InviteHandler) Page(c *gin.Context) {
	if !h.svc.Enabled() {
		c.Status(http.StatusNotFound)
		return
	}
	h.setQuietHeaders(c)
	if _, err := h.ensureSession(c); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	now := time.Now()
	revealed, reveal, err := h.svc.IsRevealed(now)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	cfg := h.svc.Config()
	if !revealed {
		data := map[string]any{
			"RevealUnix": reveal.Unix(),
			"ServerUnix": now.Unix(),
		}
		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html; charset=utf-8")
		if err := h.templates.ExecuteTemplate(c.Writer, "countdown.html", data); err != nil {
			_ = c.Error(err)
		}
		return
	}

	hasVideo := false
	if st, err := os.Stat(cfg.InviteVideoPath); err == nil && !st.IsDir() && st.Size() > 0 {
		hasVideo = true
	}
	// Title card только при первом показе в сессии браузера — через query ?t=1 или всегда;
	// по плану: показываем title card; клиент уберёт при reduced-motion.
	showTitle := c.Query("skip_title") != "1"
	data := map[string]any{
		"HasVideo":      hasVideo,
		"ShowTitleCard": showTitle,
		"VideoURL":      "/" + cfg.InvitePageUID + "/media/card.mp4",
	}
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(c.Writer, "card.html", data); err != nil {
		_ = c.Error(err)
	}
}

// AdminPage — секретный таймлайн.
func (h *InviteHandler) AdminPage(c *gin.Context) {
	if !h.svc.Enabled() {
		c.Status(http.StatusNotFound)
		return
	}
	h.setQuietHeaders(c)
	stats, err := h.svc.AdminStats(c.Request.Context())
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(c.Writer, "admin.html", stats); err != nil {
		_ = c.Error(err)
	}
}

// PostEvent — POST /invite/api/events.
func (h *InviteHandler) PostEvent(c *gin.Context) {
	if !h.svc.Enabled() {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: "not found"})
		return
	}
	sid, err := h.ensureSession(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "session"})
		return
	}
	var req model.InviteEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "bad request"})
		return
	}
	if err := h.svc.RecordEvent(c.Request.Context(), sid, req.Type, req.Payload); err != nil {
		if errors.Is(err, service.ErrInviteBadEvent) {
			c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "bad event"})
			return
		}
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "internal"})
		return
	}
	c.Status(http.StatusNoContent)
}

// PostRSVP — POST /invite/api/rsvp.
func (h *InviteHandler) PostRSVP(c *gin.Context) {
	if !h.svc.Enabled() {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: "not found"})
		return
	}
	sid, err := h.ensureSession(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "session"})
		return
	}
	var req model.InviteRSVPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "bad request"})
		return
	}
	out, err := h.svc.SaveRSVP(c.Request.Context(), sid, req)
	if err != nil {
		if errors.Is(err, service.ErrInviteBadAnswer) {
			c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "bad answer"})
			return
		}
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "internal"})
		return
	}
	c.JSON(http.StatusOK, out)
}

// CalendarICS — GET /invite/api/calendar.ics.
func (h *InviteHandler) CalendarICS(c *gin.Context) {
	if !h.svc.Enabled() {
		c.Status(http.StatusNotFound)
		return
	}
	sid, err := h.ensureSession(c)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	body, err := h.svc.BuildICS(c.Request.Context(), sid)
	if err != nil {
		if errors.Is(err, dao.ErrNotFound) || errors.Is(err, service.ErrInviteBadAnswer) {
			c.Status(http.StatusNotFound)
			return
		}
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Header("Content-Type", "text/calendar; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="date.ics"`)
	c.String(http.StatusOK, body)
}

// ServeVideo — GET /{pageUid}/media/card.mp4 с Range.
func (h *InviteHandler) ServeVideo(c *gin.Context) {
	if !h.svc.Enabled() {
		c.Status(http.StatusNotFound)
		return
	}
	cfg := h.svc.Config()
	path := cfg.InviteVideoPath
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		c.Status(http.StatusNotFound)
		return
	}
	// http.ServeFile поддерживает Range.
	c.Header("Content-Type", "video/mp4")
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "public, max-age=3600")
	http.ServeFile(c.Writer, c.Request, filepath.Clean(path))
}

// NotFoundQuiet — глухой 404.
func (h *InviteHandler) NotFoundQuiet(c *gin.Context) {
	c.Status(http.StatusNotFound)
}
