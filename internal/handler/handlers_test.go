package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"onionguard-filehost/internal/config"
	"onionguard-filehost/internal/storage"
)

func setupTestApp(t *testing.T) (*Handler, *http.ServeMux, func()) {
	tempDir, err := os.MkdirTemp("", "og-handler-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	uploadDir := filepath.Join(tempDir, "uploads")
	metaFile := filepath.Join(tempDir, "meta.json")

	store, err := storage.NewStorage(uploadDir, metaFile)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}

	cfg := &config.AppConfig{
		ListenAddr:     ":8080",
		DataDir:        tempDir,
		UploadDir:      uploadDir,
		MetaFilePath:   metaFile,
		MaxUploadBytes: 10 * 1024 * 1024,
		DefaultTTL:     24 * time.Hour,
		SiteTitle:      "Test File Host",
	}

	h := NewHandler(cfg, store, nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}

	return h, mux, cleanup
}

func TestHandlerIndex(t *testing.T) {
	_, mux, cleanup := setupTestApp(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "OnionGuard") {
		t.Fatalf("Expected body to contain OnionGuard reference")
	}
	if !strings.Contains(body, "Zero-JS") {
		t.Fatalf("Expected body to mention Zero-JS")
	}
	// Verify no script tags
	if strings.Contains(body, "<script") {
		t.Fatalf("Violation of Zero-JS: found <script> tag in HTML output!")
	}
}

func TestHandlerUploadAndDownload(t *testing.T) {
	_, mux, cleanup := setupTestApp(t)
	defer cleanup()

	// 1. Prepare multipart upload
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	part, err := w.CreateFormFile("file", "secret_manifesto.txt")
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	content := "Freedom, privacy, and sovereignty. Protected by OnionGuard."
	_, _ = io.WriteString(part, content)
	_ = w.WriteField("ttl", "1h")
	_ = w.WriteField("burn", "0")
	_ = w.Close()

	uploadReq := httptest.NewRequest(http.MethodPost, "/upload", &b)
	uploadReq.Header.Set("Content-Type", w.FormDataContentType())
	uploadReq.Header.Set("Accept", "application/json") // API mode
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, uploadReq)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var uploadResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &uploadResp); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	fileID := uploadResp["id"].(string)
	deleteToken := uploadResp["delete_token"].(string)
	if fileID == "" || deleteToken == "" {
		t.Fatalf("Missing file ID or delete token in response")
	}

	// 2. View details
	viewReq := httptest.NewRequest(http.MethodGet, "/v/"+fileID, nil)
	viewReq.Header.Set("Accept", "text/html")
	viewRec := httptest.NewRecorder()

	mux.ServeHTTP(viewRec, viewReq)
	if viewRec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on view, got %d", viewRec.Code)
	}
	if !strings.Contains(viewRec.Body.String(), "secret_manifesto.txt") {
		t.Fatalf("Expected filename in view page")
	}
	// Zero-JS check
	if strings.Contains(viewRec.Body.String(), "<script") {
		t.Fatalf("Violation of Zero-JS in view page")
	}

	// 3. Download
	downReq := httptest.NewRequest(http.MethodGet, "/d/"+fileID, nil)
	downRec := httptest.NewRecorder()

	mux.ServeHTTP(downRec, downReq)
	if downRec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on download, got %d", downRec.Code)
	}
	if downRec.Body.String() != content {
		t.Fatalf("Downloaded content mismatch. Expected %q, got %q", content, downRec.Body.String())
	}

	// 4. Delete file
	delForm := strings.NewReader("token=" + deleteToken)
	delReq := httptest.NewRequest(http.MethodPost, "/delete/"+fileID, delForm)
	delReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	delReq.Header.Set("Accept", "application/json")
	delRec := httptest.NewRecorder()

	mux.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on delete, got %d: %s", delRec.Code, delRec.Body.String())
	}

	// 5. Verify gone
	downReq2 := httptest.NewRequest(http.MethodGet, "/d/"+fileID, nil)
	downRec2 := httptest.NewRecorder()
	mux.ServeHTTP(downRec2, downReq2)
	if downRec2.Code != http.StatusNotFound {
		t.Fatalf("Expected 404 NotFound after deletion, got %d", downRec2.Code)
	}
}

func TestHandlerHealth(t *testing.T) {
	_, mux, cleanup := setupTestApp(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", w.Code)
	}
}
