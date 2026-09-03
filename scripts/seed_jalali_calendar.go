package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	ptime "github.com/yaa110/go-persian-calendar"
)

type calendarRow struct {
	gregorianDate        time.Time
	jalaliYear           uint16
	jalaliMonth          uint8
	jalaliDay            uint8
	jalaliWeekStartDate  time.Time
	jalaliMonthStartDate time.Time
	jalaliYearStartDate  time.Time
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

	start := time.Date(1910, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)

	verifyReferences(start)

	if err := cleanTable(ctx, conn); err != nil {
		log.Fatalf("clean jalali_calendar: %v", err)
	}

	const batchSize = 10000
	var batch []calendarRow

	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		row := buildRow(d)
		batch = append(batch, row)
		if len(batch) >= batchSize {
			if err := insertBatch(ctx, conn, batch); err != nil {
				log.Fatalf("insert batch: %v", err)
			}
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		if err := insertBatch(ctx, conn, batch); err != nil {
			log.Fatalf("insert final batch: %v", err)
		}
	}

	if err := conn.Exec(ctx, "SYSTEM RELOAD DICTIONARY jalali_calendar_dict"); err != nil {
		log.Fatalf("reload dictionary: %v", err)
	}

	var count uint64
	if err := conn.QueryRow(ctx, "SELECT count() FROM jalali_calendar").Scan(&count); err != nil {
		log.Fatalf("count rows: %v", err)
	}

	log.Printf("seeded %d jalali calendar rows", count)
}

func buildRow(date time.Time) calendarRow {
	pt := ptime.New(date)
	jy := uint16(pt.Year())
	jm := uint8(pt.Month())
	jd := uint8(pt.Day())

	monthStart := ptime.Date(int(jy), ptime.Month(jm), 1, 0, 0, 0, 0, time.UTC).Time()
	yearStart := ptime.Date(int(jy), ptime.Farvardin, 1, 0, 0, 0, 0, time.UTC).Time()
	weekStart := saturdayWeekStart(date)

	return calendarRow{
		gregorianDate:        date,
		jalaliYear:           jy,
		jalaliMonth:          jm,
		jalaliDay:            jd,
		jalaliWeekStartDate:  weekStart,
		jalaliMonthStartDate: monthStart,
		jalaliYearStartDate:  yearStart,
	}
}

func saturdayWeekStart(date time.Time) time.Time {
	offset := (int(date.Weekday()) - int(time.Saturday) + 7) % 7
	return date.AddDate(0, 0, -offset)
}

func formatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

func cleanTable(ctx context.Context, conn driver.Conn) error {
	if err := conn.Exec(ctx, "TRUNCATE TABLE IF EXISTS jalali_calendar"); err != nil {
		return fmt.Errorf("truncate: %w", err)
	}

	var count uint64
	if err := conn.QueryRow(ctx, "SELECT count() FROM jalali_calendar").Scan(&count); err != nil {
		return fmt.Errorf("verify empty: %w", err)
	}
	if count != 0 {
		return fmt.Errorf("table not empty after truncate: %d rows remain", count)
	}

	log.Println("cleaned jalali_calendar")
	return nil
}

func insertBatch(ctx context.Context, conn driver.Conn, rows []calendarRow) error {
	batch, err := conn.PrepareBatch(ctx, `
		INSERT INTO jalali_calendar (
			gregorian_date,
			jalali_year,
			jalali_month,
			jalali_day,
			jalali_week_start_date,
			jalali_month_start_date,
			jalali_year_start_date
		)
	`)
	if err != nil {
		return err
	}

	for _, row := range rows {
		if err := batch.Append(
			formatDate(row.gregorianDate),
			row.jalaliYear,
			row.jalaliMonth,
			row.jalaliDay,
			formatDate(row.jalaliWeekStartDate),
			formatDate(row.jalaliMonthStartDate),
			formatDate(row.jalaliYearStartDate),
		); err != nil {
			return err
		}
	}

	return batch.Send()
}

func verifyReferences(start time.Time) {
	// Saturday maps to itself.
	saturday := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	if !saturdayWeekStart(saturday).Equal(saturday) {
		log.Fatalf("Saturday week start mismatch for %s", saturday.Format("2006-01-02"))
	}

	// Known Jalali month boundary: 1404/01/01 -> 2025-03-21
	norooz := time.Date(2025, 3, 21, 0, 0, 0, 0, time.UTC)
	pt := ptime.New(norooz)
	if pt.Year() != 1404 || pt.Month() != 1 || pt.Day() != 1 {
		log.Fatalf("Norooz conversion mismatch: got %d/%d/%d", pt.Year(), pt.Month(), pt.Day())
	}

	// Known Jalali date for seed start: 1910-01-01 -> 1288/10/11
	ref := ptime.New(start)
	if ref.Year() != 1288 || ref.Month() != 10 || ref.Day() != 11 {
		log.Fatalf("1910-01-01 conversion mismatch: got %d/%d/%d", ref.Year(), ref.Month(), ref.Day())
	}

	// Saturday-start week: 2026-09-07 is Sunday, week starts 2026-09-05 (Saturday).
	sunday := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	expectedWeek := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	if !saturdayWeekStart(sunday).Equal(expectedWeek) {
		log.Fatalf("week start mismatch for %s", sunday.Format("2006-01-02"))
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
