package auth

import (
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// applyNetrcAuth sets the Authorization header on req using .netrc credentials
// for the request's hostname. When login is "token" (or empty), the password
// is sent as a Bearer token. Otherwise login and password are sent as Basic Auth.
//
// Deprecated: netrc cannot express vendor auth schemes or custom header names,
// because its keyword set is fixed and the file is shared with curl and git.
// It remains the fallback for hosts absent from the credentials file; see
// [Store.Apply] and [DefaultCredentialsPath].
func applyNetrcAuth(req *http.Request) {
	host := req.URL.Hostname()
	login, password, ok := netrcLookup(host)
	if !ok {
		return
	}
	if login == "" || login == "token" {
		req.Header.Set("Authorization", "Bearer "+password)
	} else {
		req.Header.Set("Authorization", "Basic "+basicCreds(login, password))
	}
}

// basicCreds encodes a username and password for the Basic auth scheme.
func basicCreds(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}

// NetrcExists reports whether a netrc file is present in the user's home
// directory. Callers use it to warn that the deprecated credential source is
// being relied on.
func NetrcExists() bool {
	path, err := netrcPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// netrcPath returns the netrc file location for the current user.
func netrcPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(home, "_netrc"), nil
	}
	return filepath.Join(home, ".netrc"), nil
}

// netrcLookup reads ~/.netrc and extracts credentials for the given host.
// Empty strings and false are returned if no entry matches or the file is absent.
func netrcLookup(host string) (login, password string, ok bool) {
	path, err := netrcPath()
	if err != nil {
		return "", "", false
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", false
	}
	return parseNetrc(string(data), host)
}

// parseNetrc extracts login and password for host from netrc-formatted text.
// First matching machine entry wins. Falls back to the default entry if present.
// Processes the file line-by-line; macdef blocks are skipped until a blank line.
func parseNetrc(content, host string) (login, password string, ok bool) {
	type entry struct{ login, password string }
	var current, match, fallback *entry
	inMacro := false

	for line := range strings.SplitSeq(content, "\n") {
		if inMacro {
			if line == "" {
				inMacro = false
			}
			continue
		}

		f := strings.Fields(line)
		i := 0
		for ; i < len(f)-1; i += 2 {
			switch f[i] {
			case "machine":
				e := &entry{}
				if f[i+1] == host && match == nil {
					match = e
				}
				current = e
			case "login":
				if current != nil {
					current.login = f[i+1]
				}
			case "password":
				if current != nil {
					current.password = f[i+1]
				}
			case "macdef":
				inMacro = true
				current = nil
			}
		}
		// "default" is a lone keyword with no value token.
		if i < len(f) && f[i] == "default" {
			if fallback == nil {
				fallback = &entry{}
			}
			current = fallback
		}
	}

	if match != nil {
		return match.login, match.password, true
	}
	if fallback != nil {
		return fallback.login, fallback.password, true
	}
	return "", "", false
}
