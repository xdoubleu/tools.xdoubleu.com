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
}

func (dto SubscribeMessageDto) Topic() string {
	return dto.Subject
}

func (dto SubscribeMessageDto) Validate() (bool, map[string]string) {
	return true, make(map[string]string)
}
