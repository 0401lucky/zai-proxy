package tokenstore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveWithTokenMap(t *testing.T) {
	if err := Init("", "", "", "alice=token-a,bob=token-b", "", "admin-key"); err != nil {
		t.Fatalf("Init: %v", err)
	}

	got, err := Resolve("alice")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "token-a" {
		t.Errorf("Resolve = %q, want %q", got, "token-a")
	}
}

func TestResolvePassesThroughPersonalToken(t *testing.T) {
	if err := Init("", "", "", "alice=token-a", "", "admin-key"); err != nil {
		t.Fatalf("Init: %v", err)
	}

	got, err := Resolve("direct-zai-token")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "direct-zai-token" {
		t.Errorf("Resolve = %q, want pass-through token", got)
	}
}

func TestLegacyProxyKeyWithoutStoredToken(t *testing.T) {
	if err := Init("", "", "proxy-key", "", "", ""); err != nil {
		t.Fatalf("Init: %v", err)
	}

	_, err := Resolve("proxy-key")
	if !errors.Is(err, ErrNoStoredToken) {
		t.Fatalf("Resolve err = %v, want ErrNoStoredToken", err)
	}
}

func TestSetTokenWritesTokenMapFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zai_tokens.json")
	if err := Init("", "", "", "", path, "admin-key"); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := SetToken("alice", "token-a"); err != nil {
		t.Fatalf("SetToken: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !containsAll(string(data), `"alice"`, `"token-a"`) {
		t.Errorf("file content = %q", string(data))
	}

	status := GetStatus()
	if !status.Configured || status.Count != 1 || status.Tokens[0].Source != "file" {
		t.Errorf("status = %+v", status)
	}
}

func TestDeleteTokenWritesTokenMapFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zai_tokens.json")
	if err := Init("", "", "", "alice=token-a,bob=token-b", path, "admin-key"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := DeleteToken("alice"); err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if containsAll(string(data), `"alice"`) {
		t.Errorf("file should not contain alice, got %q", string(data))
	}
	if !containsAll(string(data), `"bob"`, `"token-b"`) {
		t.Errorf("file should contain bob token, got %q", string(data))
	}
}

func containsAll(s string, needles ...string) bool {
	for _, needle := range needles {
		if !contains(s, needle) {
			return false
		}
	}
	return true
}

func contains(s, needle string) bool {
	return len(needle) == 0 || (len(s) >= len(needle) && indexOf(s, needle) >= 0)
}

func indexOf(s, needle string) int {
	for i := 0; i+len(needle) <= len(s); i++ {
		if s[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
