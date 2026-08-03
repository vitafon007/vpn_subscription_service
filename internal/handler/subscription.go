package handler

import (
	"errors"
	"net/http"

	"github.com/chistotel/vpn_subscription_service/internal/model"
	"github.com/chistotel/vpn_subscription_service/internal/service"
	"github.com/gin-gonic/gin"
)

// SubscriptionHandler обрабатывает публичную выдачу подписки.
type SubscriptionHandler struct {
	subs *service.SubscriptionService
}

// NewSubscriptionHandler создаёт SubscriptionHandler.
func NewSubscriptionHandler(subs *service.SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{subs: subs}
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
	result, err := h.subs.Get(c.Request.Context(), token)
	if err != nil {
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
	c.Data(http.StatusOK, "text/plain; charset=utf-8", result.Body)
}
