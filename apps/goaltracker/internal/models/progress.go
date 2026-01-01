package models

import (
	"time"
)

const ProgressDateFormat = "2006-01-02"

type Progress struct {
	TypeID int64             `json:"typeId"`
	Date   time.Time         `json:"date"`
	Value  string            `json:"value"`
	Config map[string]string `json:"config"`
}
