package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/iranrates/marketick/internal/model"
)

func (c *Client) QueryTicks(
	ctx context.Context,
	calendar model.Calendar,
	symbol string,
	from time.Time,
	to time.Time,
	page int,
	limit int,
) ([]model.TickRecord, bool, error) {
	limitRows := limit + 1
	offset := (page - 1) * limit
	fromStr := from.Format("2006-01-02")
	toStr := to.Format("2006-01-02")

	var query string
	if calendar == model.CalendarJalali {
		query = `
			SELECT
				toString(value) AS value,
				toString(volume) AS volume,
				toString(ts) AS ts
			FROM ticks
			WHERE symbol = ?
			  AND toDate32(toTimezone(ts, 'Asia/Tehran')) >= toDate32(?)
			  AND toDate32(toTimezone(ts, 'Asia/Tehran')) <= toDate32(?)
			ORDER BY ts
			LIMIT ? OFFSET ?
		`
	} else {
		query = `
			SELECT
				toString(value) AS value,
				toString(volume) AS volume,
				toString(ts) AS ts
			FROM ticks
			WHERE symbol = ?
			  AND toDate(ts) >= toDate(?)
			  AND toDate(ts) <= toDate(?)
			ORDER BY ts
			LIMIT ? OFFSET ?
		`
	}

	rows, err := c.conn.Query(ctx, query, symbol, fromStr, toStr, limitRows, offset)
	if err != nil {
		return nil, false, fmt.Errorf("query ticks: %w", err)
	}
	defer rows.Close()

	var ticks []model.TickRecord
	for rows.Next() {
		var tick model.TickRecord
		if err := rows.Scan(&tick.Value, &tick.Volume, &tick.Timestamp); err != nil {
			return nil, false, fmt.Errorf("scan tick: %w", err)
		}
		ticks = append(ticks, tick)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate ticks: %w", err)
	}

	hasMore := len(ticks) > limit
	if hasMore {
		ticks = ticks[:limit]
	}
	if ticks == nil {
		ticks = []model.TickRecord{}
	}

	return ticks, hasMore, nil
}
