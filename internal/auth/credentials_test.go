package auth

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeCreds writes a credentials file with owner-only permissions and returns its path.
func writeCreds(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "credentials")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

// loadStore loads a credentials file that is expected to be valid.
func loadStore(t *testing.T, content string) *Store {
	t.Helper()
	store, err := Load(writeCreds(t, content))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return store
}

func TestStoreApplyHeaderShapes(t *testing.T) {
	// Every documented credential shape, and the header each one produces.
	content := `credentials:
  bearer.example.com:
    token: tok123
  scheme.example.com:
    scheme: Token
    token: abc123
  header.example.com:
    header: PRIVATE-TOKEN
    token: glpat-xxx
  both.example.com:
    header: X-Auth
    scheme: Token
    token: abc
  basic.example.com:
    username: myuser
    password: mypass
`
	store := loadStore(t, content)

	tests := []struct {
		name       string
		host       string
		wantHeader string
		wantValue  string
	}{
		{
			name:       "token alone defaults to Bearer",
			host:       "bearer.example.com",
			wantHeader: "Authorization",
			wantValue:  "Bearer tok123",
		},
		{
			name:       "explicit scheme replaces Bearer",
			host:       "scheme.example.com",
			wantHeader: "Authorization",
			wantValue:  "Token abc123",
		},
		{
			// A custom header means a vendor format; an unrequested "Bearer "
			// prefix would break the call, so no scheme is assumed.
			name:       "custom header sends the token bare",
			host:       "header.example.com",
			wantHeader: "PRIVATE-TOKEN",
			wantValue:  "glpat-xxx",
		},
		{
			name:       "custom header with explicit scheme",
			host:       "both.example.com",
			wantHeader: "X-Auth",
			wantValue:  "Token abc",
		},
		{
			name:       "username and password produce Basic",
			host:       "basic.example.com",
			wantHeader: "Authorization",
			wantValue:  "Basic " + base64Encode("myuser:mypass"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "https://"+tt.host+"/v1/x", nil)
			store.Apply(req)
			if got := req.Header.Get(tt.wantHeader); got != tt.wantValue {
				t.Errorf("%s = %q, want %q", tt.wantHeader, got, tt.wantValue)
			}
			// A custom header must not also populate Authorization.
			if tt.wantHeader != "Authorization" {
				if got := req.Header.Get("Authorization"); got != "" {
					t.Errorf("Authorization = %q, want empty", got)
				}
			}
		})
	}
}

func TestStoreApplyHostMatching(t *testing.T) {
	store := loadStore(t, "credentials:\n  API.Example.COM:\n    token: tok\n")

	t.Run("host match is case-insensitive", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "https://api.example.com/x", nil)
		store.Apply(req)
		if got := req.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer tok")
		}
	})

	t.Run("port is ignored when matching", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "https://api.example.com:8443/x", nil)
		store.Apply(req)
		if got := req.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer tok")
		}
	})

	t.Run("nil store applies nothing", func(t *testing.T) {
		var nilStore *Store
		req, _ := http.NewRequest("GET", "https://api.example.com/x", nil)
		nilStore.Apply(req)
		if got := req.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization = %q, want empty", got)
		}
	})
}

// TestStoreFallsBackToNetrc verifies a host absent from the credentials file
// still authenticates from netrc, so existing setups keep working.
func TestStoreFallsBackToNetrc(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	netrc := "machine legacy.example.com login token password netrctok\n"
	if err := os.WriteFile(filepath.Join(home, ".netrc"), []byte(netrc), 0600); err != nil {
		t.Fatal(err)
	}

	store := loadStore(t, "credentials:\n  new.example.com:\n    scheme: Token\n    token: newtok\n")

	t.Run("credentials file wins for its own host", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "https://new.example.com/x", nil)
		store.Apply(req)
		if got := req.Header.Get("Authorization"); got != "Token newtok" {
			t.Errorf("Authorization = %q, want %q", got, "Token newtok")
		}
	})

	t.Run("netrc covers hosts the credentials file omits", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "https://legacy.example.com/x", nil)
		store.Apply(req)
		if got := req.Header.Get("Authorization"); got != "Bearer netrctok" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer netrctok")
		}
	})

	t.Run("unknown host gets no header", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "https://nowhere.example.com/x", nil)
		store.Apply(req)
		if got := req.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization = %q, want empty", got)
		}
	})
}

func TestLoadMissingFileIsNotAnError(t *testing.T) {
	// A missing credentials file must not be fatal; netrc remains the fallback.
	store, err := Load(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("missing file should not error, got: %v", err)
	}
	if store == nil {
		t.Fatal("expected a usable store")
	}
	if store.HasCredentials() {
		t.Error("empty store should report no credentials")
	}
}

func TestLoadValidation(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "token and username conflict",
			content: "credentials:\n  a.example.com:\n    token: t\n    username: u\n",
			wantErr: "both token and username",
		},
		{
			name:    "token and password conflict",
			content: "credentials:\n  a.example.com:\n    token: t\n    password: p\n",
			wantErr: "both token and password",
		},
		{
			name:    "neither token nor username",
			content: "credentials:\n  a.example.com:\n    scheme: Token\n",
			wantErr: "neither token nor username",
		},
		{
			name:    "invalid header name",
			content: "credentials:\n  a.example.com:\n    header: \"Bad Header\"\n    token: t\n",
			wantErr: "invalid header name",
		},
		{
			// A scheme is a single word; a vendor prefix belongs in the token.
			name:    "scheme with whitespace",
			content: "credentials:\n  a.example.com:\n    scheme: \"Token token=\"\n    token: t\n",
			wantErr: "invalid scheme",
		},
		{
			name:    "unknown field",
			content: "credentials:\n  a.example.com:\n    tokenn: t\n",
			wantErr: "",
		},
		{
			name:    "malformed yaml",
			content: "credentials:\n  a.example.com:\n  - not a map\n",
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(writeCreds(t, tt.content))
			if err == nil {
				t.Fatal("expected an error")
			}
			if tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadEmptyFile(t *testing.T) {
	store, err := Load(writeCreds(t, ""))
	if err != nil {
		t.Fatalf("empty file should load, got: %v", err)
	}
	if store.HasCredentials() {
		t.Error("empty file should yield no credentials")
	}
}

func TestHasCredentials(t *testing.T) {
	store := loadStore(t, "credentials:\n  a.example.com:\n    token: t\n")
	if !store.HasCredentials() {
		t.Error("store with an entry should report credentials")
	}
	var nilStore *Store
	if nilStore.HasCredentials() {
		t.Error("nil store should report no credentials")
	}
}
