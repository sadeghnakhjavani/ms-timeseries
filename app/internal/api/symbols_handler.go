package api

import (
	"context"
	"net/http"

	chstore "github.com/iranrates/marketick/internal/clickhouse"
	"github.com/iranrates/marketick/internal/model"
)

type SymbolsHandler struct {
	store *chstore.Client
}

func NewSymbolsHandler(store *chstore.Client) *SymbolsHandler {
	return &SymbolsHandler{store: store}
}

func (h *SymbolsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	symbol := r.PathValue("symbol")
	if symbol == "" {
		WriteError(w, http.StatusBadRequest, "invalid_request", "symbol is required")
		return
	}

	ctx := context.Background()
	summary, err := h.store.DeleteSymbol(ctx, symbol)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete symbol")
		return
	}

	WriteSuccess(w, http.StatusOK, model.DeleteSummary{
		Symbol:  symbol,
		Deleted: summary,
	})
}
