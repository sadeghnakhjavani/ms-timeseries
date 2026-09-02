package model

type Candle struct {
	BucketStart string  `json:"bucket_start"`
	Open        string  `json:"open"`
	High        string  `json:"high"`
	Low         string  `json:"low"`
	Close       string  `json:"close"`
	Volume      string  `json:"volume"`
	TickCount   uint64  `json:"tick_count"`
	JalaliYear  *uint16 `json:"jalali_year,omitempty"`
	JalaliMonth *uint8  `json:"jalali_month,omitempty"`
	JalaliDay   *uint8  `json:"jalali_day,omitempty"`
}

type CandlesResponse struct {
	Symbol   string   `json:"symbol"`
	Calendar Calendar `json:"calendar"`
	Candles  []Candle `json:"candles"`
}

type DeleteSummary struct {
	Symbol  string            `json:"symbol"`
	Deleted map[string]string `json:"deleted"`
}
