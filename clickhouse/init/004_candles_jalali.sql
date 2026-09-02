CREATE TABLE IF NOT EXISTS candles_1d_jalali (
    symbol        String,
    bucket_start  Date32,
    open          AggregateFunction(argMin, Decimal(18,8), DateTime64(3,'UTC')),
    high          AggregateFunction(max, Decimal(18,8)),
    low           AggregateFunction(min, Decimal(18,8)),
    close         AggregateFunction(argMax, Decimal(18,8), DateTime64(3,'UTC')),
    volume        AggregateFunction(sum, Decimal(18,8)),
    ticks         AggregateFunction(count)
) ENGINE = AggregatingMergeTree
PARTITION BY symbol
ORDER BY (symbol, bucket_start);

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_candles_1d_jalali
TO candles_1d_jalali
AS SELECT
    symbol,
    toDate32(toTimezone(ts, 'Asia/Tehran')) AS bucket_start,
    argMinState(value, ts) AS open,
    maxState(value)        AS high,
    minState(value)        AS low,
    argMaxState(value, ts) AS close,
    sumState(volume)       AS volume,
    countState()           AS ticks
FROM ticks
WHERE calendar = 'jalali'
GROUP BY symbol, toDate32(toTimezone(ts, 'Asia/Tehran'));

CREATE TABLE IF NOT EXISTS candles_1w_jalali (
    symbol        String,
    bucket_start  Date32,
    open          AggregateFunction(argMin, Decimal(18,8), DateTime64(3,'UTC')),
    high          AggregateFunction(max, Decimal(18,8)),
    low           AggregateFunction(min, Decimal(18,8)),
    close         AggregateFunction(argMax, Decimal(18,8), DateTime64(3,'UTC')),
    volume        AggregateFunction(sum, Decimal(18,8)),
    ticks         AggregateFunction(count)
) ENGINE = AggregatingMergeTree
PARTITION BY symbol
ORDER BY (symbol, bucket_start);

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_candles_1w_jalali
TO candles_1w_jalali
AS SELECT
    symbol,
    dictGet(
        'jalali_calendar_dict',
        'jalali_week_start_date',
        toDate32(toTimezone(ts, 'Asia/Tehran'))
    ) AS bucket_start,
    argMinState(value, ts) AS open,
    maxState(value)        AS high,
    minState(value)        AS low,
    argMaxState(value, ts) AS close,
    sumState(volume)       AS volume,
    countState()           AS ticks
FROM ticks
WHERE calendar = 'jalali'
GROUP BY symbol, dictGet(
    'jalali_calendar_dict',
    'jalali_week_start_date',
    toDate32(toTimezone(ts, 'Asia/Tehran'))
);

CREATE TABLE IF NOT EXISTS candles_1m_jalali (
    symbol        String,
    bucket_start  Date32,
    open          AggregateFunction(argMin, Decimal(18,8), DateTime64(3,'UTC')),
    high          AggregateFunction(max, Decimal(18,8)),
    low           AggregateFunction(min, Decimal(18,8)),
    close         AggregateFunction(argMax, Decimal(18,8), DateTime64(3,'UTC')),
    volume        AggregateFunction(sum, Decimal(18,8)),
    ticks         AggregateFunction(count)
) ENGINE = AggregatingMergeTree
PARTITION BY symbol
ORDER BY (symbol, bucket_start);

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_candles_1m_jalali
TO candles_1m_jalali
AS SELECT
    symbol,
    dictGet(
        'jalali_calendar_dict',
        'jalali_month_start_date',
        toDate32(toTimezone(ts, 'Asia/Tehran'))
    ) AS bucket_start,
    argMinState(value, ts) AS open,
    maxState(value)        AS high,
    minState(value)        AS low,
    argMaxState(value, ts) AS close,
    sumState(volume)       AS volume,
    countState()           AS ticks
FROM ticks
WHERE calendar = 'jalali'
GROUP BY symbol, dictGet(
    'jalali_calendar_dict',
    'jalali_month_start_date',
    toDate32(toTimezone(ts, 'Asia/Tehran'))
);

CREATE TABLE IF NOT EXISTS candles_1y_jalali (
    symbol        String,
    bucket_start  Date32,
    open          AggregateFunction(argMin, Decimal(18,8), DateTime64(3,'UTC')),
    high          AggregateFunction(max, Decimal(18,8)),
    low           AggregateFunction(min, Decimal(18,8)),
    close         AggregateFunction(argMax, Decimal(18,8), DateTime64(3,'UTC')),
    volume        AggregateFunction(sum, Decimal(18,8)),
    ticks         AggregateFunction(count)
) ENGINE = AggregatingMergeTree
PARTITION BY symbol
ORDER BY (symbol, bucket_start);

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_candles_1y_jalali
TO candles_1y_jalali
AS SELECT
    symbol,
    dictGet(
        'jalali_calendar_dict',
        'jalali_year_start_date',
        toDate32(toTimezone(ts, 'Asia/Tehran'))
    ) AS bucket_start,
    argMinState(value, ts) AS open,
    maxState(value)        AS high,
    minState(value)        AS low,
    argMaxState(value, ts) AS close,
    sumState(volume)       AS volume,
    countState()           AS ticks
FROM ticks
WHERE calendar = 'jalali'
GROUP BY symbol, dictGet(
    'jalali_calendar_dict',
    'jalali_year_start_date',
    toDate32(toTimezone(ts, 'Asia/Tehran'))
);
