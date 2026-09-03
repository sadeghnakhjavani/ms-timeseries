package api

import (
	"net/http"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	WriteSuccess(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	WriteSuccess(w, http.StatusOK, map[string]string{
		"service": "ms-timeseries",
		"health":  "/health",
		"api":     "/api/v1",
	})
}

func isPublicPath(method, path string) bool {
	return method == http.MethodGet && (path == "/health" || path == "/")
}
