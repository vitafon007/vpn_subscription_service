// Package xui — HTTP-клиент к API панели 3x-ui.
package xui

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrNotFound возвращается, когда клиент в панели не найден.
var ErrNotFound = errors.New("xui: client not found")

// ErrNotConfigured возвращается, если база URL или API-токен не заданы.
var ErrNotConfigured = errors.New("xui: not configured")

// ErrUnavailable возвращается при сетевой/HTTP ошибке доступа к панели.
var ErrUnavailable = errors.New("xui: unavailable")

// ErrNoSubID — клиент найден, но у него пустой subId.
var ErrNoSubID = errors.New("xui: client has empty subId")

// Client — клиент REST API 3x-ui с Bearer-аутентификацией.
type Client struct {
	baseURL    string
	apiToken   string
	httpClient *http.Client
}

// New создаёт клиент 3x-ui. baseURL без завершающего слэша
// (при кастомном webBasePath панели включайте его в URL, например https://host:6217/babaduk).
// insecureSkipVerify нужен при HTTPS на IP (сертификат на домен не совпадает с хостом).
// Если insecureSkipVerify=false, но host в URL — IP, skip включается автоматически для https.
func New(baseURL, apiToken string, insecureSkipVerify bool) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	skip := insecureSkipVerify || httpsToIP(baseURL)
	if skip && !insecureSkipVerify {
		log.Printf("xui: auto InsecureSkipVerify=true (HTTPS к IP-адресу)")
	}
	log.Printf("xui: client base=%q insecure_skip_verify=%v", baseURL, skip)

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skip, //nolint:gosec // HTTPS к docker gateway IP / явный флаг
			MinVersion:         tls.VersionTLS12,
		},
	}

	return &Client{
		baseURL:  baseURL,
		apiToken: strings.TrimSpace(apiToken),
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// httpsToIP сообщает, что URL — https://<ip>/...
func httpsToIP(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return false
	}
	host := u.Hostname()
	return net.ParseIP(host) != nil
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

// FindClientByEmail ищет клиента панели по email (поле Email в 3x-ui) и возвращает subId.
func (c *Client) FindClientByEmail(ctx context.Context, email string) (ClientInfo, error) {
	if !c.Configured() {
		return ClientInfo{}, ErrNotConfigured
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return ClientInfo{}, ErrNotFound
	}

	var sawNoSubID bool

	if info, err := c.findViaClientsGet(ctx, email); err == nil {
		return info, nil
	} else if errors.Is(err, ErrNoSubID) {
		sawNoSubID = true
	} else if errors.Is(err, ErrUnavailable) || errors.Is(err, ErrNotConfigured) {
		return ClientInfo{}, err
	}

	if info, err := c.findViaClientsList(ctx, email); err == nil {
		return info, nil
	} else if errors.Is(err, ErrNoSubID) {
		sawNoSubID = true
	} else if errors.Is(err, ErrUnavailable) {
		return ClientInfo{}, err
	}

	if info, err := c.findViaClientsPaged(ctx, email); err == nil {
		return info, nil
	} else if errors.Is(err, ErrNoSubID) {
		sawNoSubID = true
	} else if errors.Is(err, ErrUnavailable) {
		return ClientInfo{}, err
	}

	if info, err := c.findViaInboundList(ctx, email); err == nil {
		return info, nil
	} else if errors.Is(err, ErrUnavailable) {
		return ClientInfo{}, err
	}

	// ClientTraffic тоже содержит subId — последний шанс.
	if info, err := c.findViaTraffic(ctx, email); err == nil {
		return info, nil
	}

	if sawNoSubID {
		return ClientInfo{}, ErrNoSubID
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

	path := "/panel/api/clients/subLinks/" + pathEscape(subID)
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

	paths := []string{
		"/panel/api/clients/traffic/" + pathEscape(email),
		"/panel/api/inbounds/getClientTraffics/" + pathEscape(email),
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
	path := "/panel/api/clients/get/" + pathEscape(email)
	var resp apiResponse
	if err := c.getJSON(ctx, path, &resp); err != nil {
		return ClientInfo{}, err
	}
	if !resp.Success || len(resp.Obj) == 0 || string(resp.Obj) == "null" {
		return ClientInfo{}, ErrNotFound
	}
	return parseClientObj(resp.Obj, email)
}

func (c *Client) findViaClientsList(ctx context.Context, email string) (ClientInfo, error) {
	var resp apiResponse
	if err := c.getJSON(ctx, "/panel/api/clients/list", &resp); err != nil {
		return ClientInfo{}, err
	}
	if !resp.Success {
		return ClientInfo{}, ErrNotFound
	}

	var items []map[string]any
	if err := json.Unmarshal(resp.Obj, &items); err != nil {
		return ClientInfo{}, fmt.Errorf("xui clients/list parse: %w", err)
	}

	emailLower := strings.ToLower(email)
	var foundEmptySub bool
	for _, item := range items {
		em, _ := item["email"].(string)
		if strings.ToLower(em) != emailLower {
			continue
		}
		sub, _ := item["subId"].(string)
		if strings.TrimSpace(sub) == "" {
			foundEmptySub = true
			continue
		}
		return ClientInfo{Email: em, SubID: sub}, nil
	}
	if foundEmptySub {
		return ClientInfo{}, ErrNoSubID
	}
	return ClientInfo{}, ErrNotFound
}

func (c *Client) findViaClientsPaged(ctx context.Context, email string) (ClientInfo, error) {
	q := url.Values{}
	q.Set("page", "1")
	q.Set("pageSize", "50")
	q.Set("search", email)
	path := "/panel/api/clients/list/paged?" + q.Encode()

	var resp apiResponse
	if err := c.getJSON(ctx, path, &resp); err != nil {
		return ClientInfo{}, err
	}
	if !resp.Success {
		return ClientInfo{}, ErrNotFound
	}

	var page struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(resp.Obj, &page); err != nil {
		return ClientInfo{}, fmt.Errorf("xui clients/paged parse: %w", err)
	}

	emailLower := strings.ToLower(email)
	var foundEmptySub bool
	for _, item := range page.Items {
		em, _ := item["email"].(string)
		if strings.ToLower(em) != emailLower {
			continue
		}
		sub, _ := item["subId"].(string)
		if strings.TrimSpace(sub) == "" {
			foundEmptySub = true
			continue
		}
		return ClientInfo{Email: em, SubID: sub}, nil
	}
	if foundEmptySub {
		return ClientInfo{}, ErrNoSubID
	}
	return ClientInfo{}, ErrNotFound
}

func (c *Client) findViaInboundList(ctx context.Context, email string) (ClientInfo, error) {
	var resp apiResponse
	if err := c.getJSON(ctx, "/panel/api/inbounds/list", &resp); err != nil {
		return ClientInfo{}, err
	}
	if !resp.Success {
		return ClientInfo{}, fmt.Errorf("%w: inbounds/list: %s", ErrUnavailable, resp.Msg)
	}

	// В актуальном API settings — JSON-объект; legacy мог отдавать JSON-строку.
	var inbounds []struct {
		Settings json.RawMessage `json:"settings"`
	}
	if err := json.Unmarshal(resp.Obj, &inbounds); err != nil {
		return ClientInfo{}, fmt.Errorf("xui inbounds/list parse: %w", err)
	}

	emailLower := strings.ToLower(email)
	var foundEmptySub bool
	for _, ib := range inbounds {
		settings, ok := parseInboundSettings(ib.Settings)
		if !ok {
			continue
		}
		for _, cl := range settings.Clients {
			if strings.ToLower(cl.Email) != emailLower {
				continue
			}
			if strings.TrimSpace(cl.SubID) == "" {
				foundEmptySub = true
				continue
			}
			return ClientInfo{Email: cl.Email, SubID: cl.SubID}, nil
		}
	}
	if foundEmptySub {
		return ClientInfo{}, ErrNoSubID
	}
	return ClientInfo{}, ErrNotFound
}

type inboundSettingsClients struct {
	Clients []struct {
		Email string `json:"email"`
		SubID string `json:"subId"`
	} `json:"clients"`
}

// parseInboundSettings принимает settings как объект или как JSON-encoded string.
func parseInboundSettings(raw json.RawMessage) (inboundSettingsClients, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return inboundSettingsClients{}, false
	}
	var settings inboundSettingsClients
	if err := json.Unmarshal(raw, &settings); err == nil {
		return settings, true
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err != nil || asString == "" {
		return inboundSettingsClients{}, false
	}
	if err := json.Unmarshal([]byte(asString), &settings); err != nil {
		return inboundSettingsClients{}, false
	}
	return settings, true
}

func parseClientObj(raw json.RawMessage, fallbackEmail string) (ClientInfo, error) {
	var wrap struct {
		Client *struct {
			Email string `json:"email"`
			SubID string `json:"subId"`
		} `json:"client"`
		Email string `json:"email"`
		SubID string `json:"subId"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return ClientInfo{}, fmt.Errorf("xui clients/get parse: %w", err)
	}

	info := ClientInfo{Email: fallbackEmail}
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
	if strings.TrimSpace(info.SubID) == "" {
		return ClientInfo{}, ErrNoSubID
	}
	return info, nil
}

func parseTrafficObj(raw json.RawMessage) (TrafficInfo, error) {
	var t struct {
		Up         int64  `json:"up"`
		Down       int64  `json:"down"`
		Total      int64  `json:"total"`
		ExpiryTime int64  `json:"expiryTime"`
		Email      string `json:"email"`
		SubID      string `json:"subId"`
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

func (c *Client) findViaTraffic(ctx context.Context, email string) (ClientInfo, error) {
	paths := []string{
		"/panel/api/clients/traffic/" + pathEscape(email),
		"/panel/api/inbounds/getClientTraffics/" + pathEscape(email),
	}
	for _, path := range paths {
		var resp apiResponse
		if err := c.getJSON(ctx, path, &resp); err != nil {
			continue
		}
		if !resp.Success || len(resp.Obj) == 0 || string(resp.Obj) == "null" {
			continue
		}
		var t struct {
			Email string `json:"email"`
			SubID string `json:"subId"`
		}
		if err := json.Unmarshal(resp.Obj, &t); err != nil {
			continue
		}
		if strings.TrimSpace(t.SubID) == "" {
			return ClientInfo{}, ErrNoSubID
		}
		em := t.Email
		if em == "" {
			em = email
		}
		return ClientInfo{Email: em, SubID: t.SubID}, nil
	}
	return ClientInfo{}, ErrNotFound
}

func (c *Client) getJSON(ctx context.Context, path string, dest *apiResponse) error {
	fullURL := c.baseURL + path
	log.Printf("xui request: GET %s", fullURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		log.Printf("xui result: GET %s build error: %v", fullURL, err)
		return fmt.Errorf("%w: build request: %v", ErrUnavailable, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Accept", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("xui result: GET %s transport error: %v", fullURL, err)
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		log.Printf("xui result: GET %s read error: %v", fullURL, err)
		return fmt.Errorf("%w: read: %v", ErrUnavailable, err)
	}

	// Не JSON / HTML 404 панели (часто неверный XUI_BASE_URL или webBasePath).
	ct := res.Header.Get("Content-Type")
	if res.StatusCode == http.StatusNotFound && !strings.Contains(ct, "json") {
		log.Printf("xui result: GET %s status=%d ct=%q body=%q", fullURL, res.StatusCode, ct, truncate(body, 200))
		return fmt.Errorf("%w: http 404 (проверьте XUI_BASE_URL и webBasePath панели)", ErrUnavailable)
	}
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		log.Printf("xui result: GET %s status=%d (auth)", fullURL, res.StatusCode)
		return fmt.Errorf("%w: http %d (проверьте XUI_API_TOKEN)", ErrUnavailable, res.StatusCode)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		log.Printf("xui result: GET %s status=%d body=%q", fullURL, res.StatusCode, truncate(body, 200))
		if res.StatusCode == http.StatusNotFound {
			return ErrNotFound
		}
		return fmt.Errorf("%w: http %d: %s", ErrUnavailable, res.StatusCode, truncate(body, 200))
	}
	if err := json.Unmarshal(body, dest); err != nil {
		log.Printf("xui result: GET %s status=%d json error: %v body=%q", fullURL, res.StatusCode, err, truncate(body, 200))
		return fmt.Errorf("%w: json: %v", ErrUnavailable, err)
	}
	log.Printf("xui result: GET %s status=%d success=%v msg=%q obj=%q",
		fullURL, res.StatusCode, dest.Success, dest.Msg, truncate(dest.Obj, 300))
	return nil
}

func pathEscape(s string) string {
	return url.PathEscape(s)
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
