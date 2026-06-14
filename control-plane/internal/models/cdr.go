package models

import (
	"encoding/json"
	"time"
)

// CDR is one parsed call detail record.
type CDR struct {
	ID                string          `json:"id"`
	Direction         string          `json:"direction"`
	CallerIDNumber    string          `json:"caller_id_number"`
	CallerIDName      string          `json:"caller_id_name"`
	DestinationNumber string          `json:"destination_number"`
	Context           string          `json:"context"`
	HangupCause       string          `json:"hangup_cause"`
	StartEpoch        int64           `json:"start_epoch"`
	AnswerEpoch       int64           `json:"answer_epoch"`
	EndEpoch          int64           `json:"end_epoch"`
	Duration          int             `json:"duration"`
	Billsec           int             `json:"billsec"`
	RecordingPath     string          `json:"recording_path"`
	Raw               json.RawMessage `json:"raw,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
}

// CDRFilter narrows a CDR query. Empty fields are ignored; epoch bounds 0 are
// ignored. Answered restricts to answered calls (billsec>0).
type CDRFilter struct {
	Number      string // matches caller OR destination
	HangupCause string
	FromEpoch   int64
	ToEpoch     int64
	AnsweredOnly bool
	Limit       int
	Offset      int
}

// CDRStats is a daily rollup row.
type CDRStats struct {
	Day        string `json:"day"`         // YYYY-MM-DD (UTC)
	Total      int    `json:"total"`
	Answered   int    `json:"answered"`
	Abandoned  int    `json:"abandoned"`   // not answered
	TalkTime   int64  `json:"talk_time"`   // sum(billsec)
	AvgBillsec int    `json:"avg_billsec"`
}
