CREATE TABLE IF NOT EXISTS ticks (
    symbol   String,
    calendar Enum8('gregorian' = 1, 'jalali' = 2),
    value    Decimal(18,8),
    volume   Decimal(18,8),
    ts       DateTime64(3, 'UTC')
) ENGINE = MergeTree
PARTITION BY symbol
ORDER BY (symbol, ts);
