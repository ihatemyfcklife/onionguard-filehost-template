package main

import (
	"bytes"
	"context"
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

	og "github.com/ihatemyfcklife/onionguard"
	"github.com/ihatemyfcklife/onionguard/middleware"
	"onionguard-filehost/internal/config"
	"onionguard-filehost/internal/handler"
	"onionguard-filehost/internal/storage"
	"onionguard-filehost/internal/views"
)

func TestOnionGuardFileHostIntegration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "og-integration-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	uploadDir := filepath.Join(tempDir, "uploads")
	metaFile := filepath.Join(tempDir, "meta.json")

	store, err := storage.NewStorage(uploadDir, metaFile)
	if err != nil {
		t.Fatalf("failed to initialize storage: %v", err)
	}

	appCfg := &config.AppConfig{
		ListenAddr:      ":8080",
		DataDir:         tempDir,
		UploadDir:       uploadDir,
		MetaFilePath:    metaFile,
		MaxUploadBytes:  10 * 1024 * 1024,
		DefaultTTL:      24 * time.Hour,
		WaitRoomEnabled: true,
		WaitRoomSeconds: 1 * time.Second,
		CaptchaEnabled:  true,
		APIToken:        "test-secret-token",
		SiteTitle:       "Test OnionGuard Host",
	}

	ogCfg := og.DefaultConfig()
	ogCfg.MaxBodyBytes = appCfg.MaxUploadBytes + 1024*1024
	ogCfg.WaitRoom.Enabled = true
	ogCfg.WaitRoom.WaitTime = 1 * time.Second
	ogCfg.Captcha.Enabled = true
	ogCfg.CustomWaitRoomHTML = views.CustomWaitRoomHTML
	ogCfg.CustomChallengeHTML = views.CustomChallengeHTML
	ogCfg.CustomErrorHTML = views.CustomErrorHTML

	engine, err := og.New(ogCfg, og.WithTokenValidator(og.TokenValidatorFunc(func(ctx context.Context, rawToken string) (string, bool, error) {
		if rawToken == appCfg.APIToken {
			return "api-test-client", true, nil
		}
		return "", false, nil
	})))
	if err != nil {
		t.Fatalf("failed to initialize onionguard: %v", err)
	}
	defer engine.Close()

	mux := http.NewServeMux()
	h := handler.NewHandler(appCfg, store, engine)
	h.RegisterRoutes(mux)

	// Protected handler with OnionGuard admission control
	ts := httptest.NewServer(middleware.Middleware(engine)(mux))
	defer ts.Close()

	client := ts.Client()

	// --- TEST 1: Unadmitted anonymous request receives WaitRoom or Challenge (Zero-JS) ---
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/", nil)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)

	// Verify security headers
	if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") {
		t.Errorf("Expected strict CSP default-src 'none', got %q", csp)
	}

	// Verify ZERO-JS: absolutely NO script tags
	if strings.Contains(strings.ToLower(bodyStr), "<script") {
		t.Fatalf("Zero-JS violation: found <script> tag in OnionGuard waitroom/challenge response!")
	}

	// Should be either Wait Room (429) or Challenge (403)
	if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusForbidden {
		t.Fatalf("Expected 429 Wait Room or 403 Challenge for new visitor, got %d", resp.StatusCode)
	}

	// --- TEST 2: API Token bypasses challenge (demonstrating OnionGuard Tier 2 authentication) ---
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("file", "tor_document.txt")
	_, _ = part.Write([]byte("Confidential data uploaded via OnionGuard API Token"))
	_ = mw.WriteField("ttl", "1h")
	_ = mw.Close()

	uploadReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/upload", &buf)
	uploadReq.Header.Set("Authorization", "Bearer "+appCfg.APIToken)
	uploadReq.Header.Set("Content-Type", mw.FormDataContentType())
	uploadReq.Header.Set("Accept", "application/json")

	uploadResp, err := client.Do(uploadReq)
	if err != nil {
		t.Fatalf("API upload failed: %v", err)
	}
	defer uploadResp.Body.Close()

	if uploadResp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(uploadResp.Body)
		t.Fatalf("Expected 201 Created via API token, got %d: %s", uploadResp.StatusCode, string(raw))
	}

	var metaResp map[string]any
	if err := json.NewDecoder(uploadResp.Body).Decode(&metaResp); err != nil {
		t.Fatalf("Failed to decode upload JSON: %v", err)
	}

	fileID := metaResp["id"].(string)
	deleteToken := metaResp["delete_token"].(string)
	if fileID == "" || deleteToken == "" {
		t.Fatalf("Invalid response metadata: %+v", metaResp)
	}

	// --- TEST 3: Download file using API token ---
	downReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/d/"+fileID, nil)
	downReq.Header.Set("Authorization", "Bearer "+appCfg.APIToken)
	downResp, err := client.Do(downReq)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}
	defer downResp.Body.Close()

	if downResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK on download, got %d", downResp.StatusCode)
	}
	downBody, _ := io.ReadAll(downResp.Body)
	if !strings.Contains(string(downBody), "Confidential data uploaded via OnionGuard API Token") {
		t.Fatalf("Downloaded content mismatch: %s", string(downBody))
	}

	// --- TEST 4: Delete file with delete token ---
	delReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/delete/"+fileID, strings.NewReader("token="+deleteToken))
	delReq.Header.Set("Authorization", "Bearer "+appCfg.APIToken)
	delReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	delReq.Header.Set("Accept", "application/json")

	delResp, err := client.Do(delReq)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	defer delResp.Body.Close()

	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK on delete, got %d", delResp.StatusCode)
	}
}
