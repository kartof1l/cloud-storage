package repository

import (
	"database/sql"
	"time"

	"github.com/google/uuid"

	"cloud-storage-go/internal/models"
)

type ScheduleRepository struct {
	db *sql.DB
}

func NewScheduleRepository(db *sql.DB) *ScheduleRepository {
	return &ScheduleRepository{db: db}
}

func (r *ScheduleRepository) Create(event *models.ScheduleEvent) error {
	event.ID = uuid.New().String()
	event.CreatedAt = time.Now()
	event.UpdatedAt = time.Now()

	query := `INSERT INTO schedule_events (id, title, description, event_date, event_time, event_type, created_by, created_at, updated_at)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := r.db.Exec(query, event.ID, event.Title, event.Description, event.EventDate,
		event.EventTime, event.EventType, event.CreatedBy, event.CreatedAt, event.UpdatedAt)
	return err
}

func (r *ScheduleRepository) GetAll() ([]models.ScheduleEvent, error) {
	query := `SELECT id, title, description, event_date, event_time, event_type, created_by, created_at, updated_at 
              FROM schedule_events ORDER BY event_date, event_time`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []models.ScheduleEvent
	for rows.Next() {
		var e models.ScheduleEvent
		err := rows.Scan(&e.ID, &e.Title, &e.Description, &e.EventDate, &e.EventTime,
			&e.EventType, &e.CreatedBy, &e.CreatedAt, &e.UpdatedAt)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func (r *ScheduleRepository) Update(event *models.ScheduleEvent) error {
	event.UpdatedAt = time.Now()
	query := `UPDATE schedule_events SET title=$1, description=$2, event_date=$3, event_time=$4, 
              event_type=$5, updated_at=$6 WHERE id=$7`
	_, err := r.db.Exec(query, event.Title, event.Description, event.EventDate, event.EventTime,
		event.EventType, event.UpdatedAt, event.ID)
	return err
}

func (r *ScheduleRepository) Delete(id string) error {
	query := `DELETE FROM schedule_events WHERE id=$1`
	_, err := r.db.Exec(query, id)
	return err
}

func (r *ScheduleRepository) GetByID(id string) (*models.ScheduleEvent, error) {
	query := `SELECT id, title, description, event_date, event_time, event_type, created_by, created_at, updated_at 
              FROM schedule_events WHERE id=$1`
	var e models.ScheduleEvent
	err := r.db.QueryRow(query, id).Scan(&e.ID, &e.Title, &e.Description, &e.EventDate,
		&e.EventTime, &e.EventType, &e.CreatedBy, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}
func (r *ScheduleRepository) DB() *sql.DB {
	return r.db
}
