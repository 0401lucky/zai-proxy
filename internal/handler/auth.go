package handler

import (
	"errors"
	"net/http"
	"strings"

	"zai-proxy/internal/tokenstore"
)

func extractAPIKey(r *http.Request) string {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == "" {
		token = r.Header.Get("x-api-key")
	}
	return strings.TrimSpace(token)
}

func resolveZAIRequestToken(r *http.Request) (string, error) {
	return tokenstore.Resolve(extractAPIKey(r))
}

func writeTokenResolveError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, tokenstore.ErrMissingToken):
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	case errors.Is(err, tokenstore.ErrNoStoredToken):
		http.Error(w, "Stored z.ai token is not configured", http.StatusServiceUnavailable)
	default:
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}
}
