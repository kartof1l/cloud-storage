package models

import "time"

// internal/models/schedule.go
type ScheduleTask struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	DueDate     string    `json:"due_date"`
	DueTime     string    `json:"due_time"`
	Priority    string    `json:"priority"`
	Completed   bool      `json:"completed"`
	Region      string    `json:"region"`   // Новое поле: "Krasnoyarsk", "Lesosibirsk"
	Timezone    string    `json:"timezone"` // "Asia/Krasnoyarsk", "Asia/Krasnoyarsk"
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
