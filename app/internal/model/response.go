package model

type Metadata struct {
	Page    int  `json:"page"`
	Limit   int  `json:"limit"`
	HasMore bool `json:"has_more"`
	Count   int  `json:"count"`
}

type Envelope struct {
	Data     interface{} `json:"data"`
	Metadata *Metadata   `json:"metadata,omitempty"`
}
