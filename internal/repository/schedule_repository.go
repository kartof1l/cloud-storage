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

func (r *ScheduleRepository) DB() *sql.DB {
	return r.db
}

// Task methods
func (r *ScheduleRepository) CreateTask(task *models.ScheduleTask) error {
	task.ID = uuid.New().String()
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()
	task.Completed = false

	query := `INSERT INTO schedule_tasks (id, title, description, due_date, due_time, priority, completed, created_by, created_at, updated_at)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
	_, err := r.db.Exec(query, task.ID, task.Title, task.Description, task.DueDate,
		task.DueTime, task.Priority, task.Completed, task.CreatedBy, task.CreatedAt, task.UpdatedAt)
	return err
}

func (r *ScheduleRepository) GetAllTasks() ([]models.ScheduleTask, error) {
	query := `SELECT id, title, description, due_date, due_time, priority, completed, created_by, created_at, updated_at 
              FROM schedule_tasks ORDER BY completed, due_date, due_time`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.ScheduleTask
	for rows.Next() {
		var t models.ScheduleTask
		err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.DueDate, &t.DueTime,
			&t.Priority, &t.Completed, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (r *ScheduleRepository) UpdateTask(task *models.ScheduleTask) error {
	task.UpdatedAt = time.Now()
	query := `UPDATE schedule_tasks SET title=$1, description=$2, due_date=$3, due_time=$4, 
              priority=$5, updated_at=$6 WHERE id=$7`
	_, err := r.db.Exec(query, task.Title, task.Description, task.DueDate, task.DueTime,
		task.Priority, task.UpdatedAt, task.ID)
	return err
}

func (r *ScheduleRepository) ToggleTaskComplete(id string, completed bool) error {
	query := `UPDATE schedule_tasks SET completed=$1, updated_at=$2 WHERE id=$3`
	_, err := r.db.Exec(query, completed, time.Now(), id)
	return err
}

func (r *ScheduleRepository) DeleteTask(id string) error {
	query := `DELETE FROM schedule_tasks WHERE id=$1`
	_, err := r.db.Exec(query, id)
	return err
}
