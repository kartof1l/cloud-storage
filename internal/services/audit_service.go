package services

import (
    "time"

    "github.com/google/uuid"

    "cloud-storage-go/internal/repository"
)

type AuditService struct {
    auditRepo *repository.AuditRepository
}

func NewAuditService(auditRepo *repository.AuditRepository) *AuditService {
    return &AuditService{auditRepo: auditRepo}
}

func (s *AuditService) Log(userID uuid.UUID, userEmail, action, entityType string, entityID *uuid.UUID, entityName, details, ip, userAgent string) error {
    log := &repository.AuditLog{
        ID:         uuid.New(),
        UserID:     userID,
        UserEmail:  userEmail,
        Action:     action,
        EntityType: entityType,
        EntityID:   entityID,
        EntityName: entityName,
        Details:    details,
        IPAddress:  ip,
        UserAgent:  userAgent,
        CreatedAt:  time.Now(),
    }
    return s.auditRepo.Create(log)
}

func (s *AuditService) GetLogs(page, limit int) ([]repository.AuditLog, int64, error) {
    offset := (page - 1) * limit
    return s.auditRepo.GetLogs(limit, offset)
}
