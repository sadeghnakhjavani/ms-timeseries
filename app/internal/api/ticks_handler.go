package api

import (
	"encoding/json"
	"io"
	"net/http"

	chstore "github.com/iranrates/marketick/internal/clickhouse"
	"github.com/iranrates/marketick/internal/model"
)

type TicksHandler struct {
	store      *chstore.Client
	pagination PaginationSettings
}

func NewTicksHandler(store *chstore.Client, pagination PaginationSettings) *TicksHandler {
	return &TicksHandler{
		store:      store,
		pagination: pagination,
	}
}

func (h *TicksHandler) Read(w http.ResponseWriter, r *http.Request) {
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

	ticks, hasMore, err := h.store.QueryTicks(ctx, calendar, symbol, from, to, page, limit)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to query ticks")
		return
	}

	WriteSuccessWithMetadata(w, http.StatusOK, model.TicksResponse{
		Symbol:   symbol,
		Calendar: calendar,
		Ticks:    ticks,
	}, model.Metadata{
		Page:    page,
		Limit:   limit,
		HasMore: hasMore,
		Count:   len(ticks),
	})
}

func (h *TicksHandler) Ingest(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	var req model.IngestRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_request", "malformed JSON body")
		return
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		WriteError(w, http.StatusBadRequest, "invalid_request", "malformed JSON body")
		return
	}

	if err := validateIngestRequest(req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	if symbol, a, b, conflict := detectBatchCalendarConflicts(req.Ticks); conflict {
		WriteError(w, http.StatusBadRequest, "calendar_mismatch",
			"symbol "+symbol+" has conflicting calendars in batch: "+string(a)+" and "+string(b))
		return
	}

	ctx := r.Context()
	symbols := distinctSymbols(req.Ticks)
	stored, err := h.store.GetSymbolCalendars(ctx, symbols)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to lookup symbols")
		return
	}

	if symbol, registered, received, mismatch := validateAgainstStored(req.Ticks, stored); mismatch {
		WriteError(w, http.StatusBadRequest, "calendar_mismatch",
			"symbol "+symbol+" is registered as "+string(registered)+" but received "+string(received))
		return
	}

	expected := expectedCalendars(req.Ticks)
	toRegister := newSymbolsToRegister(expected, stored)
	if len(toRegister) > 0 {
		if err := h.store.RegisterSymbols(ctx, toRegister); err != nil {
			WriteError(w, http.StatusInternalServerError, "internal_error", "failed to register symbols")
			return
		}

		stored, err = h.store.GetSymbolCalendars(ctx, symbols)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal_error", "failed to re-check symbols")
			return
		}

		if symbol, registered, received, mismatch := validateAgainstStored(req.Ticks, stored); mismatch {
			WriteError(w, http.StatusBadRequest, "calendar_mismatch",
				"symbol "+symbol+" is registered as "+string(registered)+" but received "+string(received))
			return
		}
	}

	ticks := ticksToModel(req.Ticks)
	if err := h.store.InsertTicks(ctx, ticks); err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to insert ticks")
		return
	}

	WriteSuccess(w, http.StatusCreated, model.IngestResponse{Accepted: len(ticks)})
}
