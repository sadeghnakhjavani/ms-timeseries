package clickhouse

import (
	"context"
	"fmt"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/iranrates/marketick/internal/config"
)

type Client struct {
	conn driver.Conn
	db   string
}

func NewClient(cfg config.Config) (*Client, error) {
	conn, err := ch.Open(&ch.Options{
		Addr: []string{fmt.Sprintf("%s:%d", cfg.CHHost, cfg.CHPort)},
		Auth: ch.Auth{
			Database: cfg.CHDatabase,
			Username: cfg.CHUser,
			Password: cfg.CHPassword,
		},
		DialTimeout:     5 * time.Second,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
	})
	if err != nil {
		return nil, fmt.Errorf("open clickhouse: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping clickhouse: %w", err)
	}

	return &Client{conn: conn, db: cfg.CHDatabase}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Conn() driver.Conn {
	return c.conn
}

var requiredTables = []string{
	"ticks",
	"candles_1d_greg",
	"candles_1w_greg",
	"candles_1m_greg",
	"candles_1y_greg",
	"candles_1d_jalali",
	"candles_1w_jalali",
	"candles_1m_jalali",
	"candles_1y_jalali",
	"symbols",
	"jalali_calendar",
}

func (c *Client) StartupChecks(ctx context.Context) error {
	for _, table := range requiredTables {
		var count uint64
		query := fmt.Sprintf("SELECT count() FROM %s LIMIT 1", table)
		if err := c.conn.QueryRow(ctx, query).Scan(&count); err != nil {
			return fmt.Errorf("table %s not available: %w", table, err)
		}
	}

	var dictExists uint64
	if err := c.conn.QueryRow(ctx,
		"SELECT count() FROM system.dictionaries WHERE name = 'jalali_calendar_dict' AND database = ?",
		c.db,
	).Scan(&dictExists); err != nil {
		return fmt.Errorf("check dictionary: %w", err)
	}
	if dictExists == 0 {
		return fmt.Errorf("dictionary jalali_calendar_dict not available")
	}

	var calCount uint64
	if err := c.conn.QueryRow(ctx, "SELECT count() FROM jalali_calendar").Scan(&calCount); err != nil {
		return fmt.Errorf("check jalali_calendar: %w", err)
	}
	if calCount == 0 {
		return fmt.Errorf("jalali_calendar is empty; run: docker compose exec marketick /app/jalali-seed")
	}

	return nil
}
