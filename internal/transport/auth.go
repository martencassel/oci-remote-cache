package transport

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

type FakeRT struct {
	RoundTripFunc func(*http.Request) (*http.Response, error)
}

func (f *FakeRT) RoundTrip(req *http.Request) (*http.Response, error) {
	return f.RoundTripFunc(req)
}

type FakeFetcher struct {
	Token string
	Err   error
}

func (f *FakeFetcher) Fetch(c *WWWChallenge) (string, error) {
	return f.Token, f.Err
}

type tokenCache struct {
	mu     sync.Mutex
	tokens map[string]string
}

func NewTokenCache() *tokenCache {
	return &tokenCache{
		tokens: make(map[string]string),
	}
}

func (c *tokenCache) Get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	token, ok := c.tokens[key]
	return token, ok
}

func (c *tokenCache) Set(key, token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tokens == nil {
		c.tokens = make(map[string]string)
	}
	c.tokens[key] = token
}

type TokenFetcher interface {
	Fetch(*WWWChallenge) (string, error)
}

type AuthRoundTripper struct {
	Next    http.RoundTripper
	cache   *tokenCache
	fetcher TokenFetcher
}

func NewAuthRoundTripper(cache *tokenCache, fetcher TokenFetcher) RTFactory {
	return func(next http.RoundTripper) http.RoundTripper {
		return &AuthRoundTripper{
			Next:    next,
			cache:   cache,
			fetcher: fetcher,
		}
	}
}

func cloneRequest(r *http.Request) *http.Request {
	r2 := r.Clone(r.Context())
	r2.Header = r.Header.Clone()
	return r2
}

func (rt *AuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// 1. First attempt: send request without token
	resp, err := rt.Next.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	// 2. Parse challenge
	authHeader := resp.Header.Get("WWW-Authenticate")
	challenge, err := parseWWWAuthenticate(authHeader)
	if err != nil || !challenge.IsValid() {
		return resp, nil // Can't parse challenge, give up
	}

	// We are going to retry; drain and close the 401 body so the connection
	// can be reused.  401 bodies are tiny (auth error JSON), so a small limit
	// is enough.
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	resp.Body.Close()

	// 3. Compute correct cache key
	key := challenge.CacheKey(req.URL.Host)

	// 4. Check cache
	if token, ok := rt.cache.Get(key); ok {
		req2 := cloneRequest(req)
		req2.Header.Set("Authorization", "Bearer "+token)
		return rt.Next.RoundTrip(req2)
	}

	// 5. Fetch new token
	newToken, err := fetchToken(challenge)
	if err != nil {
		return resp, nil // No token in cache, give up
	}

	// 6. Store token
	rt.cache.Set(key, newToken)

	// 7. Retry request with new token
	req2 := cloneRequest(req)
	req2.Header.Set("Authorization", "Bearer "+newToken)
	return rt.Next.RoundTrip(req2)
}

type WWWChallenge struct {
	Realm   string
	Service string
	Scope   string
}

func (c *WWWChallenge) IsValid() bool {
	return c.Realm != "" && c.Service != "" && c.Scope != ""
}

func (c *WWWChallenge) CacheKey(host string) string {
	return fmt.Sprintf("%s|%s|%s", host, c.Service, c.Scope)
}

func parseWWWAuthenticate(header string) (*WWWChallenge, error) {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, fmt.Errorf("unsupported WWW-Authenticate header: %s", header)
	}
	challenge := &WWWChallenge{}
	for _, kv := range strings.Split(parts[1], ",") {
		kv = strings.TrimSpace(kv)
		kvParts := strings.SplitN(kv, "=", 2)
		if len(kvParts) != 2 {
			continue
		}
		key := kvParts[0]
		value := strings.Trim(kvParts[1], `"`)
		switch key {
		case "realm":
			challenge.Realm = value
		case "service":
			challenge.Service = value
		case "scope":
			challenge.Scope = value
		}
	}
	return challenge, nil
}

func fetchToken(challenge *WWWChallenge) (string, error) {
	req, err := http.NewRequest("GET", challenge.Realm, nil)
	if err != nil {
		return "", err
	}
	q := req.URL.Query()
	q.Set("service", challenge.Service)
	q.Set("scope", challenge.Scope)
	req.URL.RawQuery = q.Encode()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned %d", resp.StatusCode)
	}
	var tokenResp struct {
		Token string `json:"token"`
	}
	err = json.NewDecoder(resp.Body).Decode(&tokenResp)
	if err != nil {
		return "", err
	}
	return tokenResp.Token, nil
}
