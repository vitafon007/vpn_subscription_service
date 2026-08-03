// Package xui — HTTP-клиент к API панели 3x-ui.
package xui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrNotFound возвращается, когда клиент в панели не найден.
var ErrNotFound = errors.New("xui: client not found")

// ErrNotConfigured возвращается, если база URL или API-токен не заданы.
var ErrNotConfigured = errors.New("xui: not configured")

// Client — клиент REST API 3x-ui с Bearer-аутентификацией.
type Client struct {
	baseURL    string
	apiToken   string
	httpClient *http.Client
}

// New создаёт клиент 3x-ui. baseURL без завершающего слэша.
func New(baseURL, apiToken string) *Client {
	return &Client{
		baseURL:  strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiToken: strings.TrimSpace(apiToken),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Configured сообщает, можно ли выполнять запросы к панели.
func (c *Client) Configured() bool {
	return c != nil && c.baseURL != "" && c.apiToken != ""
}

// ClientInfo — краткие данные клиента панели.
type ClientInfo struct {
	// Email — email (логин) клиента в панели.
	Email string
	// SubID — идентификатор подписки (subId) в 3x-ui.
	SubID string
}

// TrafficInfo — счётчики трафика клиента.
type TrafficInfo struct {
	// Upload — исходящий трафик в байтах.
	Upload int64
	// Download — входящий трафик в байтах.
	Download int64
	// Total — лимит трафика в байтах (0 = без лимита).
	Total int64
	// ExpireUnix — срок действия в unix-секундах (0 = без срока).
	ExpireUnix int64
}

type apiResponse struct {
	Success bool            `json:"success"`
	Msg     string          `json:"msg"`
	Obj     json.RawMessage `json:"obj"`
}

// FindClientByEmail ищет клиента панели по email и возвращает subId.
// Сначала GET /panel/api/clients/get/:email, затем разбор clients в inbounds/list.
// GET …/getClientTraffics/:email используется как доп. проверка существования.
func (c *Client) FindClientByEmail(ctx context.Context, email string) (ClientInfo, error) {
	if !c.Configured() {
		return ClientInfo{}, ErrNotConfigured
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return ClientInfo{}, ErrNotFound
	}

	if info, err := c.findViaClientsGet(ctx, email); err == nil {
		return info, nil
	}

	if info, err := c.findViaInboundList(ctx, email); err == nil {
		return info, nil
	}

	// Клиент есть в traffic, но subId не найден — для bind это всё равно ошибка.
	if _, err := c.GetClientTraffic(ctx, email); err == nil {
		return ClientInfo{}, ErrNotFound
	}

	return ClientInfo{}, ErrNotFound
}

// GetSubLinks возвращает список протокол-ссылок для subId.
func (c *Client) GetSubLinks(ctx context.Context, subID string) ([]string, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	subID = strings.TrimSpace(subID)
	if subID == "" {
		return nil, ErrNotFound
	}

	path := "/panel/api/clients/subLinks/" + url.PathEscape(subID)
	var resp apiResponse
	if err := c.getJSON(ctx, path, &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		if looksMissing(resp.Msg) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("xui subLinks: %s", resp.Msg)
	}

	var links []string
	if err := json.Unmarshal(resp.Obj, &links); err != nil {
		return nil, fmt.Errorf("xui subLinks parse: %w", err)
	}
	return links, nil
}

// GetClientTraffic возвращает трафик клиента по email.
func (c *Client) GetClientTraffic(ctx context.Context, email string) (TrafficInfo, error) {
	if !c.Configured() {
		return TrafficInfo{}, ErrNotConfigured
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return TrafficInfo{}, ErrNotFound
	}

	// Современный путь clients/traffic, затем legacy inbounds/getClientTraffics.
	paths := []string{
		"/panel/api/clients/traffic/" + url.PathEscape(email),
		"/panel/api/inbounds/getClientTraffics/" + url.PathEscape(email),
	}
	var lastErr error
	for _, path := range paths {
		var resp apiResponse
		if err := c.getJSON(ctx, path, &resp); err != nil {
			lastErr = err
			continue
		}
		if !resp.Success || len(resp.Obj) == 0 || string(resp.Obj) == "null" {
			lastErr = ErrNotFound
			continue
		}
		info, err := parseTrafficObj(resp.Obj)
		if err != nil {
			lastErr = err
			continue
		}
		return info, nil
	}
	if lastErr == nil {
		lastErr = ErrNotFound
	}
	return TrafficInfo{}, lastErr
}

func (c *Client) findViaClientsGet(ctx context.Context, email string) (ClientInfo, error) {
	path := "/panel/api/clients/get/" + url.PathEscape(email)
	var resp apiResponse
	if err := c.getJSON(ctx, path, &resp); err != nil {
		return ClientInfo{}, err
	}
	if !resp.Success || len(resp.Obj) == 0 || string(resp.Obj) == "null" {
		return ClientInfo{}, ErrNotFound
	}

	var wrap struct {
		Client *struct {
			Email string `json:"email"`
			SubID string `json:"subId"`
		} `json:"client"`
		Email string `json:"email"`
		SubID string `json:"subId"`
	}
	if err := json.Unmarshal(resp.Obj, &wrap); err != nil {
		return ClientInfo{}, fmt.Errorf("xui clients/get parse: %w", err)
	}

	info := ClientInfo{Email: email}
	if wrap.Client != nil {
		if wrap.Client.Email != "" {
			info.Email = wrap.Client.Email
		}
		info.SubID = wrap.Client.SubID
	}
	if info.SubID == "" {
		info.SubID = wrap.SubID
	}
	if wrap.Email != "" {
		info.Email = wrap.Email
	}
	if info.SubID == "" {
		return ClientInfo{}, ErrNotFound
	}
	return info, nil
}

func (c *Client) findViaInboundList(ctx context.Context, email string) (ClientInfo, error) {
	var resp apiResponse
	if err := c.getJSON(ctx, "/panel/api/inbounds/list", &resp); err != nil {
		return ClientInfo{}, err
	}
	if !resp.Success {
		return ClientInfo{}, fmt.Errorf("xui inbounds/list: %s", resp.Msg)
	}

	var inbounds []struct {
		Settings string `json:"settings"`
	}
	if err := json.Unmarshal(resp.Obj, &inbounds); err != nil {
		return ClientInfo{}, fmt.Errorf("xui inbounds/list parse: %w", err)
	}

	emailLower := strings.ToLower(email)
	for _, ib := range inbounds {
		if ib.Settings == "" {
			continue
		}
		var settings struct {
			Clients []struct {
				Email string `json:"email"`
				SubID string `json:"subId"`
			} `json:"clients"`
		}
		if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
			continue
		}
		for _, cl := range settings.Clients {
			if strings.ToLower(cl.Email) == emailLower && cl.SubID != "" {
				return ClientInfo{Email: cl.Email, SubID: cl.SubID}, nil
			}
		}
	}
	return ClientInfo{}, ErrNotFound
}

func parseTrafficObj(raw json.RawMessage) (TrafficInfo, error) {
	var t struct {
		Up         int64 `json:"up"`
		Down       int64 `json:"down"`
		Total      int64 `json:"total"`
		ExpiryTime int64 `json:"expiryTime"`
	}
	if err := json.Unmarshal(raw, &t); err != nil {
		return TrafficInfo{}, fmt.Errorf("xui traffic parse: %w", err)
	}
	expire := t.ExpiryTime
	if expire > 1_000_000_000_000 { // миллисекунды → секунды
		expire = expire / 1000
	}
	return TrafficInfo{
		Upload:     t.Up,
		Download:   t.Down,
		Total:      t.Total,
		ExpireUnix: expire,
	}, nil
}

func (c *Client) getJSON(ctx context.Context, path string, dest *apiResponse) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("xui request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Accept", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("xui do: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("xui read: %w", err)
	}
	if res.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("xui http %d: %s", res.StatusCode, truncate(body, 200))
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("xui json: %w", err)
	}
	return nil
}

func looksMissing(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "not found") ||
		strings.Contains(m, "does not exist") ||
		strings.Contains(m, "no client")
}

func truncate(b []byte, n int) string {
	b = bytes.TrimSpace(b)
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
