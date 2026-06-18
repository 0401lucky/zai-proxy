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
	"time"
)

var (
	ErrMissingToken  = errors.New("missing api key")
	ErrMissingKey    = errors.New("missing proxy key")
	ErrNoStoredToken = errors.New("stored z.ai token is not configured")
	ErrAdminDisabled = errors.New("admin token api is disabled")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrTokenNotFound = errors.New("token key not found")
	accountByKey     = map[string]Account{}
	sourceByKey      = map[string]string{}
	legacyProxyKey   string
	poolAPIKey       string
	adminAPIKey      string
	singleTokenFile  string
	tokenMapFile     string
	poolCursor       int
	storeLock        sync.RWMutex
)

type Status struct {
	Configured       bool        `json:"configured"`
	Count            int         `json:"count"`
	AdminEnabled     bool        `json:"admin_enabled"`
	PoolEnabled      bool        `json:"pool_enabled"`
	PoolKeyPreview   string      `json:"pool_key_preview,omitempty"`
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
	UserID       string `json:"user_id,omitempty"`
	Email        string `json:"email,omitempty"`
	Name         string `json:"name,omitempty"`
	Role         string `json:"role,omitempty"`
	UpdatedAt    int64  `json:"updated_at,omitempty"`
}

type Account struct {
	Token     string `json:"token"`
	JWT       string `json:"jwt,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	Email     string `json:"email,omitempty"`
	Name      string `json:"name,omitempty"`
	Role      string `json:"role,omitempty"`
	UpdatedAt int64  `json:"updated_at,omitempty"`
}

func Init(initialToken, filePath, apiKey, poolKey, tokenMap, mapFile, adminKey string) error {
	storeLock.Lock()
	defer storeLock.Unlock()

	accountByKey = map[string]Account{}
	sourceByKey = map[string]string{}
	legacyProxyKey = strings.TrimSpace(apiKey)
	poolAPIKey = strings.TrimSpace(poolKey)
	adminAPIKey = strings.TrimSpace(adminKey)
	singleTokenFile = strings.TrimSpace(filePath)
	tokenMapFile = strings.TrimSpace(mapFile)
	poolCursor = 0

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

	if poolAPIKey != "" && secureEqual(requestToken, poolAPIKey) {
		storeLock.RUnlock()
		return resolveFromPool()
	}
	if account, ok := accountByKey[requestToken]; ok {
		storeLock.RUnlock()
		if account.Token == "" {
			return "", ErrNoStoredToken
		}
		return account.Token, nil
	}
	if legacyProxyKey != "" && secureEqual(requestToken, legacyProxyKey) {
		storeLock.RUnlock()
		return "", ErrNoStoredToken
	}
	storeLock.RUnlock()
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
	return setAccount(key, Account{
		Token:     strings.TrimSpace(newToken),
		UpdatedAt: time.Now().Unix(),
	}, false)
}

func SetAccount(key string, account Account) error {
	account.Token = strings.TrimSpace(account.Token)
	account.JWT = strings.TrimSpace(account.JWT)
	account.UserID = strings.TrimSpace(account.UserID)
	account.Email = strings.TrimSpace(account.Email)
	account.Name = strings.TrimSpace(account.Name)
	account.Role = strings.TrimSpace(account.Role)
	if account.UpdatedAt == 0 {
		account.UpdatedAt = time.Now().Unix()
	}
	return setAccount(key, account, true)
}

func setAccount(key string, account Account, structured bool) error {
	key = strings.TrimSpace(key)
	if key == "" {
		key = legacyProxyKey
	}
	if key == "" {
		return ErrMissingKey
	}
	if account.Token == "" {
		return ErrMissingToken
	}

	storeLock.Lock()
	defer storeLock.Unlock()

	accountByKey[key] = account
	if tokenMapFile != "" {
		if err := writeTokenMapFileLocked(); err != nil {
			return err
		}
		sourceByKey[key] = "file"
		return nil
	}

	if !structured && singleTokenFile != "" && legacyProxyKey != "" && secureEqual(key, legacyProxyKey) {
		if err := writeTokenFile(singleTokenFile, account.Token); err != nil {
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

	if _, ok := accountByKey[key]; !ok {
		return ErrTokenNotFound
	}
	delete(accountByKey, key)
	delete(sourceByKey, key)
	if tokenMapFile != "" {
		return writeTokenMapFileLocked()
	}
	return nil
}

func GetStatus() Status {
	storeLock.RLock()
	defer storeLock.RUnlock()

	keys := sortedKeysLocked()

	tokens := make([]TokenInfo, 0, len(keys))
	for _, key := range keys {
		account := accountByKey[key]
		tokens = append(tokens, TokenInfo{
			Key:          key,
			Source:       sourceByKey[key],
			TokenPreview: previewToken(account.Token),
			UserID:       account.UserID,
			Email:        account.Email,
			Name:         account.Name,
			Role:         account.Role,
			UpdatedAt:    account.UpdatedAt,
		})
	}

	source := "none"
	if len(tokens) > 0 {
		source = tokens[0].Source
	}

	return Status{
		Configured:       len(accountByKey) > 0,
		Count:            len(accountByKey),
		AdminEnabled:     adminAPIKey != "" || legacyProxyKey != "",
		PoolEnabled:      poolAPIKey != "",
		PoolKeyPreview:   previewToken(poolAPIKey),
		LegacyProxyKey:   previewToken(legacyProxyKey),
		TokenFile:        singleTokenFile,
		TokenMapFile:     tokenMapFile,
		Tokens:           tokens,
		DeprecatedSource: source,
	}
}

func resolveFromPool() (string, error) {
	storeLock.Lock()
	defer storeLock.Unlock()

	keys := sortedKeysLocked()
	if len(keys) == 0 {
		return "", ErrNoStoredToken
	}
	key := keys[poolCursor%len(keys)]
	poolCursor = (poolCursor + 1) % len(keys)
	account := accountByKey[key]
	if account.Token == "" {
		return "", ErrNoStoredToken
	}
	return account.Token, nil
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
		accountByKey[key] = Account{
			Token:     token,
			JWT:       token,
			UpdatedAt: time.Now().Unix(),
		}
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
	accountByKey[legacyProxyKey] = Account{
		Token:     token,
		JWT:       token,
		UpdatedAt: time.Now().Unix(),
	}
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

	var rawTokens map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawTokens); err != nil {
		return err
	}
	for key, raw := range rawTokens {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}

		var token string
		if err := json.Unmarshal(raw, &token); err == nil {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}
			accountByKey[key] = Account{
				Token:     token,
				JWT:       token,
				UpdatedAt: time.Now().Unix(),
			}
			sourceByKey[key] = "file-map"
			continue
		}

		var account Account
		if err := json.Unmarshal(raw, &account); err != nil {
			continue
		}
		account.Token = strings.TrimSpace(account.Token)
		account.JWT = strings.TrimSpace(account.JWT)
		if account.Token == "" && account.JWT != "" {
			account.Token = account.JWT
		}
		if account.Token == "" {
			continue
		}
		if account.UpdatedAt == 0 {
			account.UpdatedAt = time.Now().Unix()
		}
		accountByKey[key] = account
		sourceByKey[key] = "file-map"
	}
	return nil
}

func writeTokenMapFileLocked() error {
	if tokenMapFile == "" {
		return nil
	}
	return writeJSONFile(tokenMapFile, accountByKey)
}

func sortedKeysLocked() []string {
	keys := make([]string, 0, len(accountByKey))
	for key := range accountByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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
