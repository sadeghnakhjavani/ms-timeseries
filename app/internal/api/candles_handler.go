package api

import (
	"net/http"

	chstore "github.com/iranrates/ms-timeseries/internal/clickhouse"
	"github.com/iranrates/ms-timeseries/internal/model"
)

type CandlesHandler struct {
	store      *chstore.Client
	pagination PaginationSettings
}

func NewCandlesHandler(store *chstore.Client, pagination PaginationSettings) *CandlesHandler {
	return &CandlesHandler{
		store:      store,
		pagination: pagination,
	}
}

func (h *CandlesHandler) Read(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Has("calendar") {
		WriteError(w, http.StatusBadRequest, "invalid_request", "calendar query parameter is not allowed")
		return
	}

	symbol := q.Get("symbol")
	if symbol == "" {
		WriteError(w, http.StatusBadRequest, "invalid_request", "symbol is required")
		return
	}

	timeframe := q.Get("timeframe")
	if err := validateTimeframe(timeframe); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	from, err := parseDateParam(q.Get("from"), "from")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	to, err := parseDateParam(q.Get("to"), "to")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := validateDateRange(from, to); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	page, err := parsePageParam(q.Get("page"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	limit, err := parseLimitParam(q.Get("limit"), h.pagination)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	ctx := r.Context()
	calendar, found, err := h.store.GetSymbolCalendar(ctx, symbol)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to lookup symbol")
		return
	}
	if !found {
		WriteError(w, http.StatusNotFound, "not_found", "symbol not found")
		return
	}

	candles, hasMore, err := h.store.QueryCandles(ctx, calendar, timeframe, symbol, from, to, page, limit)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to query candles")
		return
	}

	WriteSuccessWithMetadata(w, http.StatusOK, model.CandlesResponse{
		Symbol:   symbol,
		Calendar: calendar,
		Candles:  candles,
	}, model.Metadata{
		Page:    page,
		Limit:   limit,
		HasMore: hasMore,
		Count:   len(candles),
	})
}
