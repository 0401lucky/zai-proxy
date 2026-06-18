package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port             string
	UpstreamBaseURL  string
	ChatEndpointPath string
}

var Cfg *Config

func LoadConfig() {
	godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	upstreamBaseURL := os.Getenv("ZAI_UPSTREAM_BASE_URL")
	if upstreamBaseURL == "" {
		upstreamBaseURL = "https://chat.z.ai"
	}
	upstreamBaseURL = strings.TrimRight(upstreamBaseURL, "/")

	chatEndpointPath := os.Getenv("ZAI_CHAT_ENDPOINT_PATH")
	if chatEndpointPath == "" {
		chatEndpointPath = "/api/v2/chat/completions"
	}
	if !strings.HasPrefix(chatEndpointPath, "/") {
		chatEndpointPath = "/" + chatEndpointPath
	}

	Cfg = &Config{
		Port:             port,
		UpstreamBaseURL:  upstreamBaseURL,
		ChatEndpointPath: chatEndpointPath,
	}
}

func UpstreamBaseURL() string {
	if Cfg == nil || Cfg.UpstreamBaseURL == "" {
		return "https://chat.z.ai"
	}
	return strings.TrimRight(Cfg.UpstreamBaseURL, "/")
}

func ChatEndpointURL() string {
	path := "/api/v2/chat/completions"
	if Cfg != nil && Cfg.ChatEndpointPath != "" {
		path = Cfg.ChatEndpointPath
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return UpstreamBaseURL() + path
}
