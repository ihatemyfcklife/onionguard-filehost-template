package model

import (
	"time"
)

// FileMetadata stores technical and administrative metadata for an uploaded file.
type FileMetadata struct {
	ID            string    `json:"id"`
	OriginalName  string    `json:"original_name"`
	ContentType   string    `json:"content_type"`
	Size          int64     `json:"size"`
	SHA256        string    `json:"sha256"`
	UploadedAt    time.Time `json:"uploaded_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	BurnAfterRead bool      `json:"burn_after_read"`
	DeleteToken   string    `json:"delete_token"`
	SessionID     string    `json:"session_id,omitempty"`
	DownloadCount int       `json:"download_count"`
}

// IsExpired checks whether the file lifetime has exceeded the current time.
func (f *FileMetadata) IsExpired(now time.Time) bool {
	if f.ExpiresAt.IsZero() {
		return false
	}
	return now.After(f.ExpiresAt)
}
