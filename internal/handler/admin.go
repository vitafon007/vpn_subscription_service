package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/chistotel/vpn_subscription_service/internal/model"
	"github.com/chistotel/vpn_subscription_service/internal/service"
	"github.com/gin-gonic/gin"
)

// AdminHandler обрабатывает админ-эндпоинты пользователей.
type AdminHandler struct {
	admin *service.AdminUserService
}

// NewAdminHandler создаёт AdminHandler.
func NewAdminHandler(admin *service.AdminUserService) *AdminHandler {
	return &AdminHandler{admin: admin}
}

// BindUser godoc
// @Summary      Привязать клиента 3x-ui
// @Description  Ищет клиента в панели по email (login) и выдаёт subscription URL. Клиенты в 3x-ui не создаются.
// @Tags         admin
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  model.BindUserRequest  true  "Логин = email в 3x-ui"
// @Success      200   {object}  model.BindUserResponse
// @Failure      400   {object}  model.ErrorResponse
// @Failure      401   {object}  model.ErrorResponse
// @Failure      404   {object}  model.ErrorResponse
// @Failure      503   {object}  model.ErrorResponse
// @Router       /api/v1/admin/users [post]
func (h *AdminHandler) BindUser(c *gin.Context) {
	var req model.BindUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid request"})
		return
	}
	resp, err := h.admin.Bind(c.Request.Context(), req.Login)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// SetTitle godoc
// @Summary      Установить заголовок подписки
// @Description  Задаёт Profile-Title для пользователя. Для login=default пользователь создаётся при необходимости.
// @Tags         admin
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        login  path  string                true  "Логин пользователя"
// @Param        body   body  model.SetTitleRequest true  "Заголовок"
// @Success      200    {object}  model.MessageResponse
// @Failure      400    {object}  model.ErrorResponse
// @Failure      401    {object}  model.ErrorResponse
// @Failure      404    {object}  model.ErrorResponse
// @Router       /api/v1/admin/users/{login}/title [put]
func (h *AdminHandler) SetTitle(c *gin.Context) {
	login := c.Param("login")
	var req model.SetTitleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid request"})
		return
	}
	if err := h.admin.SetTitle(c.Request.Context(), login, req.Title); err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, model.MessageResponse{Message: "ok"})
}

// AddAnnounce godoc
// @Summary      Добавить анонс
// @Description  Добавляет анонс в round-robin очередь пользователя.
// @Tags         admin
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        login  path  string                   true  "Логин пользователя"
// @Param        body   body  model.AddAnnounceRequest true  "Текст анонса"
// @Success      200    {object}  model.AnnounceItem
// @Failure      400    {object}  model.ErrorResponse
// @Failure      401    {object}  model.ErrorResponse
// @Failure      404    {object}  model.ErrorResponse
// @Router       /api/v1/admin/users/{login}/announces [post]
func (h *AdminHandler) AddAnnounce(c *gin.Context) {
	login := c.Param("login")
	var req model.AddAnnounceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid request"})
		return
	}
	item, err := h.admin.AddAnnounce(c.Request.Context(), login, req.Body)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

// ListAnnounces godoc
// @Summary      Список анонсов
// @Description  Возвращает все анонсы пользователя.
// @Tags         admin
// @Security     BearerAuth
// @Produce      json
// @Param        login  path  string  true  "Логин пользователя"
// @Success      200    {object}  model.AnnounceListResponse
// @Failure      401    {object}  model.ErrorResponse
// @Failure      404    {object}  model.ErrorResponse
// @Router       /api/v1/admin/users/{login}/announces [get]
func (h *AdminHandler) ListAnnounces(c *gin.Context) {
	login := c.Param("login")
	resp, err := h.admin.ListAnnounces(c.Request.Context(), login)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteAnnounce godoc
// @Summary      Удалить анонс
// @Description  Удаляет анонс пользователя по идентификатору.
// @Tags         admin
// @Security     BearerAuth
// @Produce      json
// @Param        login  path  string  true  "Логин пользователя"
// @Param        id     path  int     true  "ID анонса"
// @Success      200    {object}  model.MessageResponse
// @Failure      400    {object}  model.ErrorResponse
// @Failure      401    {object}  model.ErrorResponse
// @Failure      404    {object}  model.ErrorResponse
// @Router       /api/v1/admin/users/{login}/announces/{id} [delete]
func (h *AdminHandler) DeleteAnnounce(c *gin.Context) {
	login := c.Param("login")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid announce id"})
		return
	}
	if err := h.admin.DeleteAnnounce(c.Request.Context(), login, id); err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, model.MessageResponse{Message: "ok"})
}

func writeAdminError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidLogin), errors.Is(err, service.ErrCannotBindDefault):
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
	case errors.Is(err, service.ErrNotFound):
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: "not found"})
	case errors.Is(err, service.ErrXUINotConfigured):
		c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Error: "xui not configured"})
	default:
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "internal error"})
	}
}
