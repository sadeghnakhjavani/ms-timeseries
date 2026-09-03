package clickhouse

import (
	"context"
	"fmt"
	"strings"

	"github.com/iranrates/ms-timeseries/internal/model"
)

func (c *Client) GetSymbolCalendars(ctx context.Context, symbols []string) (map[string]model.Calendar, error) {
	if len(symbols) == 0 {
		return map[string]model.Calendar{}, nil
	}

	placeholders := make([]string, len(symbols))
	args := make([]interface{}, len(symbols))
	for i, s := range symbols {
		placeholders[i] = "?"
		args[i] = s
	}

	query := fmt.Sprintf(
		"SELECT symbol, calendar FROM symbols FINAL WHERE symbol IN (%s)",
		strings.Join(placeholders, ", "),
	)

	rows, err := c.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query symbols: %w", err)
	}
	defer rows.Close()

	result := make(map[string]model.Calendar, len(symbols))
	for rows.Next() {
		var symbol string
		var calendar string
		if err := rows.Scan(&symbol, &calendar); err != nil {
			return nil, fmt.Errorf("scan symbol: %w", err)
		}
		result[symbol] = model.Calendar(calendar)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate symbols: %w", err)
	}

	return result, nil
}

func (c *Client) RegisterSymbols(ctx context.Context, registrations map[string]model.Calendar) error {
	if len(registrations) == 0 {
		return nil
	}

	batch, err := c.conn.PrepareBatch(ctx, "INSERT INTO symbols (symbol, calendar)")
	if err != nil {
		return fmt.Errorf("prepare symbol batch: %w", err)
	}

	for symbol, calendar := range registrations {
		if err := batch.Append(symbol, string(calendar)); err != nil {
			return fmt.Errorf("append symbol: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("send symbol batch: %w", err)
	}

	return nil
}

func (c *Client) GetSymbolCalendar(ctx context.Context, symbol string) (model.Calendar, bool, error) {
	var calendar string
	err := c.conn.QueryRow(ctx,
		"SELECT calendar FROM symbols FINAL WHERE symbol = ?",
		symbol,
	).Scan(&calendar)
	if err != nil {
		if isNoRows(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("query symbol calendar: %w", err)
	}
	return model.Calendar(calendar), true, nil
}

func isNoRows(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no rows")
}
