package auth

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v4"
)

// DefaultCredentialsPath is the credentials file location used when no explicit
// path is given. It deliberately lives outside any project directory: file_read
// is sandboxed to the project root, so a credentials file inside a project could
// be handed to the agent or picked up by a local source glob.
const DefaultCredentialsPath = "~/.mcpsmithy/credentials"

// defaultAuthHeader is the header used when a credential does not name one.
const defaultAuthHeader = "Authorization"

// defaultAuthScheme is the scheme used for token credentials that do not name
// one. It applies only to the Authorization header; a credential targeting a
// custom header sends the token bare unless it sets a scheme explicitly.
const defaultAuthScheme = "Bearer"

// Credential is the auth material for a single host.
//
// The fields present determine the header that gets sent:
//
//	token                     → Authorization: Bearer <token>
//	token + scheme            → Authorization: <scheme> <token>
//	token + header            → <header>: <token>
//	token + header + scheme   → <header>: <scheme> <token>
//	username + password       → Authorization: Basic base64(username:password)
type Credential struct {
	// Auth scheme prefixing the token, e.g. "Token". Defaults to "Bearer"
	// when the credential targets the Authorization header, and to no scheme
	// when it targets a custom header.
	Scheme string `yaml:"scheme,omitempty"`
	// Header to carry the credential. Defaults to "Authorization".
	// Set it for APIs that use their own header, e.g. "PRIVATE-TOKEN".
	Header string `yaml:"header,omitempty"`
	// Static API token. Mutually exclusive with username/password.
	Token string `yaml:"token,omitempty"`
	// Username for Basic auth. Mutually exclusive with token.
	Username string `yaml:"username,omitempty"`
	// Password for Basic auth.
	Password string `yaml:"password,omitempty"`
}

// credentialsFile is the on-disk shape of the credentials file.
type credentialsFile struct {
	// Credentials keyed by hostname, mirroring netrc's "machine" entries.
	Credentials map[string]Credential `yaml:"credentials"`
}

// Store resolves credentials by hostname. Hosts absent from the store fall back
// to ~/.netrc, which is deprecated but still honoured.
//
// A nil *Store applies no credentials at all, which keeps callers that have no
// credential source (tests, or code paths that must not authenticate) simple.
type Store struct {
	hosts map[string]Credential
	// netrcFallback is false only when the caller wants the store to be the
	// sole source of credentials.
	netrcFallback bool
}

// Load reads a credentials file and returns a Store.
//
// A missing file is not an error: the returned Store is empty and every lookup
// falls back to ~/.netrc, matching the silent-miss behaviour netrc has always
// had. A file that exists but cannot be read, parsed, or validated is an error,
// because silently ignoring a malformed credentials file would look like an
// authentication bug at request time.
func Load(path string) (*Store, error) {
	store := &Store{netrcFallback: true}

	resolved, err := expandHome(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return nil, fmt.Errorf("credentials file %s: %w", resolved, err)
	}
	if err := checkPerms(resolved, info.Mode()); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("credentials file %s: %w", resolved, err)
	}

	// An empty file is treated as "no credentials" rather than a parse error,
	// so touching the file to create it does not break startup.
	if len(strings.TrimSpace(string(data))) == 0 {
		return store, nil
	}

	var file credentialsFile
	if err := yaml.Load(data, &file, yaml.WithKnownFields()); err != nil {
		return nil, fmt.Errorf("parsing credentials file %s: %w", resolved, err)
	}

	store.hosts = make(map[string]Credential, len(file.Credentials))
	for host, cred := range file.Credentials {
		if err := cred.validate(host); err != nil {
			return nil, fmt.Errorf("credentials file %s: %w", resolved, err)
		}
		store.hosts[strings.ToLower(host)] = cred
	}
	return store, nil
}

// HasCredentials reports whether the store holds any entries. Callers use it to
// decide whether to warn that netrc is being relied on.
func (s *Store) HasCredentials() bool {
	return s != nil && len(s.hosts) > 0
}

// Apply sets the auth header on req for the request's hostname.
// Hosts without an entry fall back to ~/.netrc. A nil Store is a no-op.
func (s *Store) Apply(req *http.Request) {
	if s == nil {
		return
	}
	host := strings.ToLower(req.URL.Hostname())
	cred, ok := s.hosts[host]
	if !ok {
		if s.netrcFallback {
			applyNetrcAuth(req)
		}
		return
	}
	name, value := cred.header()
	req.Header.Set(name, value)
}

// header returns the header name and value to send for this credential.
func (c Credential) header() (name, value string) {
	name = c.Header
	if name == "" {
		name = defaultAuthHeader
	}

	if c.Username != "" {
		return name, "Basic " + basicCreds(c.Username, c.Password)
	}

	scheme := c.Scheme
	// Bearer is only assumed for the standard header. A custom header means a
	// vendor format, where an unrequested "Bearer " prefix would break the call.
	if scheme == "" && c.Header == "" {
		scheme = defaultAuthScheme
	}
	if scheme == "" {
		return name, c.Token
	}
	return name, scheme + " " + c.Token
}

// validate checks that a credential entry is well formed. It reports the host
// so the error points at the offending entry.
func (c Credential) validate(host string) error {
	if host == "" {
		return fmt.Errorf("credential with an empty host key")
	}
	if c.Token != "" && c.Username != "" {
		return fmt.Errorf("credential for %s sets both token and username; use one or the other", host)
	}
	if c.Token == "" && c.Username == "" {
		return fmt.Errorf("credential for %s sets neither token nor username", host)
	}
	if c.Token != "" && c.Password != "" {
		return fmt.Errorf("credential for %s sets both token and password; use one or the other", host)
	}
	if c.Header != "" && !validHeaderName(c.Header) {
		return fmt.Errorf("credential for %s has invalid header name %q", host, c.Header)
	}
	// The scheme is joined to the token with a single space, per RFC 7235. A
	// scheme containing its own whitespace would produce a malformed header, so
	// vendor formats like "Token token=abc" belong in the token field.
	if c.Scheme != "" && !validHeaderName(c.Scheme) {
		return fmt.Errorf("credential for %s has invalid scheme %q; a scheme is a single word, so put any vendor prefix in the token (scheme: Token, token: token=abc)", host, c.Scheme)
	}
	if !validHeaderValue(c.Token) {
		return fmt.Errorf("credential for %s has a token containing control characters", host)
	}
	if !validHeaderValue(c.Username) || !validHeaderValue(c.Password) {
		return fmt.Errorf("credential for %s has a username or password containing control characters", host)
	}
	return nil
}

// validHeaderName reports whether s is a valid HTTP token as defined by
// RFC 9110, which is the grammar for both header field names and auth schemes.
func validHeaderName(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range []byte(s) {
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		case strings.IndexByte("!#$%&'*+-.^_`|~", c) >= 0:
		default:
			return false
		}
	}
	return true
}

// validHeaderValue reports whether s is safe to send as a header value, i.e.
// contains no control characters that would let it inject additional headers.
func validHeaderValue(s string) bool {
	for _, c := range []byte(s) {
		if c < 0x20 && c != '\t' || c == 0x7f {
			return false
		}
	}
	return true
}

// checkPerms rejects a credentials file that is readable by anyone other than
// its owner, following the convention ssh and curl apply to their own secrets.
func checkPerms(path string, mode fs.FileMode) error {
	if mode.Perm()&0o077 != 0 {
		return fmt.Errorf("credentials file %s has permissions %04o; it must not be readable by group or others (chmod 600 %s)",
			path, mode.Perm(), path)
	}
	return nil
}

// expandHome resolves a leading ~/ against the user's home directory.
func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", path, err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
}
