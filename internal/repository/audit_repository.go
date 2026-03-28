package repository

import (
    "database/sql"
    "time"

    "github.com/google/uuid"
)

type AuditLog struct {
    ID         uuid.UUID `json:"id"`
    UserID     uuid.UUID `json:"user_id"`
    UserEmail  string    `json:"user_email"`
    Action     string    `json:"action"`
    EntityType string    `json:"entity_type"`
    EntityID   *uuid.UUID `json:"entity_id,omitempty"`
    EntityName string    `json:"entity_name,omitempty"`
    Details    string    `json:"details,omitempty"`
    IPAddress  string    `json:"ip_address"`
    UserAgent  string    `json:"user_agent"`
    CreatedAt  time.Time `json:"created_at"`
}

type AuditRepository struct {
    db *sql.DB
}

func NewAuditRepository(db *sql.DB) *AuditRepository {
    return &AuditRepository{db: db}
}

func (r *AuditRepository) Create(log *AuditLog) error {
    query := `
        INSERT INTO audit_logs (id, user_id, user_email, action, entity_type, entity_id, entity_name, details, ip_address, user_agent, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
    `
    _, err := r.db.Exec(query,
        log.ID, log.UserID, log.UserEmail, log.Action,
        log.EntityType, log.EntityID, log.EntityName, log.Details,
        log.IPAddress, log.UserAgent, log.CreatedAt,
    )
    return err
}

func (r *AuditRepository) GetLogs(limit, offset int) ([]AuditLog, int64, error) {
    var total int64
    r.db.QueryRow("SELECT COUNT(*) FROM audit_logs").Scan(&total)

    rows, err := r.db.Query(`
        SELECT id, user_id, user_email, action, entity_type, entity_id, entity_name, details, ip_address, user_agent, created_at
        FROM audit_logs
        ORDER BY created_at DESC
        LIMIT $1 OFFSET $2
    `, limit, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    var logs []AuditLog
    for rows.Next() {
        var l AuditLog
        err := rows.Scan(&l.ID, &l.UserID, &l.UserEmail, &l.Action, &l.EntityType,
            &l.EntityID, &l.EntityName, &l.Details, &l.IPAddress, &l.UserAgent, &l.CreatedAt)
        if err != nil {
            return nil, 0, err
        }
        logs = append(logs, l)
    }
    return logs, total, nil
}
