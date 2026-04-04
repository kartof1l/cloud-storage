package models

import "time"

type ScheduleTask struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	DueDate     string    `json:"due_date"`
	DueTime     string    `json:"due_time"`
	Priority    string    `json:"priority"` // high, medium, low
	Completed   bool      `json:"completed"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
