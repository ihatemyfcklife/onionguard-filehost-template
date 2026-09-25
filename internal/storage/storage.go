package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"onionguard-filehost/internal/model"
)

var (
	ErrFileNotFound = errors.New("file not found or has expired")
	ErrInvalidToken = errors.New("invalid deletion token")
)

// Storage handles local disk storage for files and persists metadata safely.
type Storage struct {
	mu           sync.RWMutex
	uploadDir    string
	metaFilePath string
	files        map[string]*model.FileMetadata
}

// NewStorage initializes storage directories, loads existing metadata, and starts a janitor loop.
func NewStorage(uploadDir, metaFilePath string) (*Storage, error) {
	if err := os.MkdirAll(uploadDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create upload directory %s: %w", uploadDir, err)
	}
	metaDir := filepath.Dir(metaFilePath)
	if err := os.MkdirAll(metaDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create meta directory %s: %w", metaDir, err)
	}

	s := &Storage{
		uploadDir:    uploadDir,
		metaFilePath: metaFilePath,
		files:        make(map[string]*model.FileMetadata),
	}

	if err := s.loadMeta(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("warning: unable to load existing metadata from %s: %v", metaFilePath, err)
	}

	// Initial purge of expired files
	_, _ = s.CleanupExpired()
	return s, nil
}

// SaveFile reads data from reader, calculates its SHA-256 hash, stores it to disk, and tracks metadata.
func (s *Storage) SaveFile(r io.Reader, originalName, contentType string, ttl time.Duration, burnAfterRead bool, sessionID string) (*model.FileMetadata, error) {
	id := generateRandomHex(8) // 16 hex chars
	deleteToken := generateRandomHex(16) // 32 hex chars

	filePath := filepath.Join(s.uploadDir, id)
	dst, err := os.OpenFile(filePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	hasher := sha256.New()
	multiWriter := io.MultiWriter(dst, hasher)

	written, err := io.Copy(multiWriter, r)
	if err != nil {
		_ = os.Remove(filePath)
		return nil, fmt.Errorf("failed to write file content: %w", err)
	}

	hashStr := hex.EncodeToString(hasher.Sum(nil))

	now := time.Now()
	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = now.Add(ttl)
	}

	if originalName == "" {
		originalName = "upload.bin"
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	meta := &model.FileMetadata{
		ID:            id,
		OriginalName:  filepath.Base(originalName),
		ContentType:   contentType,
		Size:          written,
		SHA256:        hashStr,
		UploadedAt:    now,
		ExpiresAt:     expiresAt,
		BurnAfterRead: burnAfterRead,
		DeleteToken:   deleteToken,
		SessionID:     sessionID,
		DownloadCount: 0,
	}

	s.mu.Lock()
	s.files[id] = meta
	err = s.persistMetaLocked()
	s.mu.Unlock()

	if err != nil {
		_ = os.Remove(filePath)
		s.mu.Lock()
		delete(s.files, id)
		s.mu.Unlock()
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	return meta, nil
}

// GetFile retrieves metadata for a file, deleting it if it is expired.
func (s *Storage) GetFile(id string) (*model.FileMetadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, ok := s.files[id]
	if !ok {
		return nil, ErrFileNotFound
	}

	if meta.IsExpired(time.Now()) {
		s.deleteFileLocked(id)
		_ = s.persistMetaLocked()
		return nil, ErrFileNotFound
	}

	return meta, nil
}

// OpenFileReader opens the raw file payload on disk after verifying metadata.
func (s *Storage) OpenFileReader(id string) (io.ReadCloser, *model.FileMetadata, error) {
	meta, err := s.GetFile(id)
	if err != nil {
		return nil, nil, err
	}

	filePath := filepath.Join(s.uploadDir, id)
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.mu.Lock()
			s.deleteFileLocked(id)
			_ = s.persistMetaLocked()
			s.mu.Unlock()
			return nil, nil, ErrFileNotFound
		}
		return nil, nil, fmt.Errorf("failed to open file payload: %w", err)
	}

	return file, meta, nil
}

// RecordDownload increments the download counter and purges the file if BurnAfterRead is enabled.
func (s *Storage) RecordDownload(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, ok := s.files[id]
	if !ok {
		return ErrFileNotFound
	}

	meta.DownloadCount++
	if meta.BurnAfterRead {
		s.deleteFileLocked(id)
	}

	return s.persistMetaLocked()
}

// DeleteFile removes a file immediately using its secret deletion token.
func (s *Storage) DeleteFile(id, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, ok := s.files[id]
	if !ok {
		return ErrFileNotFound
	}

	if subtle.ConstantTimeCompare([]byte(meta.DeleteToken), []byte(token)) != 1 {
		return ErrInvalidToken
	}

	s.deleteFileLocked(id)
	return s.persistMetaLocked()
}

// ListBySession returns all active, non-expired files belonging to a specific OnionGuard session.
func (s *Storage) ListBySession(sessionID string) []*model.FileMetadata {
	if sessionID == "" {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	var list []*model.FileMetadata
	for _, meta := range s.files {
		if meta.SessionID == sessionID && !meta.IsExpired(now) {
			list = append(list, meta)
		}
	}
	return list
}

// CleanupExpired deletes all expired files from disk and metadata index.
func (s *Storage) CleanupExpired() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	cleaned := 0
	for id, meta := range s.files {
		if meta.IsExpired(now) {
			s.deleteFileLocked(id)
			cleaned++
		}
	}

	if cleaned > 0 {
		return cleaned, s.persistMetaLocked()
	}
	return 0, nil
}

// StartJanitor launches a background goroutine that periodically sweeps expired files.
func (s *Storage) StartJanitor(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if n, err := s.CleanupExpired(); err == nil && n > 0 {
					log.Printf("[janitor] purged %d expired files", n)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *Storage) deleteFileLocked(id string) {
	filePath := filepath.Join(s.uploadDir, id)
	_ = os.Remove(filePath)
	delete(s.files, id)
}

func (s *Storage) loadMeta() error {
	data, err := os.ReadFile(s.metaFilePath)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.Unmarshal(data, &s.files)
}

func (s *Storage) persistMetaLocked() error {
	data, err := json.MarshalIndent(s.files, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	tmpFile := s.metaFilePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write tmp metadata: %w", err)
	}

	return os.Rename(tmpFile, s.metaFilePath)
}

func generateRandomHex(byteLen int) string {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp + pseudo entropy if crypto/rand fails
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
