package clickhouse

import (
	"context"
	"fmt"

	"github.com/iranrates/ms-timeseries/internal/model"
)

func (c *Client) InsertTicks(ctx context.Context, ticks []model.Tick) error {
	if len(ticks) == 0 {
		return nil
	}

	batch, err := c.conn.PrepareBatch(ctx,
		"INSERT INTO ticks (symbol, calendar, value, volume, ts)",
	)
	if err != nil {
		return fmt.Errorf("prepare tick batch: %w", err)
	}

	for _, tick := range ticks {
		if err := batch.Append(
			tick.Symbol,
			string(tick.Calendar),
			tick.Value,
			tick.Volume,
			tick.TS,
		); err != nil {
			return fmt.Errorf("append tick: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("send tick batch: %w", err)
	}

	return nil
}
