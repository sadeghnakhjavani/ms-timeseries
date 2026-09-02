package api

import (
	"crypto/subtle"
	"net/http"
)

const apiKeyHeader = "X-API-Key"

type Authenticator struct {
	apiKey []byte
}

func NewAuthenticator(apiKey string) *Authenticator {
	return &Authenticator{apiKey: []byte(apiKey)}
}

func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.Method, r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		key := r.Header.Get(apiKeyHeader)
		if key == "" {
			WriteError(w, http.StatusUnauthorized, "unauthorized", "missing API key")
			return
		}

		if subtle.ConstantTimeCompare([]byte(key), a.apiKey) != 1 {
			WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid API key")
			return
		}

		next.ServeHTTP(w, r)
	})
}
