package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/chistotel/vpn_subscription_service/internal/dao"
	"github.com/chistotel/vpn_subscription_service/internal/model"
	"github.com/chistotel/vpn_subscription_service/internal/xui"
)

// Ошибки сервисного слоя.
var (
	// ErrNotFound — сущность не найдена.
	ErrNotFound = errors.New("service: not found")
	// ErrXUINotConfigured — 3x-ui не сконфигурирован.
	ErrXUINotConfigured = errors.New("service: xui not configured")
	// ErrCannotBindDefault — нельзя привязать служебного пользователя default.
	ErrCannotBindDefault = errors.New("service: cannot bind default user")
	// ErrInvalidLogin — пустой или некорректный логин.
	ErrInvalidLogin = errors.New("service: invalid login")
)

// AdminUserService — админ-операции: bind, title, announces.
type AdminUserService struct {
	users     *dao.UserDAO
	titles    *dao.TitleDAO
	announces *dao.AnnounceDAO
	xui       *xui.Client
	publicURL string
}

// NewAdminUserService создаёт AdminUserService.
func NewAdminUserService(
	users *dao.UserDAO,
	titles *dao.TitleDAO,
	announces *dao.AnnounceDAO,
	xuiClient *xui.Client,
	publicBaseURL string,
) *AdminUserService {
	return &AdminUserService{
		users:     users,
		titles:    titles,
		announces: announces,
		xui:       xuiClient,
		publicURL: strings.TrimRight(publicBaseURL, "/"),
	}
}

// Bind привязывает существующего клиента 3x-ui (login = email) и возвращает subscription URL.
// Повторный bind обновляет xui_sub_id и сохраняет прежний token.
func (s *AdminUserService) Bind(ctx context.Context, login string) (model.BindUserResponse, error) {
	login = strings.TrimSpace(login)
	if login == "" {
		return model.BindUserResponse{}, ErrInvalidLogin
	}
	if login == "default" {
		return model.BindUserResponse{}, ErrCannotBindDefault
	}
	if s.xui == nil || !s.xui.Configured() {
		return model.BindUserResponse{}, ErrXUINotConfigured
	}

	info, err := s.xui.FindClientByEmail(ctx, login)
	if err != nil {
		if errors.Is(err, xui.ErrNotFound) {
			return model.BindUserResponse{}, ErrNotFound
		}
		if errors.Is(err, xui.ErrNotConfigured) {
			return model.BindUserResponse{}, ErrXUINotConfigured
		}
		return model.BindUserResponse{}, fmt.Errorf("bind find xui: %w", err)
	}

	token, err := newSubToken()
	if err != nil {
		return model.BindUserResponse{}, err
	}

	email := info.Email
	if email == "" {
		email = login
	}

	u, err := s.users.UpsertBind(ctx, login, token, info.SubID, email)
	if err != nil {
		return model.BindUserResponse{}, err
	}
	if u.SubToken == nil || *u.SubToken == "" {
		return model.BindUserResponse{}, fmt.Errorf("bind: empty sub_token after upsert")
	}

	return model.BindUserResponse{
		Login:           u.Login,
		Token:           *u.SubToken,
		SubscriptionURL: s.publicURL + "/api/v1/sub/" + *u.SubToken,
	}, nil
}

// SetTitle устанавливает Profile-Title для логина (default создаётся при отсутствии).
func (s *AdminUserService) SetTitle(ctx context.Context, login, title string) error {
	u, err := s.resolveUser(ctx, login)
	if err != nil {
		return err
	}
	return s.titles.Upsert(ctx, u.ID, title)
}

// AddAnnounce добавляет анонс пользователю.
func (s *AdminUserService) AddAnnounce(ctx context.Context, login, body string) (model.AnnounceItem, error) {
	u, err := s.resolveUser(ctx, login)
	if err != nil {
		return model.AnnounceItem{}, err
	}
	a, err := s.announces.Create(ctx, u.ID, body)
	if err != nil {
		return model.AnnounceItem{}, err
	}
	return toAnnounceItem(a), nil
}

// ListAnnounces возвращает анонсы пользователя.
func (s *AdminUserService) ListAnnounces(ctx context.Context, login string) (model.AnnounceListResponse, error) {
	u, err := s.resolveUser(ctx, login)
	if err != nil {
		return model.AnnounceListResponse{}, err
	}
	list, err := s.announces.ListByUserID(ctx, u.ID)
	if err != nil {
		return model.AnnounceListResponse{}, err
	}
	items := make([]model.AnnounceItem, 0, len(list))
	for _, a := range list {
		items = append(items, toAnnounceItem(a))
	}
	return model.AnnounceListResponse{Items: items}, nil
}

// DeleteAnnounce удаляет анонс пользователя по id.
func (s *AdminUserService) DeleteAnnounce(ctx context.Context, login string, announceID int64) error {
	u, err := s.resolveUser(ctx, login)
	if err != nil {
		return err
	}
	err = s.announces.Delete(ctx, u.ID, announceID)
	if errors.Is(err, dao.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

func (s *AdminUserService) resolveUser(ctx context.Context, login string) (dao.User, error) {
	login = strings.TrimSpace(login)
	if login == "" {
		return dao.User{}, ErrInvalidLogin
	}
	if login == "default" {
		u, err := s.users.EnsureDefault(ctx)
		if err != nil {
			return dao.User{}, err
		}
		return u, nil
	}
	u, err := s.users.GetByLogin(ctx, login)
	if errors.Is(err, dao.ErrNotFound) {
		return dao.User{}, ErrNotFound
	}
	return u, err
}

func toAnnounceItem(a dao.Announce) model.AnnounceItem {
	return model.AnnounceItem{
		ID:          a.ID,
		UserID:      a.UserID,
		Body:        a.Body,
		LastShownAt: a.LastShownAt,
		CreatedAt:   a.CreatedAt,
	}
}

func newSubToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
