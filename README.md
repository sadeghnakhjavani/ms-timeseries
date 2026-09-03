# MS-TIMESERIES

Financial microservice that ingests market ticks and serves OHLCV candles (`1d`, `1w`, `1m`, `1y`) in both Gregorian and Jalali calendars, backed by ClickHouse.

## Architecture

Single container:

- **ms-timeseries** — Go HTTP API + ClickHouse (tick storage, live aggregation via materialized views, symbol registry, Jalali calendar dictionary)

```
                    ┌──────────────────────┐
                    │       Client         │
                    └──────────┬───────────┘
                               │ X-API-Key
                               ▼
                    ┌──────────────────────┐
                    │     ms-timeseries      │
                    │  Go API + ClickHouse │
                    └──────────┬───────────┘
                               │
                 ┌─────────────┴─────────────┐
                 ▼                           ▼
          POST /ticks                 GET /candles
                 │                           │
                 ▼                           ▼
             symbols                   candle tables
                 │                 (Gregorian / Jalali)
                 ▼
               ticks ──► materialized views ──► candles
```

Incoming ticks are written only to the `ticks` table. Eight materialized views aggregate ticks into candle tables at insert time using `AggregatingMergeTree` with `-State`/`-Merge` combinators. No scheduled aggregation workers exist.

## Quick start

```bash
cp .env.example .env
# Edit .env and set API_KEY

docker compose up -d --build
```

On first start, if `jalali_calendar` is empty the container auto-runs `/app/jalali-seed` before the API starts.

## Usage

Start or rebuild:

```bash
docker compose up -d --build
```

Stop and remove the container:

```bash
docker compose down --remove-orphans
```

### Exposed ports

| Port | Service |
|------|---------|
| `8080` | HTTP API |
| `8123` | ClickHouse HTTP UI |
| `9000` | ClickHouse native |

Host port `8080` can be changed via `APP_PORT` in `.env`.

### Seed commands

Reload the Jalali calendar (truncates and reloads `jalali_calendar`):

```bash
docker compose exec ms-timeseries /app/jalali-seed
```

Load sample BTCUSDT / USDIRR tick data:

```bash
docker compose exec ms-timeseries /app/sample-seed
```

Drop and recreate the ClickHouse database (destructive — removes all ticks, candles, and symbols):

```bash
docker compose exec ms-timeseries /app/drop-database
docker compose exec ms-timeseries /app/jalali-seed
```

Or restart the container after `drop-database`; the entrypoint auto-runs `jalali-seed` when `jalali_calendar` is empty.

Locally (requires ClickHouse on `localhost`):

```bash
cd scripts
CH_HOST=localhost go run drop_database.go
CH_HOST=localhost go run seed_jalali_calendar.go
```

## Environment variables

| Variable     | Description                          | Default      |
|--------------|--------------------------------------|--------------|
| `CH_HOST`    | ClickHouse host (use `localhost` in the container) | `localhost`  |
| `CH_PORT`    | ClickHouse native port               | `9000`       |
| `CH_USER`    | ClickHouse user                      | `default`    |
| `CH_PASSWORD`| ClickHouse password                  | (empty)      |
| `CH_DATABASE`| ClickHouse database                  | `default`    |
| `API_KEY`    | Required API key for all endpoints   | —            |
| `APP_PORT`   | Host port mapped to the app          | `8080`       |
| `PAGE_DEFAULT_LIMIT` | Default `limit` for paginated queries when omitted | `1000` |
| `PAGE_MAX_LIMIT` | Hard cap for client `limit` parameter | `1000` |

The application exits on startup if `API_KEY` is missing or empty, if ClickHouse is unreachable, if required tables/dictionary are missing, or if `jalali_calendar` is empty.

## Authentication

All endpoints require:

```http
X-API-Key: <API_KEY>
```

Missing, empty, or incorrect keys return `401 Unauthorized`. Authentication runs before request validation.

## API endpoints

There are exactly four endpoints. No Swagger/OpenAPI, no `/docs`, no `/openapi.json`.

### POST `/api/v1/ticks`

Ingest a batch of ticks.

**Request:**

```json
{
  "ticks": [
    {
      "symbol": "ABC",
      "value": "123.45000000",
      "volume": "1000.00000000",
      "calendar": "gregorian",
      "timestamp": "2026-09-02T10:15:30.123Z"
    }
  ]
}
```

**Success (`201 Created`):**

```json
{
  "data": {
    "accepted": 1
  }
}
```

**Validation failure (`400 Bad Request`):**

```json
{
  "error": {
    "code": "calendar_mismatch",
    "message": "symbol ABC is registered as gregorian but received jalali"
  }
}
```

The entire batch is validated before any write. If any tick fails validation or calendar rules, **no tick is inserted and no symbol is registered**.

### GET `/api/v1/ticks`

Query raw ticks for a symbol within a date range. Calendar is derived server-side from the `symbols` table — clients must **not** send `calendar`.

**Query parameters:** `symbol`, `from`, `to`, `page` (default `1`), `limit` (default from `PAGE_DEFAULT_LIMIT`, max from `PAGE_MAX_LIMIT`)

Sending `calendar` returns `400`.

Day boundaries follow the symbol calendar: UTC dates for gregorian symbols, `Asia/Tehran` dates for jalali symbols.

Raw ticks expire after **7 days** (see [Raw tick TTL](#raw-tick-ttl)). Use candles for historical data beyond that window.

**Success (`200 OK`):**

```json
{
  "data": {
    "symbol": "ABC",
    "calendar": "gregorian",
    "ticks": [
      {
        "value": "123.45000000",
        "volume": "1000.00000000",
        "timestamp": "2026-09-02 10:15:30.123"
      }
    ]
  },
  "metadata": {
    "page": 1,
    "limit": 1000,
    "has_more": false,
    "count": 1
  }
}
```

Unknown symbols return `404`.

### GET `/api/v1/candles`

Query OHLCV candles. Calendar is derived server-side from the `symbols` table — clients must **not** send `calendar`.

**Query parameters:** `symbol`, `from`, `to`, `timeframe` (`1d`, `1w`, `1m`, `1y`), `page` (default `1`), `limit` (default from `PAGE_DEFAULT_LIMIT`, max from `PAGE_MAX_LIMIT`)

Sending `calendar` returns `400`.

**Success (`200 OK`):**

```json
{
  "data": {
    "symbol": "ABC",
    "calendar": "gregorian",
    "candles": [
      {
        "bucket_start": "2026-09-02 00:00:00",
        "open": "123.45000000",
        "high": "123.45000000",
        "low": "123.45000000",
        "close": "123.45000000",
        "volume": "1000.00000000",
        "tick_count": 1
      }
    ]
  },
  "metadata": {
    "page": 1,
    "limit": 1000,
    "has_more": false,
    "count": 1
  }
}
```

Metadata uses `LIMIT`/`OFFSET` with a `limit + 1` probe to set `has_more` — no total-count query is run. `count` is the number of items returned in `data` for the current page (e.g. length of the `candles` or `ticks` array).

Jalali symbols include `jalali_year`, `jalali_month`, and `jalali_day` on each candle.

Unknown symbols return `404`.

### DELETE `/api/v1/symbols/{symbol}`

Delete a symbol from all application tables (`ticks`, eight candle tables, `symbols`). The global `jalali_calendar` table is never modified. Deleting a non-existent symbol is a successful no-op.

**Success (`200 OK`):**

```json
{
  "data": {
    "symbol": "ABC",
    "deleted": {
      "ticks": "dropped",
      "candles_1d_greg": "dropped",
      "symbols": "deleted"
    }
  }
}
```

## Calendar semantics

Each symbol has exactly **one immutable calendar** (`gregorian` or `jalali`), set on first successful ingest:

- **First registration wins** — later ticks with a different calendar reject the entire batch.
- Two new ticks for the same symbol with different calendars in one batch are rejected entirely.
- There is no `market` field anywhere.

### Gregorian candles

- UTC timezone
- `1d` — UTC midnight (`toStartOfDay`)
- `1w` — Monday-start ISO week (`toStartOfWeek(ts, 1)`)
- `1m` — Gregorian month start
- `1y` — Gregorian year start

### Jalali candles

- `Asia/Tehran` (+03:30)
- Saturday-start weeks
- Month/year boundaries from precomputed `jalali_calendar` dictionary
- No runtime Jalali arithmetic in the HTTP application

## Raw tick TTL

Raw ticks in the `ticks` table expire after **7 days** (`TTL toDateTime(ts) + INTERVAL 7 DAY`). This is safe because materialized views aggregate every inserted tick into candle tables immediately. Candle aggregates and `symbols` rows are unaffected by tick TTL.

### Manual TTL monitoring

Run periodically as an early-warning check:

```sql
SELECT count()
FROM ticks
WHERE ts < now() - INTERVAL 6 DAY;
```

## ClickHouse version

Pinned image base: `clickhouse:26.3.25.2-jammy` (ClickHouse **26.3.25.2**). The Go API and ClickHouse run together in the `ms-timeseries` container. See [Exposed ports](#exposed-ports) above.

## Development

Build the app locally (requires Go 1.22+):

```bash
cd app
go build -o ms-timeseries .
```

Run the seed script (requires ClickHouse):

```bash
cd scripts
go run seed_jalali_calendar.go
```

Run tests/build inside Docker:

```bash
docker compose build
docker compose up -d
```

See [Seed commands](#seed-commands) for calendar and sample data scripts.

## Transaction semantics

Validation failures are atomic: no writes occur.

ClickHouse does not provide a conventional multi-statement transaction spanning symbol registration and tick insertion. If an infrastructure failure occurs after symbol registration but before tick insertion, the API does not claim database-level transactionality. The normal contract is that validation rejection causes zero writes.
