package zai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"zai-proxy/internal/config"
	"zai-proxy/internal/proxy"
	"zai-proxy/internal/version"
)

type Session struct {
	Token  string
	UserID string
	Name   string
	Email  string
	Role   string
}

type authResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
	Token string `json:"token"`
}

type configResponse struct {
	CompletionVersion interface{} `json:"completion_version"`
}

func ExchangeToken(ctx context.Context, token string) (Session, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Session{}, fmt.Errorf("empty token")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, config.UpstreamBaseURL()+"/api/v1/auths/", nil)
	if err != nil {
		return Session{}, err
	}
	setBrowserHeaders(req)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := proxy.GetHTTPClient().Do(req)
	if err != nil {
		return Session{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Session{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Session{}, fmt.Errorf("auth exchange failed: status=%d body=%s", resp.StatusCode, truncateBody(body))
	}

	var payload authResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return Session{}, err
	}
	if strings.TrimSpace(payload.Token) == "" || strings.TrimSpace(payload.ID) == "" {
		return Session{}, fmt.Errorf("auth exchange returned incomplete session")
	}

	return Session{
		Token:  strings.TrimSpace(payload.Token),
		UserID: strings.TrimSpace(payload.ID),
		Name:   strings.TrimSpace(payload.Name),
		Email:  strings.TrimSpace(payload.Email),
		Role:   strings.TrimSpace(payload.Role),
	}, nil
}

func CompletionVersion(ctx context.Context, sessionToken string) (int, error) {
	sessionToken = strings.TrimSpace(sessionToken)
	if sessionToken == "" {
		return 0, fmt.Errorf("empty session token")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, config.UpstreamBaseURL()+"/api/config", nil)
	if err != nil {
		return 0, err
	}
	setBrowserHeaders(req)
	req.Header.Set("Authorization", "Bearer "+sessionToken)

	resp, err := proxy.GetHTTPClient().Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("config check failed: status=%d body=%s", resp.StatusCode, truncateBody(body))
	}

	var payload configResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, err
	}
	switch value := payload.CompletionVersion.(type) {
	case float64:
		if value == 0 {
			return 1, nil
		}
		return int(value), nil
	case string:
		value = strings.TrimSpace(value)
		if value == "" {
			return 1, nil
		}
		version, err := strconv.Atoi(value)
		if err != nil {
			return 1, nil
		}
		return version, nil
	default:
		return 1, nil
	}
}

func setBrowserHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Origin", "https://chat.z.ai")
	req.Header.Set("Referer", "https://chat.z.ai/")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("X-FE-Version", version.GetFeVersion())
}

func truncateBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 500 {
		return text[:500]
	}
	return text
}
