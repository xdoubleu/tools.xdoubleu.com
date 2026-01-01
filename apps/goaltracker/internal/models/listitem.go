package models

import "time"

type ListItem struct {
	ID            int64     `json:"id"`
	Value         string    `json:"value"`
	CompletedDate time.Time `json:"completedDate"`
}
