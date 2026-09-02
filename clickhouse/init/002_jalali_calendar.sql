CREATE TABLE IF NOT EXISTS jalali_calendar (
    gregorian_date          Date32,
    jalali_year             UInt16,
    jalali_month            UInt8,
    jalali_day              UInt8,
    jalali_week_start_date  Date32,
    jalali_month_start_date Date32,
    jalali_year_start_date  Date32
) ENGINE = MergeTree
ORDER BY gregorian_date;

CREATE DICTIONARY IF NOT EXISTS jalali_calendar_dict
(
    gregorian_date          Date32,
    jalali_year             UInt16,
    jalali_month            UInt8,
    jalali_day              UInt8,
    jalali_week_start_date  Date32,
    jalali_month_start_date Date32,
    jalali_year_start_date  Date32
)
PRIMARY KEY gregorian_date
SOURCE(CLICKHOUSE(TABLE 'jalali_calendar'))
LIFETIME(0)
LAYOUT(HASHED());
