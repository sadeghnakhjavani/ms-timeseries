# MARKETICK — Execution Plan for Cursor Agent

Repository name: `marketick`

Purpose: Financial microservice that ingests ticks and serves OHLCV candles (`1d/1w/1m/1y`) in both Gregorian and Jalali calendars, backed by ClickHouse.

This document is the **single source of truth** for the build. Every decision below is fixed. The Cursor agent must not make architectural or behavioral decisions that contradict this document.

---

## 0. Non-negotiable constraints

1. Two running containers only:
  - `clickhouse`
  - `app`
   No third running container (no cron/worker container).
2. Language: Go.
  The application uses **Go standard library only** for HTTP:
  - `net/http`
  - `encoding/json`
  - `net/url`
  - other standard-library packages as required
   No:
  - Gin
  - Echo
  - Fiber
  - chi
  - huma
  - humago
  - any other HTTP/router/API framework
3. **No Swagger/OpenAPI.**
  Do not generate:
  - Swagger UI
  - OpenAPI specification
  - `/docs`
  - `/openapi.json`
   Endpoint contracts are documented only in this plan and `README.md`.
4. ClickHouse aggregate storage uses:
  - `AggregatingMergeTree`
  - `-State` combinators in materialized views
  - `-Merge` combinators when querying
   No scheduled aggregation jobs are required. Materialized views process incoming rows on insert.
5. There are exactly **8 candle tables + 8 materialized views**:
  ### Gregorian
  - `1d`
  - `1w`
  - `1m`
  - `1y`
   Gregorian candles use:
  - UTC
  - Monday-start ISO weeks
  - native ClickHouse date functions directly on UTC `ts`
  ### Jalali
  - `1d`
  - `1w`
  - `1m`
  - `1y`
   Jalali candles use:
  - `Asia/Tehran`
  - Tehran's current fixed `+03:30` behavior
  - Saturday-start weeks
  - the precomputed `jalali_calendar` dictionary
   No runtime Jalali arithmetic is performed in application request handling or ClickHouse SQL.
6. Every tick contains an explicit `calendar`:
  - `gregorian`
  - `jalali`
   There is **no** `market` **field anywhere**.
   Calendar routing:
  - `gregorian` → 4 Gregorian candle MVs/tables
  - `jalali` → 4 Jalali candle MVs/tables
   A symbol has exactly **one immutable calendar**.
   The calendar is therefore required on ingest and stored with every tick.
   On read, the calendar is derived server-side from the `symbols` table. The client must never provide the calendar to the candle endpoint.
7. **Calendar mismatch rule**
  If a symbol already exists in `symbols` with calendar `gregorian`, a later tick for that symbol declaring `jalali` is invalid.
   Likewise, a symbol registered as `jalali` can never receive `gregorian` ticks.
   A calendar mismatch causes the **entire incoming batch to be rejected**.
   No tick from that batch is inserted.
   No new symbol from that batch is registered.
   No partial success is allowed.
8. Raw `ticks` data has a **7-day TTL**.
  The TTL must never cause candle data to disappear.
   Materialized views aggregate every inserted tick into the relevant candle tables at insert time.
9. Exactly 3 HTTP endpoints exist:
  ```text
   POST   /api/v1/ticks
   DELETE /api/v1/symbols/{symbol}
   GET    /api/v1/candles
  ```
10. All three endpoints require API Key authentication.
  The API key is supplied using:
    Missing or invalid API keys return:
    Authentication happens before endpoint-specific validation.
11. Tick schema:
  ```text
    symbol
    value
    volume
    calendar
    timestamp
  ```
    `timestamp` is interpreted/stored as UTC.

---



## Phase 1 — Repository scaffold

Create the following structure first:

```text
marketick/
├── docker-compose.yml
├── .env.example
├── README.md
│
├── clickhouse/
│   ├── init/
│   │   ├── 001_ticks.sql
│   │   ├── 002_jalali_calendar.sql
│   │   ├── 003_candles_gregorian.sql
│   │   ├── 004_candles_jalali.sql
│   │   ├── 005_symbols.sql
│   │   └── 006_ttl.sql
│   │
│   └── config/
│       └── config.xml
│
├── app/
│   ├── go.mod
│   ├── go.sum
│   ├── main.go
│   ├── Dockerfile
│   │
│   └── internal/
│       ├── config/
│       │   └── config.go
│       │
│       ├── clickhouse/
│       │   ├── client.go
│       │   ├── ingest.go
│       │   ├── candles.go
│       │   └── symbols.go
│       │
│       ├── api/
│       │   ├── router.go
│       │   ├── auth.go
│       │   ├── ticks_handler.go
│       │   ├── candles_handler.go
│       │   ├── symbols_handler.go
│       │   ├── validate.go
│       │   └── respond.go
│       │
│       └── model/
│           ├── tick.go
│           └── candle.go
│
└── scripts/
    ├── go.mod
    ├── go.sum
    └── seed_jalali_calendar.go
```



### Module separation

The application and seed script are separate Go modules.

`app/go.mod` contains only:

```text
github.com/ClickHouse/clickhouse-go/v2
```

as its third-party application dependency.

`scripts/go.mod` independently contains:

- the selected maintained Jalali conversion library
- `github.com/ClickHouse/clickhouse-go/v2` because the seed script needs to write to ClickHouse

The Jalali conversion dependency must **not** be part of `app/go.mod`.

There must be no `internal/jalali` package.

Jalali conversion logic is confined to:

- `scripts/seed_jalali_calendar.go`
- precomputed ClickHouse calendar data

The HTTP application must not perform Jalali date arithmetic.

### First commit

Create the skeleton first, with package declarations and module files, before implementing business logic.

Commit this scaffold as the first commit.

---



## Phase 2 — Docker setup



### 2.1 `clickhouse` service

Use the pinned ClickHouse image:

```text
clickhouse/clickhouse-server:26.3.25.2-jammy
```

Do **not** use `latest`.

Mount:

```text
./clickhouse/init:/docker-entrypoint-initdb.d/
```

Mount a persistent volume for:

```text
/var/lib/clickhouse
```

Expose:

```text
8123
9000
```

The application uses the native protocol through `clickhouse-go/v2`.

Healthcheck:

```text
wget --spider http://localhost:8123/ping
```



### 2.2 `app` service

Build the application from the repository root so that both `app/` and `scripts/` are available inside the image.

The Dockerfile may build the application binary from `app/`, but the image must also contain the `scripts/` directory because the seed command is executed through the same image.

Do not assume that:

```text
build: ./app
```

can access repository-level `scripts/`.

The final Docker configuration must make this command valid:

```bash
docker compose run --rm app go run ../scripts/seed_jalali_calendar.go
```

or an equivalent command that correctly references the copied scripts directory.

The exact command/path must be documented in `README.md`.

Environment variables:

```text
CH_HOST
CH_PORT
CH_USER
CH_PASSWORD
CH_DATABASE
API_KEY
APP_PORT
```

The `API_KEY` is required.

If `API_KEY` is missing or empty, the application must fail fast during startup.

Example `.env.example`:

```env
CH_HOST=clickhouse
CH_PORT=9000
CH_USER=default
CH_PASSWORD=
CH_DATABASE=default

API_KEY=change-me

APP_PORT=8080
```

The real `.env` must not be committed.

### 2.3 Startup order

`app` depends on:

```text
clickhouse
```

with:

```text
condition: service_healthy
```

No third running container is allowed.

---



## Phase 3 — ClickHouse schema

All SQL goes into `clickhouse/init/` in this exact order.

There are:

- 1 raw tick table
- 8 candle tables
- 1 symbols table
- 1 Jalali calendar table
- 1 Jalali dictionary

Therefore:

```text
10 application tables
+ 1 global calendar table
+ 1 dictionary
```

The 10 application tables are:

```text
ticks
candles_1d_greg
candles_1w_greg
candles_1m_greg
candles_1y_greg
candles_1d_jalali
candles_1w_jalali
candles_1m_jalali
candles_1y_jalali
symbols
```

`jalali_calendar` is global reference data and is not included in the 10 symbol-related tables.

---



### 3.1 — `001_ticks.sql`

```sql
CREATE TABLE IF NOT EXISTS ticks (
    symbol   String,
    calendar Enum8('gregorian' = 1, 'jalali' = 2),
    value    Decimal(18,8),
    volume   Decimal(18,8),
    ts       DateTime64(3, 'UTC')
) ENGINE = MergeTree
PARTITION BY symbol
ORDER BY (symbol, ts)
TTL toDateTime(ts) + INTERVAL 7 DAY;
```

Notes:

- `calendar` completely replaces the old `market` field.
- `Enum8` provides compact storage and rejects invalid values.
- `PARTITION BY symbol` is deliberate because symbol deletion must use `DROP PARTITION`.
- `ORDER BY (symbol, ts)` is optimized for symbol-first time-series queries.
- TTL applies only to raw ticks.

No application code may write directly into candle tables.

All ticks must enter through:

```text
ticks
```

so all materialized views are triggered.

---



### 3.2 — `002_jalali_calendar.sql`

```sql
CREATE TABLE IF NOT EXISTS jalali_calendar (
    gregorian_date          Date,
    jalali_year             UInt16,
    jalali_month            UInt8,
    jalali_day              UInt8,
    jalali_week_start_date  Date,
    jalali_month_start_date Date,
    jalali_year_start_date  Date
) ENGINE = MergeTree
ORDER BY gregorian_date;

CREATE DICTIONARY IF NOT EXISTS jalali_calendar_dict
(
    gregorian_date          Date,
    jalali_year             UInt16,
    jalali_month            UInt8,
    jalali_day              UInt8,
    jalali_week_start_date  Date,
    jalali_month_start_date Date,
    jalali_year_start_date  Date
)
PRIMARY KEY gregorian_date
SOURCE(CLICKHOUSE(TABLE 'jalali_calendar'))
LIFETIME(0)
LAYOUT(FLAT());
```

The table starts empty.

The seed script populates it.

`LIFETIME(0)` means the dictionary does not automatically refresh.

After changes to the underlying table:

```sql
SYSTEM RELOAD DICTIONARY jalali_calendar_dict;
```

must be executed.

This is a setup concern, not a runtime operation.

---



### 3.3 — Jalali calendar seed generator

File:

```text
scripts/seed_jalali_calendar.go
```

Use a currently maintained Go Jalali conversion library.

The agent must verify the current maintained option at implementation time rather than assuming a package name.

Generate Gregorian dates for:

```text
1990-01-01
through
2100-01-01
```

For each date calculate:

- Jalali year
- Jalali month
- Jalali day
- Jalali month start Gregorian date
- Jalali year start Gregorian date
- Jalali Saturday-start week Gregorian date

Saturday-start week:

```go
offset := (int(date.Weekday()) - int(time.Saturday) + 7) % 7
weekStart := date.AddDate(0, 0, -offset)
```

Verify against known reference dates.

At minimum verify:

- a known Saturday maps to itself
- known Jalali month boundaries
- known Jalali year boundary

Insert using ClickHouse Go driver batches of approximately 10k rows.

The operation must be idempotent.

Running the seed script twice must not create duplicate calendar rows.

A truncate-and-reload strategy is acceptable.

After successful insertion:

```sql
SYSTEM RELOAD DICTIONARY jalali_calendar_dict;
```

must be executed.

---



## 3.4 — `003_candles_gregorian.sql`

Create four `AggregatingMergeTree` candle tables and four materialized views.

Example:

```sql
CREATE TABLE IF NOT EXISTS candles_1d_greg (
    symbol        String,
    bucket_start  DateTime,
    open          AggregateFunction(argMin, Decimal(18,8), DateTime64(3,'UTC')),
    high          AggregateFunction(max, Decimal(18,8)),
    low           AggregateFunction(min, Decimal(18,8)),
    close         AggregateFunction(argMax, Decimal(18,8), DateTime64(3,'UTC')),
    volume        AggregateFunction(sum, Decimal(18,8)),
    ticks         AggregateFunction(count)
) ENGINE = AggregatingMergeTree
PARTITION BY symbol
ORDER BY (symbol, bucket_start);
```

Materialized view:

```sql
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_candles_1d_greg
TO candles_1d_greg
AS SELECT
    symbol,
    toStartOfDay(ts) AS bucket_start,
    argMinState(value, ts) AS open,
    maxState(value)        AS high,
    minState(value)        AS low,
    argMaxState(value, ts) AS close,
    sumState(volume)       AS volume,
    countState()           AS ticks
FROM ticks
WHERE calendar = 'gregorian'
GROUP BY symbol, toStartOfDay(ts);
```

Repeat for:

### `1w`

```sql
toStartOfWeek(ts, 1)
```

Monday start.

### `1m`

```sql
toStartOfMonth(ts)
```



### `1y`

```sql
toStartOfYear(ts)
```

Tables:

```text
candles_1d_greg
candles_1w_greg
candles_1m_greg
candles_1y_greg
```

MVs:

```text
mv_candles_1d_greg
mv_candles_1w_greg
mv_candles_1m_greg
mv_candles_1y_greg
```

All Gregorian MVs must contain:

```sql
WHERE calendar = 'gregorian'
```

---



## 3.5 — `004_candles_jalali.sql`

Create:

```text
candles_1d_jalali
candles_1w_jalali
candles_1m_jalali
candles_1y_jalali
```

with matching MVs.

Daily bucket:

```sql
toDate(toTimezone(ts, 'Asia/Tehran'))
```

Example:

```sql
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_candles_1d_jalali
TO candles_1d_jalali
AS SELECT
    symbol,
    toDate(toTimezone(ts, 'Asia/Tehran')) AS bucket_start,
    argMinState(value, ts) AS open,
    maxState(value)        AS high,
    minState(value)        AS low,
    argMaxState(value, ts) AS close,
    sumState(volume)       AS volume,
    countState()           AS ticks
FROM ticks
WHERE calendar = 'jalali'
GROUP BY symbol, toDate(toTimezone(ts, 'Asia/Tehran'));
```

For weekly:

```sql
dictGet(
    'jalali_calendar_dict',
    'jalali_week_start_date',
    toDate(toTimezone(ts, 'Asia/Tehran'))
)
```

For monthly:

```sql
dictGet(
    'jalali_calendar_dict',
    'jalali_month_start_date',
    toDate(toTimezone(ts, 'Asia/Tehran'))
)
```

For yearly:

```sql
dictGet(
    'jalali_calendar_dict',
    'jalali_year_start_date',
    toDate(toTimezone(ts, 'Asia/Tehran'))
)
```

All Jalali MVs must contain:

```sql
WHERE calendar = 'jalali'
```

The Jalali dictionary must be populated before any Jalali tick is ingested.

---



## 3.6 — `005_symbols.sql`

```sql
CREATE TABLE IF NOT EXISTS symbols (
    symbol     String,
    calendar   Enum8('gregorian' = 1, 'jalali' = 2),
    created_at DateTime DEFAULT now()
) ENGINE = ReplacingMergeTree(created_at)
ORDER BY symbol;
```

Purpose:

- cheap symbol → calendar lookup
- ingest-time calendar validation
- candle read routing

There is exactly one logical calendar per symbol.

### Registration semantics

The application is responsible for correctness.

The rule is:

> **First successful registration wins.**

If a symbol does not exist:

```text
symbol = ABC
calendar = gregorian
```

then `ABC` is permanently Gregorian.

A later:

```text
ABC + jalali
```

must be rejected.

A later:

```text
ABC + gregorian
```

is accepted without changing the symbol definition.

There is no update operation for calendar.

`ReplacingMergeTree` is defensive only; it must not be treated as a uniqueness constraint or transactional locking mechanism.

When reading the symbol calendar, use a query that guarantees the current logical row is selected, including `FINAL` where required by the implementation.

---



## 3.7 — `006_ttl.sql`

The raw `ticks` table has a mandatory 7-day TTL.

The TTL is safe because materialized views process inserted rows immediately.

The application must never write directly to:

```text
candles_*
```

tables.

All data ingestion must go through:

```text
ticks
```

Therefore an old tick can disappear from `ticks` while its candle aggregate remains.

README must include this monitoring query:

```sql
SELECT count()
FROM ticks
WHERE ts < now() - INTERVAL 6 DAY;
```

This is a manual early-warning check.

---



# Phase 4 — Go application



## 4.1 — Dependencies



### `app/go.mod`

Only third-party application dependency:

```text
github.com/ClickHouse/clickhouse-go/v2
```

Everything else is standard library.

### `scripts/go.mod`

The separate seed module may contain:

```text
github.com/ClickHouse/clickhouse-go/v2
<selected-maintained-Jalali-library>
```

The Jalali library must not be imported by any application package under:

```text
app/internal/
```

---



## 4.2 — Authentication

All three endpoints require:

```http
X-API-Key: <API_KEY>
```

The configured secret comes from:

```text
API_KEY
```

environment variable.

Authentication rules:

1. Missing header → `401`
2. Empty header → `401`
3. Incorrect key → `401`
4. Correct key → continue to endpoint validation

Use:

```go
crypto/subtle.ConstantTimeCompare
```

for the API key comparison.

Authentication must happen before request-body/query/path validation.

Do not expose the configured API key in logs.

Do not log incoming API keys.

All three endpoints require the same API key:

```text
POST   /api/v1/ticks
DELETE /api/v1/symbols/{symbol}
GET    /api/v1/candles
```

No endpoint is intentionally public.

If `API_KEY` is missing or empty at application startup, the application must fail fast and exit non-zero.

---



## 4.3 — Routing

Use Go 1.22+ `http.ServeMux`.

```go
mux := http.NewServeMux()

mux.HandleFunc("POST /api/v1/ticks", ticksHandler.Ingest)
mux.HandleFunc("DELETE /api/v1/symbols/{symbol}", symbolsHandler.Delete)
mux.HandleFunc("GET /api/v1/candles", candlesHandler.Read)
```

`go.mod` must specify Go 1.22 or newer.

No external router.

Authentication should wrap the mux or be applied consistently to all three handlers.

No additional API routes should be created.

---



## 4.4 — `POST /api/v1/ticks`



### Request

Body:

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

The `ticks` array is mandatory even when there is only one tick.

### Validation

Decode with:

```go
json.NewDecoder(r.Body).Decode(&req)
```

Then manually validate.

Requirements:

- `ticks` must not be empty.
- `symbol` must be non-empty.
- `calendar` must be exactly:
  - `gregorian`
  - `jalali`
- `value` must be a positive decimal.
- `volume` must be a positive decimal.
- `timestamp` must parse as RFC3339.
- request must not contain malformed JSON.
- request body should be bounded to a reasonable maximum size.

Validation errors return:

```text
400 Bad Request
```

with the shared JSON error structure.

---



### Entire-batch validation

The complete batch must be validated **before any write occurs**.

This is critical.

If any tick has a validation error:

```text
NO tick is inserted.
NO symbol is registered.
```

If any existing symbol has a calendar mismatch:

```text
NO tick is inserted.
NO symbol is registered.
```

If two ticks for a new symbol inside the same batch specify different calendars:

```text
NO tick is inserted.
NO symbol is registered.
```

Example invalid batch:

```text
ABC → gregorian
ABC → jalali
```

The entire batch is rejected.

This is a validation-atomicity requirement.

---



### Symbol/calendar lookup

After local request validation:

1. Extract the distinct symbols in the batch.
2. Query existing registrations in one query:

```sql
SELECT symbol, calendar
FROM symbols
WHERE symbol IN (...)
```

1. Compare every incoming tick against the existing registration.
2. Detect conflicting calendars among new symbols inside the batch.
3. If any conflict exists, reject the whole batch.

No database writes occur before these checks succeed.

---



### New symbol registration

For symbols not yet registered:

```text
first successful registration wins
```

Register:

```text
symbol
calendar
```

using the calendar declared by the validated batch.

If the same symbol already exists with the same calendar, registration is a no-op.

The application must never overwrite an existing symbol's calendar.

### Concurrent first registration

Two concurrent requests may attempt to register the same new symbol.

The implementation must guarantee that the symbol's calendar cannot be silently changed by a race.

The required semantic result is:

> The first successful registration determines the symbol's calendar. Any concurrent request using the opposite calendar must subsequently be rejected.

The implementation must therefore re-check the persisted symbol state after registration when necessary rather than assuming the initial `SELECT` alone is sufficient.

`ReplacingMergeTree` must not be treated as a transactional lock.

---



### Write ordering

The handler must follow this logical sequence:

```text
1. Authenticate
2. Decode request
3. Validate entire request
4. Query existing symbols
5. Validate all symbol/calendar relationships
6. Validate new-symbol conflicts inside the batch
7. Register new symbols
8. Insert all ticks using one native ClickHouse batch
```

No tick write may happen before all validation succeeds.

The agent must document the distinction:

- validation failures are atomic: no writes happen
- ClickHouse does not provide a conventional multi-statement application transaction spanning symbol registration + tick insertion

Therefore, if an unexpected infrastructure failure happens after symbol registration but before tick insertion, the application must not claim database-level transactionality.

The normal API contract is nevertheless:

> A rejected request due to validation causes zero writes.

---



### Tick insertion

Use:

```go
conn.PrepareBatch(...)
```

and native ClickHouse protocol.

Do not insert ticks row-by-row.

The complete accepted batch is inserted through the native batch API.

---



### Successful response

```http
201 Created
```

Example:

```json
{
  "accepted": 100
}
```

There is no partial-success response.

If the batch is rejected:

```http
400 Bad Request
```

and:

```json
{
  "error": {
    "code": "calendar_mismatch",
    "message": "symbol ABC is registered as gregorian but received jalali"
  }
}
```

No `accepted` count is returned for a rejected batch.

---



## 4.5 — `DELETE /api/v1/symbols/{symbol}`

This endpoint also requires:

```http
X-API-Key: <API_KEY>
```

Extract the symbol with:

```go
r.PathValue("symbol")
```



### Deletion scope

Deleting a symbol must remove it from all 10 application tables:

```text
ticks
candles_1d_greg
candles_1w_greg
candles_1m_greg
candles_1y_greg
candles_1d_jalali
candles_1w_jalali
candles_1m_jalali
candles_1y_jalali
symbols
```

The global:

```text
jalali_calendar
```

table is never affected.

### Tick/candle deletion

All 9 symbol-partitioned tables use:

```text
PARTITION BY symbol
```

Therefore use:

```sql
ALTER TABLE <table> DROP PARTITION '<symbol>';
```

for:

- `ticks`
- 8 candle tables

If the partition does not exist, treat it as a no-op.

### Symbols deletion

The `symbols` table is not partitioned by symbol.

Delete its row using a mutation:

```sql
DELETE FROM symbols WHERE symbol = ?;
```

The implementation should use synchronous mutation settings where necessary so the endpoint does not report success before the deletion has become visible.

Return:

```http
200 OK
```

with a per-table deletion summary.

Deleting a symbol that does not exist is a successful no-op.

---



## 4.6 — `GET /api/v1/candles`

Authentication required:

```http
X-API-Key: <API_KEY>
```

Required query parameters:

```text
symbol
from
to
timeframe
```

Allowed timeframe values:

```text
1d
1w
1m
1y
```

There is **no** `calendar` **query parameter**.

If the client sends `calendar`, reject the request with:

```text
400 Bad Request
```

This keeps the API contract strict and ensures that the `symbols` table remains the only source of truth.

### Handler flow

1. Authenticate.
2. Validate query parameters.
3. Look up:

```sql
SELECT calendar
FROM symbols FINAL
WHERE symbol = ?
```

1. If the symbol does not exist:

```text
404 Not Found
```

1. Determine the correct candle table.

Mapping:

```text
gregorian + 1d → candles_1d_greg
gregorian + 1w → candles_1w_greg
gregorian + 1m → candles_1m_greg
gregorian + 1y → candles_1y_greg

jalali + 1d → candles_1d_jalali
jalali + 1w → candles_1w_jalali
jalali + 1m → candles_1m_jalali
jalali + 1y → candles_1y_jalali
```

Do not construct arbitrary table names from unvalidated client input.

Use an explicit whitelist mapping.

---



### Candle query

Use:

```sql
SELECT
    bucket_start,
    argMinMerge(open)  AS open,
    maxMerge(high)     AS high,
    minMerge(low)      AS low,
    argMaxMerge(close) AS close,
    sumMerge(volume)   AS volume,
    countMerge(ticks)  AS tick_count
FROM candles_{timeframe}_{greg|jalali}
WHERE symbol = ?
  AND bucket_start >= ?
  AND bucket_start <= ?
GROUP BY bucket_start
ORDER BY bucket_start;
```

Use parameterized values.

Never interpolate client-provided values directly into SQL.

Only the internally selected table name may be inserted into the SQL string, and it must come from the explicit whitelist mapping.

For Jalali candles, use the dictionary to attach:

```text
jalali_year
jalali_month
jalali_day
```

labels where required by the response contract.

---



## 4.7 — JSON responses and errors

Use:

```go
json.NewEncoder(w).Encode(...)
```

for response encoding.

Create shared helpers in:

```text
internal/api/respond.go
```

for:

- JSON responses
- JSON errors
- HTTP status handling

Error responses must be consistent.

Recommended shape:

```json
{
  "error": {
    "code": "invalid_request",
    "message": "..."
  }
}
```

Do not expose:

- SQL statements
- API keys
- internal credentials
- stack traces
- ClickHouse connection details

in API responses.

---



# Phase 5 — Startup health checks

The application must fail fast if any required startup condition is not satisfied.

### 5.1 ClickHouse connectivity

Fail startup if the ClickHouse connection cannot be established.

### 5.2 API key

Fail startup if:

```text
API_KEY
```

is missing or empty.

### 5.3 Jalali calendar

Check:

```sql
SELECT count()
FROM jalali_calendar;
```

If zero:

- log a clear error
- instruct operator to run the seed script
- exit non-zero



### 5.4 Required tables

Verify all 10 application tables exist:

```text
ticks

candles_1d_greg
candles_1w_greg
candles_1m_greg
candles_1y_greg

candles_1d_jalali
candles_1w_jalali
candles_1m_jalali
candles_1y_jalali

symbols
```

Also verify:

```text
jalali_calendar
```

and:

```text
jalali_calendar_dict
```

are available.

The application must not begin accepting requests until all required dependencies are ready.

---



# Phase 6 — README

`README.md` must document:

1. Project purpose.
2. Architecture.
3. Two-container setup.
4. Environment variables.
5. API Key authentication.
6. `X-API-Key` header.
7. All 3 endpoints.
8. Request/response examples.
9. Calendar semantics.
10. Gregorian vs Jalali candle behavior.
11. Symbol immutable-calendar rule.
12. Whole-batch rejection behavior.
13. Docker commands.
14. Jalali seed procedure.
15. Jalali dictionary reload.
16. 7-day raw tick TTL.
17. Safe-TTL reasoning.
18. Manual TTL monitoring query.
19. Delete-symbol behavior.
20. No Swagger/OpenAPI.
21. Expected ClickHouse version.
22. Development/build commands.

The README must never contradict this execution plan.

---



# Phase 7 — Verification checklist

The Cursor agent must verify every item before declaring the implementation complete.

## Infrastructure

- [ ] `docker compose up` starts both containers.
- [ ] ClickHouse becomes healthy.
- [ ] App starts only after ClickHouse is healthy.
- [ ] ClickHouse version is exactly `26.3.25.2`.
- [ ] No third running container exists.
- [ ] All 10 application tables exist.
- [ ] `jalali_calendar` exists.
- [ ] `jalali_calendar_dict` exists.



## Configuration

- [ ] `.env.example` contains `API_KEY`.
- [ ] App fails startup when `API_KEY` is missing.
- [ ] API key is never logged.
- [ ] API key comparison uses constant-time comparison.



## Jalali calendar

- [ ] Seed script runs successfully.
- [ ] `jalali_calendar` contains the 1990–2100 range.
- [ ] Seed script is idempotent.
- [ ] Dictionary reload succeeds.
- [ ] Known Jalali reference dates are verified.
- [ ] Saturday-start week calculation is verified against at least 3 known weeks.



## Authentication

- [ ] Missing `X-API-Key` returns `401`.
- [ ] Incorrect `X-API-Key` returns `401`.
- [ ] Correct `X-API-Key` allows the request.
- [ ] Authentication applies to all 3 endpoints.



## Ingest

- [ ] Single-element tick array succeeds.
- [ ] Large batch uses native ClickHouse batch insertion.
- [ ] Empty `ticks` array returns `400`.
- [ ] Invalid calendar returns `400`.
- [ ] Invalid value returns `400`.
- [ ] Invalid volume returns `400`.
- [ ] Invalid timestamp returns `400`.
- [ ] New symbol is registered automatically.
- [ ] Symbol calendar is persisted.
- [ ] Existing symbol with same calendar is accepted.
- [ ] Existing symbol with different calendar returns `400`.
- [ ] Calendar mismatch rejects the **entire batch**.
- [ ] A batch containing two new ticks for the same symbol with different calendars is rejected entirely.
- [ ] No symbol from a rejected validation batch is registered.
- [ ] No tick from a rejected validation batch is inserted.
- [ ] Concurrent first-registration attempts cannot silently change an existing symbol's calendar.



## Gregorian candles

- [ ] Gregorian `1d` starts at UTC midnight.
- [ ] Gregorian `1w` starts Monday.
- [ ] Gregorian `1m` starts at Gregorian month start.
- [ ] Gregorian `1y` starts at Gregorian year start.



## Jalali candles

- [ ] Jalali `1d` uses Tehran midnight.
- [ ] Jalali `1w` starts Saturday.
- [ ] Jalali `1m` uses Jalali month boundaries.
- [ ] Jalali `1y` uses Jalali year boundaries.
- [ ] Tehran timezone conversion is used before Jalali calendar lookup.
- [ ] No runtime Jalali arithmetic exists in HTTP request handling.



## Live aggregation

- [ ] A newly inserted tick immediately affects the current open candle.
- [ ] Subsequent `GET /api/v1/candles` reflects the new tick without requiring a scheduled refresh.
- [ ] All 8 materialized views receive the appropriate calendar-specific ticks.



## Candle reads

- [ ] Unknown symbol returns `404`.
- [ ] `symbol`, `from`, `to`, and `timeframe` are required.
- [ ] Invalid timeframe returns `400`.
- [ ] Invalid date range returns `400`.
- [ ] `calendar` is not accepted as a client parameter.
- [ ] Sending `calendar` returns `400`.
- [ ] Calendar is derived only from `symbols`.
- [ ] Gregorian symbols route only to Gregorian candle tables.
- [ ] Jalali symbols route only to Jalali candle tables.
- [ ] Client input cannot control arbitrary table names.
- [ ] Candle aggregate values are returned using the appropriate `-Merge` functions.



## Delete

- [ ] `DELETE /api/v1/symbols/{symbol}` requires API Key.
- [ ] Deletion removes the symbol from `ticks`.
- [ ] Deletion removes the symbol from all 8 candle tables.
- [ ] Deletion removes the symbol from `symbols`.
- [ ] `jalali_calendar` is never modified.
- [ ] Deleting an unknown symbol is a successful no-op.
- [ ] All 10 application tables are checked before/after deletion.



## TTL

- [ ] Ticks older than 7 days eventually disappear from `ticks`.
- [ ] Corresponding candle aggregates remain intact.
- [ ] `symbols` rows are unaffected by TTL.
- [ ] No application code writes directly to candle tables.
- [ ] README contains the 6-day TTL monitoring query.



## API contract

- [ ] Exactly 3 application endpoints exist.
- [ ] No `/docs`.
- [ ] No `/openapi.json`.
- [ ] No Swagger/OpenAPI dependency.
- [ ] No router framework.
- [ ] HTTP implementation uses standard-library `net/http`.
- [ ] JSON implementation uses standard-library `encoding/json`.



## Dependency isolation

- [ ] `app/go.mod` contains only `clickhouse-go/v2` as a third-party dependency.
- [ ] Jalali library exists only in `scripts/go.mod`.
- [ ] Jalali library is not imported by `app/internal`.
- [ ] Seed script remains independently runnable.

---



# Final implementation rules for Cursor

Before writing code, read this entire document.

Do not:

- introduce frameworks
- introduce additional services
- introduce Swagger/OpenAPI
- add undocumented endpoints
- add a `market` field
- add a client-side `calendar` parameter to candle reads
- allow symbols to switch calendars
- allow partial success for a rejected ingest batch
- write directly to candle tables
- introduce scheduled aggregation workers
- add a runtime Jalali calculation package
- silently resolve contradictions by changing the API contract

If an implementation detail is not specified but does not affect the external behavior or architecture, choose the simplest idiomatic Go/ClickHouse implementation.

If a decision would change the API contract, data model, consistency semantics, container architecture, or dependency policy, stop and document the issue rather than silently changing this plan.

The expected final system is:

```text
                    ┌──────────────────────┐
                    │       Client         │
                    └──────────┬───────────┘
                               │
                         X-API-Key
                               │
                               ▼
                    ┌──────────────────────┐
                    │    Go HTTP App       │
                    │      net/http        │
                    └──────────┬───────────┘
                               │
                 ┌─────────────┴─────────────┐
                 │                           │
                 ▼                           ▼
          POST /ticks                 GET /candles
                 │                           │
                 ▼                           │
             symbols ◄──────────────────────┤
                 │                           │
                 ▼                           ▼
               ticks                  candle tables
                 │                 ┌─────────┴─────────┐
                 │                 │                   │
                 ▼                 ▼                   ▼
          Gregorian MVs       Gregorian            Jalali
                 │              candles             candles
                 │
                 └───────────────────────────────────┐
                                                     │
                                      Jalali MVs use
                                      calendar dictionary
                                                     │
                                                     ▼
                                            jalali_calendar
```

The only persistent runtime services are:

```text
ClickHouse
Go application
```

and the only public API surface is:

```text
POST   /api/v1/ticks
DELETE /api/v1/symbols/{symbol}
GET    /api/v1/candles
```

