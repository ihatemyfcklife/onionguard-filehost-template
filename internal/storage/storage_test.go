package storage

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStorageLifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "og-filehost-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	uploadDir := filepath.Join(tempDir, "uploads")
	metaFile := filepath.Join(tempDir, "meta.json")

	store, err := NewStorage(uploadDir, metaFile)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}

	// 1. Save file
	content := []byte("Hello Tor & OnionGuard! Zero-JS anonymous file hosting.")
	meta, err := store.SaveFile(bytes.NewReader(content), "hello.txt", "text/plain", 1*time.Hour, false, "session-123")
	if err != nil {
		t.Fatalf("SaveFile failed: %v", err)
	}

	if meta.ID == "" || meta.DeleteToken == "" {
		t.Fatalf("Expected non-empty ID and DeleteToken, got id=%s, token=%s", meta.ID, meta.DeleteToken)
	}
	if meta.Size != int64(len(content)) {
		t.Fatalf("Expected size %d, got %d", len(content), meta.Size)
	}
	if meta.OriginalName != "hello.txt" {
		t.Fatalf("Expected name hello.txt, got %s", meta.OriginalName)
	}

	// 2. Read file
	rc, readMeta, err := store.OpenFileReader(meta.ID)
	if err != nil {
		t.Fatalf("OpenFileReader failed: %v", err)
	}
	defer rc.Close()

	readBytes, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if !bytes.Equal(readBytes, content) {
		t.Fatalf("Content mismatch. Expected %s, got %s", content, readBytes)
	}
	if readMeta.SHA256 != meta.SHA256 {
		t.Fatalf("Hash mismatch")
	}

	// 3. List by session
	sessionFiles := store.ListBySession("session-123")
	if len(sessionFiles) != 1 || sessionFiles[0].ID != meta.ID {
		t.Fatalf("Expected 1 file for session-123, got %d", len(sessionFiles))
	}

	// 4. Delete with invalid token
	err = store.DeleteFile(meta.ID, "wrong-token")
	if err != ErrInvalidToken {
		t.Fatalf("Expected ErrInvalidToken, got %v", err)
	}

	// 5. Delete with valid token
	err = store.DeleteFile(meta.ID, meta.DeleteToken)
	if err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	// 6. Verify deleted
	_, err = store.GetFile(meta.ID)
	if err != ErrFileNotFound {
		t.Fatalf("Expected ErrFileNotFound after deletion, got %v", err)
	}
}

func TestBurnAfterRead(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "og-filehost-test-burn-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStorage(filepath.Join(tempDir, "uploads"), filepath.Join(tempDir, "meta.json"))
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}

	content := []byte("Burn this secret immediately.")
	meta, err := store.SaveFile(bytes.NewReader(content), "secret.key", "application/octet-stream", 1*time.Hour, true, "sess-burn")
	if err != nil {
		t.Fatalf("SaveFile failed: %v", err)
	}

	// Record download (simulating download)
	err = store.RecordDownload(meta.ID)
	if err != nil {
		t.Fatalf("RecordDownload failed: %v", err)
	}

	// After download, file should be gone
	_, err = store.GetFile(meta.ID)
	if err != ErrFileNotFound {
		t.Fatalf("Expected ErrFileNotFound after burn-after-read, got %v", err)
	}
}

func TestExpiredFilePurge(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "og-filehost-test-expire-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStorage(filepath.Join(tempDir, "uploads"), filepath.Join(tempDir, "meta.json"))
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}

	// File with immediate expiration (1 millisecond)
	content := []byte("Expiring fast...")
	meta, err := store.SaveFile(bytes.NewReader(content), "quick.txt", "text/plain", 10*time.Millisecond, false, "sess-exp")
	if err != nil {
		t.Fatalf("SaveFile failed: %v", err)
	}

	time.Sleep(30 * time.Millisecond)

	cleaned, err := store.CleanupExpired()
	if err != nil {
		t.Fatalf("CleanupExpired failed: %v", err)
	}
	if cleaned != 1 {
		t.Fatalf("Expected 1 file cleaned, got %d", cleaned)
	}

	_, err = store.GetFile(meta.ID)
	if err != ErrFileNotFound {
		t.Fatalf("Expected ErrFileNotFound for expired file, got %v", err)
	}
}
