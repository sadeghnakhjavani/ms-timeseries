package api

import (
	"net/http"

	chstore "github.com/iranrates/marketick/internal/clickhouse"
)

func NewRouter(store *chstore.Client, auth *Authenticator, pagination PaginationSettings) http.Handler {
	mux := http.NewServeMux()

	ticksHandler := NewTicksHandler(store, pagination)
	candlesHandler := NewCandlesHandler(store, pagination)
	symbolsHandler := NewSymbolsHandler(store)

	mux.HandleFunc("POST /api/v1/ticks", ticksHandler.Ingest)
	mux.HandleFunc("GET /api/v1/ticks", ticksHandler.Read)
	mux.HandleFunc("DELETE /api/v1/symbols/{symbol}", symbolsHandler.Delete)
	mux.HandleFunc("GET /api/v1/candles", candlesHandler.Read)

	return auth.Middleware(mux)
}
