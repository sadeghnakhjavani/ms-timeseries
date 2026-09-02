package model

import "time"

type Calendar string

const (
	CalendarGregorian Calendar = "gregorian"
	CalendarJalali    Calendar = "jalali"
)

func (c Calendar) Valid() bool {
	return c == CalendarGregorian || c == CalendarJalali
}

type TickInput struct {
	Symbol    string    `json:"symbol"`
	Value     string    `json:"value"`
	Volume    string    `json:"volume"`
	Calendar  Calendar  `json:"calendar"`
	Timestamp time.Time `json:"timestamp"`
}

type Tick struct {
	Symbol   string
	Calendar Calendar
	Value    string
	Volume   string
	TS       time.Time
}

type IngestRequest struct {
	Ticks []TickInput `json:"ticks"`
}

type IngestResponse struct {
	Accepted int `json:"accepted"`
}

type TickRecord struct {
	Value     string `json:"value"`
	Volume    string `json:"volume"`
	Timestamp string `json:"timestamp"`
}

type TicksResponse struct {
	Symbol   string       `json:"symbol"`
	Calendar Calendar     `json:"calendar"`
	Ticks    []TickRecord `json:"ticks"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error APIError `json:"error"`
}
