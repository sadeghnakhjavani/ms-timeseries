package api

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/iranrates/marketick/internal/model"
)

const maxBodyBytes = 10 << 20 // 10 MiB

var allowedTimeframes = map[string]struct{}{
	"1d": {},
	"1w": {},
	"1m": {},
	"1y": {},
}

func validatePositiveDecimal(raw string, field string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("%s must be a positive decimal", field)
	}

	value, ok := new(big.Rat).SetString(raw)
	if !ok {
		return fmt.Errorf("%s must be a positive decimal", field)
	}
	if value.Sign() <= 0 {
		return fmt.Errorf("%s must be a positive decimal", field)
	}
	return nil
}

func validateTickInput(t model.TickInput) error {
	if strings.TrimSpace(t.Symbol) == "" {
		return fmt.Errorf("symbol must not be empty")
	}
	if !t.Calendar.Valid() {
		return fmt.Errorf("calendar must be gregorian or jalali")
	}
	if err := validatePositiveDecimal(t.Value, "value"); err != nil {
		return err
	}
	if err := validatePositiveDecimal(t.Volume, "volume"); err != nil {
		return err
	}
	if t.Timestamp.IsZero() {
		return fmt.Errorf("timestamp must be a valid RFC3339 value")
	}
	return nil
}

func validateIngestRequest(req model.IngestRequest) error {
	if len(req.Ticks) == 0 {
		return fmt.Errorf("ticks must not be empty")
	}
	for i, tick := range req.Ticks {
		if err := validateTickInput(tick); err != nil {
			return fmt.Errorf("ticks[%d]: %w", i, err)
		}
	}
	return nil
}

func validateTimeframe(tf string) error {
	if _, ok := allowedTimeframes[tf]; !ok {
		return fmt.Errorf("timeframe must be one of 1d, 1w, 1m, 1y")
	}
	return nil
}

func parseDateParam(raw string, field string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("%s is required", field)
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("%s must be a valid date", field)
}

func validateDateRange(from, to time.Time) error {
	if to.Before(from) {
		return fmt.Errorf("from must be before or equal to to")
	}
	return nil
}

func parsePageParam(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 1, nil
	}

	page, err := parsePositiveInt(raw, "page")
	if err != nil {
		return 0, err
	}
	return page, nil
}

func parseLimitParam(raw string, settings PaginationSettings) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return settings.DefaultLimit, nil
	}

	limit, err := parsePositiveInt(raw, "limit")
	if err != nil {
		return 0, err
	}
	if limit > settings.MaxLimit {
		return 0, fmt.Errorf("limit must not exceed %d", settings.MaxLimit)
	}
	return limit, nil
}

func parsePositiveInt(raw, field string) (int, error) {
	raw = strings.TrimSpace(raw)
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", field)
	}
	return value, nil
}

func detectBatchCalendarConflicts(ticks []model.TickInput) (string, model.Calendar, model.Calendar, bool) {
	expected := make(map[string]model.Calendar)
	for _, tick := range ticks {
		symbol := tick.Symbol
		if existing, ok := expected[symbol]; ok {
			if existing != tick.Calendar {
				return symbol, existing, tick.Calendar, true
			}
			continue
		}
		expected[symbol] = tick.Calendar
	}
	return "", "", "", false
}

func ticksToModel(inputs []model.TickInput) []model.Tick {
	out := make([]model.Tick, len(inputs))
	for i, t := range inputs {
		out[i] = model.Tick{
			Symbol:   t.Symbol,
			Calendar: t.Calendar,
			Value:    strings.TrimSpace(t.Value),
			Volume:   strings.TrimSpace(t.Volume),
			TS:       t.Timestamp.UTC(),
		}
	}
	return out
}

func distinctSymbols(ticks []model.TickInput) []string {
	seen := make(map[string]struct{})
	var symbols []string
	for _, tick := range ticks {
		if _, ok := seen[tick.Symbol]; ok {
			continue
		}
		seen[tick.Symbol] = struct{}{}
		symbols = append(symbols, tick.Symbol)
	}
	return symbols
}

func expectedCalendars(ticks []model.TickInput) map[string]model.Calendar {
	m := make(map[string]model.Calendar)
	for _, tick := range ticks {
		if _, ok := m[tick.Symbol]; !ok {
			m[tick.Symbol] = tick.Calendar
		}
	}
	return m
}

func validateAgainstStored(
	ticks []model.TickInput,
	stored map[string]model.Calendar,
) (string, model.Calendar, model.Calendar, bool) {
	for _, tick := range ticks {
		if cal, ok := stored[tick.Symbol]; ok && cal != tick.Calendar {
			return tick.Symbol, cal, tick.Calendar, true
		}
	}
	return "", "", "", false
}

func newSymbolsToRegister(
	expected map[string]model.Calendar,
	stored map[string]model.Calendar,
) map[string]model.Calendar {
	toRegister := make(map[string]model.Calendar)
	for symbol, calendar := range expected {
		if _, exists := stored[symbol]; !exists {
			toRegister[symbol] = calendar
		}
	}
	return toRegister
}
