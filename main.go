package main

import (
	"net/http"

	"zai-proxy/internal/config"
	"zai-proxy/internal/handler"
	"zai-proxy/internal/logger"
	"zai-proxy/internal/proxy"
	"zai-proxy/internal/version"
)

func main() {
	config.LoadConfig()
	logger.InitLogger()
	proxy.LoadProxies("proxies.txt")
	version.StartVersionUpdater()

	http.HandleFunc("/v1/models", handler.HandleModels)
	http.HandleFunc("/v1/chat/completions", handler.HandleChatCompletions)
	http.HandleFunc("/v1/messages", handler.HandleMessages)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	addr := ":" + config.Cfg.Port
	logger.LogInfo("Server starting on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		logger.LogError("Server failed: %v", err)
	}
}
