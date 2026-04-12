package transport

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type tokenFetcher struct {
	cache   *tokenCache
	fetcher TokenFetcher
}

func Fetch(challenge *WWWChallenge) (string, error) {
	req, err := http.NewRequest("GET", challenge.Realm, nil)
	if err != nil {
		return "", err
	}
	q := req.URL.Query()
	q.Set("service", challenge.Service)
	q.Set("scope", challenge.Scope)
	req.URL.RawQuery = q.Encode()

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned status %d", res.StatusCode)
	}
	var tokenResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&tokenResp); err != nil {
		return "", err
	}
	return tokenResp.Token, nil

}
func NewTokenFetcher(cache *tokenCache) *tokenFetcher {
	return &tokenFetcher{
		cache: cache,
	}
}

func (f *tokenFetcher) Fetch(challenge *WWWChallenge) (string, error) {
	cacheKey := fmt.Sprintf("%s|%s|%s", challenge.Realm, challenge.Service, challenge.Scope)
	if token, ok := f.cache.Get(cacheKey); ok {
		return token, nil
	}
	token, err := f.fetcher.Fetch(challenge)
	if err != nil {
		return "", err
	}
	f.cache.Set(cacheKey, token)
	return token, nil
}
