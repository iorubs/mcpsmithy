package auth

import (
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func TestApplyNetrcAuth(t *testing.T) {
	// Write a temp .netrc so netrcLookup finds credentials without touching the
	// real home directory.
	home := t.TempDir()
	t.Setenv("HOME", home)

	netrcPath := filepath.Join(home, ".netrc")
	netrc := "machine bearer.example.com login token password tok123\n" +
		"machine basic.example.com login myuser password mypass\n" +
		"machine bare.example.com password baretoken\n"
	if err := os.WriteFile(netrcPath, []byte(netrc), 0600); err != nil {
		t.Fatal(err)
	}

	t.Run("bearer when login is token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "https://bearer.example.com/v1/foo", nil)
		ApplyNetrcAuth(req)
		if got := req.Header.Get("Authorization"); got != "Bearer tok123" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer tok123")
		}
	})

	t.Run("basic when login is a username", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "https://basic.example.com/v1/foo", nil)
		ApplyNetrcAuth(req)
		want := "Basic " + base64Encode("myuser:mypass")
		if got := req.Header.Get("Authorization"); got != want {
			t.Errorf("Authorization = %q, want %q", got, want)
		}
	})

	t.Run("bearer when login is empty", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "https://bare.example.com/v1/foo", nil)
		ApplyNetrcAuth(req)
		if got := req.Header.Get("Authorization"); got != "Bearer baretoken" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer baretoken")
		}
	})

	t.Run("no header when host not in netrc", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "https://other.example.com/v1/foo", nil)
		ApplyNetrcAuth(req)
		if got := req.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization = %q, want empty", got)
		}
	})
}

func TestParseNetrc(t *testing.T) {
	content := "machine github.com\nlogin user\npassword ghp_token123\n\nmachine gitlab.com\nlogin oauth2\npassword glpat_abc\n\ndefault\nlogin anonymous\npassword guest\n"

	tests := []struct {
		name         string
		content      string
		host         string
		wantLogin    string
		wantPassword string
		wantOK       bool
	}{
		{
			name:         "github match",
			content:      content,
			host:         "github.com",
			wantLogin:    "user",
			wantPassword: "ghp_token123",
			wantOK:       true,
		},
		{
			name:         "default fallback",
			content:      content,
			host:         "unknown.host.com",
			wantLogin:    "anonymous",
			wantPassword: "guest",
			wantOK:       true,
		},
		{
			name:    "no match without default",
			content: "machine github.com\nlogin user\npassword ghp_token123\n",
			host:    "other.com",
			wantOK:  false,
		},
		{
			name:         "single line format",
			content:      "machine github.com login user password secret",
			host:         "github.com",
			wantLogin:    "user",
			wantPassword: "secret",
			wantOK:       true,
		},
		{
			name:         "first match wins",
			content:      "machine github.com login first password pass1\nmachine github.com login second password pass2\n",
			host:         "github.com",
			wantLogin:    "first",
			wantPassword: "pass1",
			wantOK:       true,
		},
		{
			name:    "empty file",
			content: "",
			host:    "github.com",
			wantOK:  false,
		},
		{
			// macdef blocks must be skipped until a blank line; the machine entry
			// after the block should still be found.
			name: "macdef block skipped",
			content: "machine github.com login user password ghp_token123\n" +
				"macdef init\n" +
				"some macro line\n" +
				"\n" +
				"machine after.com login after password afterpass\n",
			host:         "after.com",
			wantLogin:    "after",
			wantPassword: "afterpass",
			wantOK:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			login, password, ok := parseNetrc(tt.content, tt.host)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if login != tt.wantLogin {
				t.Errorf("login = %q, want %q", login, tt.wantLogin)
			}
			if password != tt.wantPassword {
				t.Errorf("password = %q, want %q", password, tt.wantPassword)
			}
		})
	}
}
