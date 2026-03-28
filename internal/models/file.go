package models

import (
    "time"

    "github.com/google/uuid"
)

type File struct {
    ID             uuid.UUID  `json:"id" db:"id"`
    UserID         uuid.UUID  `json:"user_id" db:"user_id"`
    Name           string     `json:"name" db:"name"`
    OriginalName   string     `json:"original_name" db:"original_name"`
    Path           string     `json:"path" db:"path"`
    Size           int64      `json:"size" db:"size"`
    FolderSize     int64      `json:"folder_size,omitempty" db:"folder_size"`
    MimeType       string     `json:"mime_type" db:"mime_type"`
    IsFolder       bool       `json:"is_folder" db:"is_folder"`
    ParentFolderID *uuid.UUID `json:"parent_folder_id,omitempty" db:"parent_folder_id"`
    CreatedAt      time.Time  `json:"created_at" db:"created_at"`
    UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

type FileUploadResponse struct {
    ID        uuid.UUID `json:"id"`
    Name      string    `json:"name"`
    Size      int64     `json:"size"`
    MimeType  string    `json:"mime_type"`
    CreatedAt time.Time `json:"created_at"`
}

type FolderCreateRequest struct {
    Name           string     `json:"name" binding:"required"`
    ParentFolderID *uuid.UUID `json:"parent_folder_id"`
}

type FileListResponse struct {
    Files []File `json:"files"`
    Total int64  `json:"total"`
    Page  int    `json:"page"`
    Limit int    `json:"limit"`
}
