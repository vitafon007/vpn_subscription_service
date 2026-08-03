package model

import "time"

// BindUserRequest — тело запроса привязки пользователя к клиенту 3x-ui.
type BindUserRequest struct {
	// Login — email клиента в панели 3x-ui.
	Login string `json:"login" binding:"required" example:"user@example.com"`
}

// BindUserResponse — ответ после успешной привязки.
type BindUserResponse struct {
	// Login — логин пользователя.
	Login string `json:"login" example:"user@example.com"`
	// Token — токен публичной подписки.
	Token string `json:"token" example:"a1b2c3…"`
	// SubscriptionURL — полный URL подписки.
	SubscriptionURL string `json:"subscription_url" example:"http://127.0.0.1:23452/api/v1/sub/a1b2c3"`
}

// SetTitleRequest — тело запроса установки заголовка подписки.
type SetTitleRequest struct {
	// Title — текст Profile-Title.
	Title string `json:"title" binding:"required" example:"My VPN"`
}

// MessageResponse — простой ответ с сообщением.
type MessageResponse struct {
	// Message — текст результата.
	Message string `json:"message" example:"ok"`
}

// AddAnnounceRequest — тело запроса добавления анонса.
type AddAnnounceRequest struct {
	// Body — текст анонса.
	Body string `json:"body" binding:"required" example:"Обновление серверов"`
}

// AnnounceItem — анонс пользователя.
type AnnounceItem struct {
	// ID — идентификатор анонса.
	ID int64 `json:"id" example:"1"`
	// UserID — идентификатор пользователя.
	UserID int64 `json:"user_id" example:"1"`
	// Body — текст анонса.
	Body string `json:"body" example:"Обновление серверов"`
	// LastShownAt — время последней выдачи (null если ещё не показывали).
	LastShownAt *time.Time `json:"last_shown_at,omitempty"`
	// CreatedAt — время создания.
	CreatedAt time.Time `json:"created_at"`
}

// AnnounceListResponse — список анонсов.
type AnnounceListResponse struct {
	// Items — анонсы пользователя.
	Items []AnnounceItem `json:"items"`
}

// SubscriptionResult — собранная подписка для HTTP-ответа (не JSON API).
type SubscriptionResult struct {
	// Body — тело ответа (base64 списка ссылок).
	Body []byte
	// ProfileTitle — значение заголовка Profile-Title (уже с префиксом base64: или пусто).
	ProfileTitle string
	// Announce — значение заголовка Announce (уже с префиксом base64: или пусто).
	Announce string
	// Userinfo — значение Subscription-Userinfo или пусто.
	Userinfo string
}
