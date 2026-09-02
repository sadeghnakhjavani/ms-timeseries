package api

import (
	"encoding/json"
	"net/http"

	"github.com/iranrates/marketick/internal/model"
)

func WriteJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func WriteSuccess(w http.ResponseWriter, status int, data interface{}) {
	WriteJSON(w, status, model.Envelope{Data: data})
}

func WriteSuccessWithMetadata(w http.ResponseWriter, status int, data interface{}, metadata model.Metadata) {
	WriteJSON(w, status, model.Envelope{
		Data:     data,
		Metadata: &metadata,
	})
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
