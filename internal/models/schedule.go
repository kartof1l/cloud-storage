package models

import "time"

type ScheduleEvent struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	EventDate   string    `json:"event_date"`
	EventTime   string    `json:"event_time"`
	EventType   string    `json:"event_type"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
