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

CREATE TABLE IF NOT EXISTS candles_1w_greg (
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

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_candles_1w_greg
TO candles_1w_greg
AS SELECT
    symbol,
    toStartOfWeek(ts, 1) AS bucket_start,
    argMinState(value, ts) AS open,
    maxState(value)        AS high,
    minState(value)        AS low,
    argMaxState(value, ts) AS close,
    sumState(volume)       AS volume,
    countState()           AS ticks
FROM ticks
WHERE calendar = 'gregorian'
GROUP BY symbol, toStartOfWeek(ts, 1);

CREATE TABLE IF NOT EXISTS candles_1m_greg (
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

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_candles_1m_greg
TO candles_1m_greg
AS SELECT
    symbol,
    toStartOfMonth(ts) AS bucket_start,
    argMinState(value, ts) AS open,
    maxState(value)        AS high,
    minState(value)        AS low,
    argMaxState(value, ts) AS close,
    sumState(volume)       AS volume,
    countState()           AS ticks
FROM ticks
WHERE calendar = 'gregorian'
GROUP BY symbol, toStartOfMonth(ts);

CREATE TABLE IF NOT EXISTS candles_1y_greg (
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

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_candles_1y_greg
TO candles_1y_greg
AS SELECT
    symbol,
    toStartOfYear(ts) AS bucket_start,
    argMinState(value, ts) AS open,
    maxState(value)        AS high,
    minState(value)        AS low,
    argMaxState(value, ts) AS close,
    sumState(volume)       AS volume,
    countState()           AS ticks
FROM ticks
WHERE calendar = 'gregorian'
GROUP BY symbol, toStartOfYear(ts);
