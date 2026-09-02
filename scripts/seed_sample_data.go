package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"strings"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type symbolSpec struct {
	name      string
	calendar  string
	start     time.Time
	basePrice float64
	dailyVol  float64
	trend     float64
}

type tickRow struct {
	symbol   string
	calendar string
	value    string
	volume   string
	ts       time.Time
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

func main() {
	host := envOrDefault("CH_HOST", "localhost")
	port := envOrDefault("CH_PORT", "9000")
	user := envOrDefault("CH_USER", "default")
	password := os.Getenv("CH_PASSWORD")
	database := envOrDefault("CH_DATABASE", "default")

	conn, err := ch.Open(&ch.Options{
		Addr: []string{fmt.Sprintf("%s:%s", host, port)},
		Auth: ch.Auth{
			Database: database,
			Username: user,
			Password: password,
		},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatalf("open clickhouse: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Ping(ctx); err != nil {
		log.Fatalf("ping clickhouse: %v", err)
	}

	if err := verifyJalaliCalendar(ctx, conn); err != nil {
		log.Fatalf("jalali calendar check: %v", err)
	}

	tehran, err := time.LoadLocation("Asia/Tehran")
	if err != nil {
		log.Fatalf("load Asia/Tehran: %v", err)
	}

	end := time.Now().UTC().Truncate(24 * time.Hour)
	specs := []symbolSpec{
		{
			name:      "BTCUSDT",
			calendar:  "gregorian",
			start:     time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			basePrice: 7200,
			dailyVol:  0.04,
			trend:     0.0008,
		},
		{
			name:      "USDIRR",
			calendar:  "jalali",
			start:     time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
			basePrice: 8500,
			dailyVol:  0.015,
			trend:     0.0012,
		},
	}

	rng := rand.New(rand.NewSource(42))

	for _, spec := range specs {
		if err := cleanSymbol(ctx, conn, spec.name); err != nil {
			log.Fatalf("clean %s: %v", spec.name, err)
		}
		if err := registerSymbol(ctx, conn, spec.name, spec.calendar); err != nil {
			log.Fatalf("register %s: %v", spec.name, err)
		}

		ticks := generateTicks(spec, end, tehran, rng)
		if err := insertTicks(ctx, conn, ticks); err != nil {
			log.Fatalf("insert %s ticks: %v", spec.name, err)
		}

		log.Printf("seeded %s (%s): %d ticks from %s to %s",
			spec.name,
			spec.calendar,
			len(ticks),
			spec.start.Format("2006-01-02"),
			end.Format("2006-01-02"),
		)
	}
}

func verifyJalaliCalendar(ctx context.Context, conn driver.Conn) error {
	var count uint64
	if err := conn.QueryRow(ctx, "SELECT count() FROM jalali_calendar").Scan(&count); err != nil {
		return fmt.Errorf("query jalali_calendar: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("jalali_calendar is empty; run seed_jalali_calendar.go first")
	}
	return nil
}

func cleanSymbol(ctx context.Context, conn driver.Conn, symbol string) error {
	for _, table := range partitionedTables {
		query := fmt.Sprintf("ALTER TABLE %s DROP PARTITION '%s'", table, escapePartition(symbol))
		if err := conn.Exec(ctx, query); err != nil {
			if !isBenignPartitionError(err) {
				return fmt.Errorf("drop partition from %s: %w", table, err)
			}
		}
	}

	if err := conn.Exec(ctx,
		"ALTER TABLE symbols DELETE WHERE symbol = ? SETTINGS mutations_sync = 2",
		symbol,
	); err != nil {
		return fmt.Errorf("delete symbol row: %w", err)
	}

	log.Printf("cleaned existing data for %s", symbol)
	return nil
}

func registerSymbol(ctx context.Context, conn driver.Conn, symbol, calendar string) error {
	if err := conn.Exec(ctx, "INSERT INTO symbols (symbol, calendar) VALUES (?, ?)", symbol, calendar); err != nil {
		return fmt.Errorf("insert symbol: %w", err)
	}
	return nil
}

func generateTicks(spec symbolSpec, end time.Time, tehran *time.Location, rng *rand.Rand) []tickRow {
	var ticks []tickRow
	price := spec.basePrice

	for day := spec.start; !day.After(end); day = day.AddDate(0, 0, 1) {
		tickCount := rng.Intn(7) + 4
		dailyChange := (rng.Float64()*2-1)*spec.dailyVol + spec.trend
		dayOpen := price
		price = price * (1 + dailyChange)
		if price <= 0 {
			price = dayOpen
		}

		for i := 0; i < tickCount; i++ {
			frac := float64(i+1) / float64(tickCount)
			tickPrice := dayOpen + (price-dayOpen)*frac
			tickPrice *= 1 + 0.002*(rng.Float64()-0.5)
			tickPrice = math.Max(tickPrice, 0.01)

			ts := randomTimestamp(day, spec.calendar, tehran, rng)
			volume := 100 + rng.Float64()*9900

			ticks = append(ticks, tickRow{
				symbol:   spec.name,
				calendar: spec.calendar,
				value:    formatDecimal(tickPrice),
				volume:   formatDecimal(volume),
				ts:       ts,
			})
		}
	}

	return ticks
}

func randomTimestamp(day time.Time, calendar string, tehran *time.Location, rng *rand.Rand) time.Time {
	if calendar == "jalali" {
		dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, tehran)
		offset := time.Duration(rng.Intn(86400)) * time.Second
		return dayStart.Add(offset).UTC()
	}

	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	offset := time.Duration(rng.Intn(86400)) * time.Second
	return dayStart.Add(offset)
}

func formatDecimal(v float64) string {
	return fmt.Sprintf("%.8f", v)
}

func insertTicks(ctx context.Context, conn driver.Conn, ticks []tickRow) error {
	const batchSize = 10000

	for offset := 0; offset < len(ticks); offset += batchSize {
		end := offset + batchSize
		if end > len(ticks) {
			end = len(ticks)
		}
		if err := insertTickBatch(ctx, conn, ticks[offset:end]); err != nil {
			return err
		}
	}
	return nil
}

func insertTickBatch(ctx context.Context, conn driver.Conn, rows []tickRow) error {
	batch, err := conn.PrepareBatch(ctx, "INSERT INTO ticks (symbol, calendar, value, volume, ts)")
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}

	for _, row := range rows {
		if err := batch.Append(row.symbol, row.calendar, row.value, row.volume, row.ts); err != nil {
			return fmt.Errorf("append tick: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("send batch: %w", err)
	}
	return nil
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
		strings.Contains(msg, "NO_SUCH_DATA_PART") ||
		strings.Contains(msg, "Partition does not exist")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
