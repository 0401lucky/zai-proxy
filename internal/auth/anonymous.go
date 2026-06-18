package auth

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/corpix/uarand"

	"zai-proxy/internal/config"
	"zai-proxy/internal/proxy"
	"zai-proxy/internal/version"
)

type AnonymousAuthResponse struct {
	Token string `json:"token"`
}

// GetAnonymousToken 从 z.ai 获取匿名 token
func GetAnonymousToken() (string, error) {
	req, err := http.NewRequest("GET", config.UpstreamBaseURL()+"/api/v1/auths/", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Origin", "https://chat.z.ai")
	req.Header.Set("Referer", "https://chat.z.ai/")
	req.Header.Set("User-Agent", uarand.GetRandom())
	req.Header.Set("X-FE-Version", version.GetFeVersion())

	resp, err := proxy.GetHTTPClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	var authResp AnonymousAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", err
	}

	return authResp.Token, nil
}
