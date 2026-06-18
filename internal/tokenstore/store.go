package tokenstore

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

var (
	ErrMissingToken  = errors.New("missing api key")
	ErrMissingKey    = errors.New("missing proxy key")
	ErrNoStoredToken = errors.New("stored z.ai token is not configured")
	ErrAdminDisabled = errors.New("admin token api is disabled")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrTokenNotFound = errors.New("token key not found")
	tokenByKey       = map[string]string{}
	sourceByKey      = map[string]string{}
	legacyProxyKey   string
	adminAPIKey      string
	singleTokenFile  string
	tokenMapFile     string
	storeLock        sync.RWMutex
)

type Status struct {
	Configured       bool        `json:"configured"`
	Count            int         `json:"count"`
	AdminEnabled     bool        `json:"admin_enabled"`
	LegacyProxyKey   string      `json:"legacy_proxy_key_preview,omitempty"`
	TokenFile        string      `json:"token_file,omitempty"`
	TokenMapFile     string      `json:"token_map_file,omitempty"`
	Tokens           []TokenInfo `json:"tokens"`
	DeprecatedSource string      `json:"source,omitempty"`
}

type TokenInfo struct {
	Key          string `json:"key"`
	Source       string `json:"source"`
	TokenPreview string `json:"token_preview,omitempty"`
}

func Init(initialToken, filePath, apiKey, tokenMap, mapFile, adminKey string) error {
	storeLock.Lock()
	defer storeLock.Unlock()

	tokenByKey = map[string]string{}
	sourceByKey = map[string]string{}
	legacyProxyKey = strings.TrimSpace(apiKey)
	adminAPIKey = strings.TrimSpace(adminKey)
	singleTokenFile = strings.TrimSpace(filePath)
	tokenMapFile = strings.TrimSpace(mapFile)

	if err := loadEnvTokenMapLocked(tokenMap); err != nil {
		return err
	}
	loadLegacyTokenLocked(strings.TrimSpace(initialToken), "env")
	if err := loadLegacyTokenFileLocked(); err != nil {
		return err
	}
	if err := loadTokenMapFileLocked(); err != nil {
		return err
	}
	return nil
}

func Resolve(requestToken string) (string, error) {
	requestToken = strings.TrimSpace(requestToken)
	if requestToken == "" {
		return "", ErrMissingToken
	}
	if requestToken == "free" {
		return requestToken, nil
	}

	storeLock.RLock()
	defer storeLock.RUnlock()

	if token, ok := tokenByKey[requestToken]; ok {
		if token == "" {
			return "", ErrNoStoredToken
		}
		return token, nil
	}
	if legacyProxyKey != "" && secureEqual(requestToken, legacyProxyKey) {
		return "", ErrNoStoredToken
	}
	return requestToken, nil
}

func IsAdminKey(key string) bool {
	key = strings.TrimSpace(key)
	storeLock.RLock()
	defer storeLock.RUnlock()

	if adminAPIKey != "" {
		return secureEqual(key, adminAPIKey)
	}
	return legacyProxyKey != "" && secureEqual(key, legacyProxyKey)
}

func AdminEnabled() bool {
	storeLock.RLock()
	defer storeLock.RUnlock()
	return adminAPIKey != "" || legacyProxyKey != ""
}

func SetToken(key, newToken string) error {
	key = strings.TrimSpace(key)
	newToken = strings.TrimSpace(newToken)
	if key == "" {
		key = legacyProxyKey
	}
	if key == "" {
		return ErrMissingKey
	}
	if newToken == "" {
		return ErrMissingToken
	}

	storeLock.Lock()
	defer storeLock.Unlock()

	tokenByKey[key] = newToken
	if tokenMapFile != "" {
		if err := writeTokenMapFileLocked(); err != nil {
			return err
		}
		sourceByKey[key] = "file"
		return nil
	}

	if singleTokenFile != "" && legacyProxyKey != "" && secureEqual(key, legacyProxyKey) {
		if err := writeTokenFile(singleTokenFile, newToken); err != nil {
			return err
		}
		sourceByKey[key] = "file"
		return nil
	}

	sourceByKey[key] = "runtime"
	return nil
}

func DeleteToken(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return ErrMissingKey
	}

	storeLock.Lock()
	defer storeLock.Unlock()

	if _, ok := tokenByKey[key]; !ok {
		return ErrTokenNotFound
	}
	delete(tokenByKey, key)
	delete(sourceByKey, key)
	if tokenMapFile != "" {
		return writeTokenMapFileLocked()
	}
	return nil
}

func GetStatus() Status {
	storeLock.RLock()
	defer storeLock.RUnlock()

	keys := make([]string, 0, len(tokenByKey))
	for key := range tokenByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	tokens := make([]TokenInfo, 0, len(keys))
	for _, key := range keys {
		tokens = append(tokens, TokenInfo{
			Key:          key,
			Source:       sourceByKey[key],
			TokenPreview: previewToken(tokenByKey[key]),
		})
	}

	source := "none"
	if len(tokens) > 0 {
		source = tokens[0].Source
	}

	return Status{
		Configured:       len(tokenByKey) > 0,
		Count:            len(tokenByKey),
		AdminEnabled:     adminAPIKey != "" || legacyProxyKey != "",
		LegacyProxyKey:   previewToken(legacyProxyKey),
		TokenFile:        singleTokenFile,
		TokenMapFile:     tokenMapFile,
		Tokens:           tokens,
		DeprecatedSource: source,
	}
}

func loadEnvTokenMapLocked(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	for _, pair := range splitPairs(raw) {
		key, token, ok := strings.Cut(pair, "=")
		if !ok {
			return fmt.Errorf("invalid ZAI_TOKEN_MAP entry %q", pair)
		}
		key = strings.TrimSpace(key)
		token = strings.TrimSpace(token)
		if key == "" || token == "" {
			return fmt.Errorf("invalid ZAI_TOKEN_MAP entry %q", pair)
		}
		tokenByKey[key] = token
		sourceByKey[key] = "env-map"
	}
	return nil
}

func splitPairs(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\n", ",")
	raw = strings.ReplaceAll(raw, ";", ",")

	var pairs []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			pairs = append(pairs, part)
		}
	}
	return pairs
}

func loadLegacyTokenLocked(token, source string) {
	if legacyProxyKey == "" || token == "" {
		return
	}
	tokenByKey[legacyProxyKey] = token
	sourceByKey[legacyProxyKey] = source
}

func loadLegacyTokenFileLocked() error {
	if singleTokenFile == "" || legacyProxyKey == "" {
		return nil
	}

	data, err := os.ReadFile(singleTokenFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	token := strings.TrimSpace(string(data))
	if token != "" {
		loadLegacyTokenLocked(token, "file")
	}
	return nil
}

func loadTokenMapFileLocked() error {
	if tokenMapFile == "" {
		return nil
	}

	data, err := os.ReadFile(tokenMapFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var fileTokens map[string]string
	if err := json.Unmarshal(data, &fileTokens); err != nil {
		return err
	}
	for key, token := range fileTokens {
		key = strings.TrimSpace(key)
		token = strings.TrimSpace(token)
		if key == "" || token == "" {
			continue
		}
		tokenByKey[key] = token
		sourceByKey[key] = "file-map"
	}
	return nil
}

func writeTokenMapFileLocked() error {
	if tokenMapFile == "" {
		return nil
	}
	return writeJSONFile(tokenMapFile, tokenByKey)
}

func writeJSONFile(path string, data interface{}) error {
	if err := ensureParentDir(path); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return os.WriteFile(path, payload, 0o600)
}

func writeTokenFile(path, token string) error {
	if err := ensureParentDir(path); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(token+"\n"), 0o600)
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		return os.MkdirAll(dir, 0o700)
	}
	return nil
}

func previewToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 12 {
		return "***"
	}
	return token[:6] + "..." + token[len(token)-6:]
}

func secureEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
