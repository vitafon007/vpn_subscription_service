package handler

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/chistotel/vpn_subscription_service/internal/model"
	"github.com/chistotel/vpn_subscription_service/internal/service"
	"github.com/gin-gonic/gin"
)

// SubscriptionHandler обрабатывает публичную выдачу подписки.
type SubscriptionHandler struct {
	subs  *service.SubscriptionService
	debug bool
}

// NewSubscriptionHandler создаёт SubscriptionHandler.
// debug=true (GIN_MODE=debug) включает подробный лог ответа /api/v1/sub/:token.
func NewSubscriptionHandler(subs *service.SubscriptionService, debug bool) *SubscriptionHandler {
	return &SubscriptionHandler{subs: subs, debug: debug}
}

// GetSubscription godoc
// @Summary      Получить подписку по токену
// @Description  Возвращает base64-список ссылок и заголовки Profile-Title / Announce / Subscription-Userinfo.
// @Tags         subscription
// @Produce      plain
// @Param        token  path  string  true  "Токен подписки"
// @Success      200    {string}  string
// @Failure      404    {object}  model.ErrorResponse
// @Failure      503    {object}  model.ErrorResponse
// @Router       /api/v1/sub/{token} [get]
func (h *SubscriptionHandler) GetSubscription(c *gin.Context) {
	token := c.Param("token")

	if h.debug {
		log.Printf("sub request: GET %s token=%q headers=%s", c.Request.URL.Path, token, formatHeaderMap(c.Request.Header))
	}

	result, err := h.subs.Get(c.Request.Context(), token)
	if err != nil {
		if h.debug {
			log.Printf("sub response: token=%q error=%v", token, err)
		}
		switch {
		case errors.Is(err, service.ErrNotFound):
			c.JSON(http.StatusNotFound, model.ErrorResponse{Error: "not found"})
		case errors.Is(err, service.ErrXUINotConfigured):
			c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Error: "xui not configured"})
		default:
			c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "internal error"})
		}
		return
	}

	if result.ProfileTitle != "" {
		c.Header("Profile-Title", result.ProfileTitle)
	}
	if result.Announce != "" {
		c.Header("Announce", result.Announce)
	}
	if result.Userinfo != "" {
		c.Header("Subscription-Userinfo", result.Userinfo)
	}

	if h.debug {
		log.Printf("sub response: token=%q status=200 headers={Profile-Title:%q Announce:%q Subscription-Userinfo:%q Content-Type:%q} body=%q",
			token,
			result.ProfileTitle,
			result.Announce,
			result.Userinfo,
			"text/plain; charset=utf-8",
			truncateForLog(string(result.Body), 2000),
		)
	}

	c.Data(http.StatusOK, "text/plain; charset=utf-8", result.Body)
}

func formatHeaderMap(h http.Header) string {
	if len(h) == 0 {
		return "{}"
	}
	parts := make([]string, 0, len(h))
	for k, vals := range h {
		parts = append(parts, k+"="+strings.Join(vals, ","))
	}
	return "{" + strings.Join(parts, "; ") + "}"
}

func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
