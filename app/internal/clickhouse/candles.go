package clickhouse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/iranrates/marketick/internal/model"
)

type candleTable struct {
	name     string
	isJalali bool
}

var candleTableMap = map[model.Calendar]map[string]candleTable{
	model.CalendarGregorian: {
		"1d": {name: "candles_1d_greg", isJalali: false},
		"1w": {name: "candles_1w_greg", isJalali: false},
		"1m": {name: "candles_1m_greg", isJalali: false},
		"1y": {name: "candles_1y_greg", isJalali: false},
	},
	model.CalendarJalali: {
		"1d": {name: "candles_1d_jalali", isJalali: true},
		"1w": {name: "candles_1w_jalali", isJalali: true},
		"1m": {name: "candles_1m_jalali", isJalali: true},
		"1y": {name: "candles_1y_jalali", isJalali: true},
	},
}

func ResolveCandleTable(calendar model.Calendar, timeframe string) (candleTable, bool) {
	byTF, ok := candleTableMap[calendar]
	if !ok {
		return candleTable{}, false
	}
	table, ok := byTF[timeframe]
	return table, ok
}

func (c *Client) QueryCandles(
	ctx context.Context,
	calendar model.Calendar,
	timeframe string,
	symbol string,
	from time.Time,
	to time.Time,
	page int,
	limit int,
) ([]model.Candle, bool, error) {
	table, ok := ResolveCandleTable(calendar, timeframe)
	if !ok {
		return nil, false, fmt.Errorf("invalid candle table mapping")
	}

	limitRows := limit + 1
	offset := (page - 1) * limit

	var query string
	if table.isJalali {
		query = fmt.Sprintf(`
			SELECT
				toString(bucket_start) AS bucket_start_str,
				toString(argMinMerge(open))  AS open,
				toString(maxMerge(high))     AS high,
				toString(minMerge(low))      AS low,
				toString(argMaxMerge(close)) AS close,
				toString(sumMerge(volume))   AS volume,
				countMerge(ticks)            AS tick_count,
				dictGet('jalali_calendar_dict', 'jalali_year', bucket_start)  AS jalali_year,
				dictGet('jalali_calendar_dict', 'jalali_month', bucket_start) AS jalali_month,
				dictGet('jalali_calendar_dict', 'jalali_day', bucket_start)   AS jalali_day
			FROM %s
			WHERE symbol = ?
			  AND bucket_start >= toDate32(?)
			  AND bucket_start <= toDate32(?)
			GROUP BY bucket_start
			ORDER BY bucket_start
			LIMIT ? OFFSET ?
		`, table.name)
	} else {
		query = fmt.Sprintf(`
			SELECT
				toString(bucket_start) AS bucket_start_str,
				toString(argMinMerge(open))  AS open,
				toString(maxMerge(high))     AS high,
				toString(minMerge(low))      AS low,
				toString(argMaxMerge(close)) AS close,
				toString(sumMerge(volume))   AS volume,
				countMerge(ticks)            AS tick_count
			FROM %s
			WHERE symbol = ?
			  AND bucket_start >= ?
			  AND bucket_start <= ?
			GROUP BY bucket_start
			ORDER BY bucket_start
			LIMIT ? OFFSET ?
		`, table.name)
	}

	fromStr := from.Format("2006-01-02")
	toStr := to.Format("2006-01-02")

	rows, err := c.conn.Query(ctx, query, symbol, fromStr, toStr, limitRows, offset)
	if err != nil {
		return nil, false, fmt.Errorf("query candles: %w", err)
	}
	defer rows.Close()

	var candles []model.Candle
	for rows.Next() {
		var candle model.Candle
		if table.isJalali {
			var jy uint16
			var jm, jd uint8
			if err := rows.Scan(
				&candle.BucketStart,
				&candle.Open,
				&candle.High,
				&candle.Low,
				&candle.Close,
				&candle.Volume,
				&candle.TickCount,
				&jy,
				&jm,
				&jd,
			); err != nil {
				return nil, false, fmt.Errorf("scan jalali candle: %w", err)
			}
			candle.JalaliYear = &jy
			candle.JalaliMonth = &jm
			candle.JalaliDay = &jd
		} else {
			if err := rows.Scan(
				&candle.BucketStart,
				&candle.Open,
				&candle.High,
				&candle.Low,
				&candle.Close,
				&candle.Volume,
				&candle.TickCount,
			); err != nil {
				return nil, false, fmt.Errorf("scan gregorian candle: %w", err)
			}
		}
		candles = append(candles, candle)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate candles: %w", err)
	}

	hasMore := len(candles) > limit
	if hasMore {
		candles = candles[:limit]
	}

	if candles == nil {
		candles = []model.Candle{}
	}

	return candles, hasMore, nil
}

var partitionedTables = []string{
	"ticks",
	"candles_1d_greg",
	"candles_1w_greg",
	"candles_1m_greg",
	"candles_1y_greg",
	"candles_1d_jalali",
	"candles_1w_jalali",
	"candles_1m_jalali",
	"candles_1y_jalali",
}

func (c *Client) DeleteSymbol(ctx context.Context, symbol string) (map[string]string, error) {
	summary := make(map[string]string, len(partitionedTables)+1)

	for _, table := range partitionedTables {
		query := fmt.Sprintf("ALTER TABLE %s DROP PARTITION '%s'", table, escapePartition(symbol))
		if err := c.conn.Exec(ctx, query); err != nil {
			if isBenignPartitionError(err) {
				summary[table] = "no-op"
				continue
			}
			return nil, fmt.Errorf("drop partition from %s: %w", table, err)
		}
		summary[table] = "dropped"
	}

	if err := c.conn.Exec(ctx,
		"ALTER TABLE symbols DELETE WHERE symbol = ? SETTINGS mutations_sync = 2",
		symbol,
	); err != nil {
		return nil, fmt.Errorf("delete symbol row: %w", err)
	}
	summary["symbols"] = "deleted"

	return summary, nil
}

func escapePartition(symbol string) string {
	return strings.ReplaceAll(symbol, "'", "\\'")
}

func isBenignPartitionError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "doesn't exist") ||
		strings.Contains(msg, "NO_SUCH_DATA_PART")
}
