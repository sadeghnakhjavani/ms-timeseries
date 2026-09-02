CREATE TABLE IF NOT EXISTS symbols (
    symbol     String,
    calendar   Enum8('gregorian' = 1, 'jalali' = 2),
    created_at DateTime DEFAULT now()
) ENGINE = ReplacingMergeTree(created_at)
ORDER BY symbol;
