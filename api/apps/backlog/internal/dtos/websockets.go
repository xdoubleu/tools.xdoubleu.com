package dtos

import (
	"time"
)

type SubscribeMessageDto struct {
	Subject string `json:"subject"`
}

type StateMessageDto struct {
	LastRefresh  *time.Time `json:"lastRefresh"`
	IsRefreshing bool       `json:"isRefreshing"`
	// Processed and Total are set during long-running jobs (e.g. Open Library
	// resync) to give clients a live "X of N" count. They are omitted for jobs
	// that emit only start/stop events (e.g. Steam refresh).
	Processed *int `json:"processed,omitempty"`
	Total     *int `json:"total,omitempty"`
}

func (dto SubscribeMessageDto) Topic() string {
	return dto.Subject
}

func (dto SubscribeMessageDto) Validate() (bool, map[string]string) {
	return true, make(map[string]string)
}
