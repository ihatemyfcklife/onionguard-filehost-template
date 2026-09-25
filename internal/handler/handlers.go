package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	og "github.com/ihatemyfcklife/onionguard"
	"onionguard-filehost/internal/config"
	"onionguard-filehost/internal/storage"
	"onionguard-filehost/internal/views"
)

// Handler holds dependencies and implements HTTP request routing.
type Handler struct {
	cfg     *config.AppConfig
	storage *storage.Storage
	engine  *og.Engine
}

// NewHandler creates an initialized Handler instance.
func NewHandler(cfg *config.AppConfig, store *storage.Storage, engine *og.Engine) *Handler {
	return &Handler{
		cfg:     cfg,
		storage: store,
		engine:  engine,
	}
}

// RegisterRoutes attaches all application endpoints to the given ServeMux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", h.HandleIndex)
	mux.HandleFunc("POST /upload", h.HandleUpload)
	mux.HandleFunc("GET /v/{id}", h.HandleView)
	mux.HandleFunc("GET /d/{id}", h.HandleDownload)
	mux.HandleFunc("POST /delete/{id}", h.HandleDelete)
	mux.HandleFunc("GET /assets/logo.png", h.HandleLogo)
	mux.HandleFunc("GET /health", h.HandleHealth)
}

// HandleIndex serves the main landing page with file upload form and session files.
func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	identity, _ := og.ClientIdentityFromContext(r.Context())
	sessionFiles := h.storage.ListBySession(identity.SessionID)

	alertMsg := ""
	if r.URL.Query().Get("deleted") == "1" {
		alertMsg = "The file was successfully deleted."
	}

	maxUploadMB := int(h.cfg.MaxUploadBytes / (1024 * 1024))
	body := views.RenderHomePage(h.cfg.SiteTitle, identity, maxUploadMB, sessionFiles, alertMsg)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, body)
}

// HandleUpload receives files via multipart/form-data.
func (h *Handler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	identity, _ := og.ClientIdentityFromContext(r.Context())

	// Parse multipart body
	err := r.ParseMultipartForm(h.cfg.MaxUploadBytes)
	if err != nil {
		h.renderError(w, r, identity, http.StatusBadRequest, "BAD_REQUEST", "Failed to read file: invalid request body or payload exceeds maximum allowed size.")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.renderError(w, r, identity, http.StatusBadRequest, "FILE_MISSING", "Please select a valid file to upload.")
		return
	}
	defer file.Close()

	// Parse TTL
	ttlVal := r.FormValue("ttl")
	ttl := parseTTL(ttlVal, h.cfg.DefaultTTL)

	// Parse Burn after read
	burn := r.FormValue("burn") == "1"

	// Detect content type
	contentType := header.Header.Get("Content-Type")
	if contentType == "" || contentType == "application/octet-stream" {
		// Sniff the first 512 bytes
		sniffBuf := make([]byte, 512)
		n, _ := file.Read(sniffBuf)
		if n > 0 {
			contentType = http.DetectContentType(sniffBuf[:n])
		}
		// Reset file seek
		if seeker, ok := file.(io.Seeker); ok {
			_, _ = seeker.Seek(0, io.SeekStart)
		}
	}

	meta, err := h.storage.SaveFile(file, header.Filename, contentType, ttl, burn, identity.SessionID)
	if err != nil {
		h.renderError(w, r, identity, http.StatusInternalServerError, "STORAGE_ERROR", fmt.Sprintf("Failed to store file: %v", err))
		return
	}

	hostURL := getHostURL(r)
	viewURL := fmt.Sprintf("%s/v/%s", hostURL, meta.ID)
	downloadURL := fmt.Sprintf("%s/d/%s", hostURL, meta.ID)

	if isAPIRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":           meta.ID,
			"name":         meta.OriginalName,
			"size":         meta.Size,
			"sha256":       meta.SHA256,
			"view_url":     viewURL,
			"download_url": downloadURL,
			"delete_token": meta.DeleteToken,
			"burn_read":    meta.BurnAfterRead,
			"expires_at":   meta.ExpiresAt,
		})
		return
	}

	// Browser redirect to view page with deletion token pre-filled
	redirectTarget := fmt.Sprintf("/v/%s?del=%s", meta.ID, meta.DeleteToken)
	http.Redirect(w, r, redirectTarget, http.StatusSeeOther)
}

// HandleView displays file details and links.
func (h *Handler) HandleView(w http.ResponseWriter, r *http.Request) {
	identity, _ := og.ClientIdentityFromContext(r.Context())
	id := r.PathValue("id")
	if id == "" {
		h.renderError(w, r, identity, http.StatusNotFound, "NOT_FOUND", "File not found.")
		return
	}

	meta, err := h.storage.GetFile(id)
	if err != nil {
		h.renderError(w, r, identity, http.StatusNotFound, "NOT_FOUND", "This file does not exist or has expired.")
		return
	}

	deleteToken := r.URL.Query().Get("del")

	// Read small preview if text
	preview := ""
	if strings.HasPrefix(meta.ContentType, "text/") && meta.Size < 65536 {
		if rc, _, err := h.storage.OpenFileReader(id); err == nil {
			buf := make([]byte, 4096)
			n, _ := rc.Read(buf)
			_ = rc.Close()
			if n > 0 {
				preview = html.EscapeString(string(buf[:n]))
			}
		}
	} else if strings.HasPrefix(meta.ContentType, "image/") {
		preview = "image"
	}

	hostURL := getHostURL(r)
	body := views.RenderFileView(h.cfg.SiteTitle, identity, meta, deleteToken, preview, hostURL)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, body)
}

// HandleDownload streams the file content directly to the client.
func (h *Handler) HandleDownload(w http.ResponseWriter, r *http.Request) {
	identity, _ := og.ClientIdentityFromContext(r.Context())
	id := r.PathValue("id")
	if id == "" {
		h.renderError(w, r, identity, http.StatusNotFound, "NOT_FOUND", "File not found.")
		return
	}

	rc, meta, err := h.storage.OpenFileReader(id)
	if err != nil {
		h.renderError(w, r, identity, http.StatusNotFound, "NOT_FOUND", "This file does not exist or has expired.")
		return
	}
	defer rc.Close()

	// Safe filename for Content-Disposition
	safeName := filepath.Base(meta.OriginalName)
	disposition := fmt.Sprintf(`attachment; filename="%s"`, mime.QEncoding.Encode("utf-8", safeName))

	w.Header().Set("Content-Type", meta.ContentType)
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", meta.Size))
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Stream payload
	_, err = io.Copy(w, rc)
	if err == nil {
		// Record download (purges immediately if burn-after-read)
		_ = h.storage.RecordDownload(id)
	}
}

// HandleDelete removes a file using the secret delete token.
func (h *Handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	identity, _ := og.ClientIdentityFromContext(r.Context())
	id := r.PathValue("id")
	token := r.FormValue("token")

	if id == "" || token == "" {
		h.renderError(w, r, identity, http.StatusBadRequest, "INVALID_REQUEST", "Missing file identifier or deletion token.")
		return
	}

	err := h.storage.DeleteFile(id, token)
	if err != nil {
		if errors.Is(err, storage.ErrInvalidToken) {
			h.renderError(w, r, identity, http.StatusForbidden, "FORBIDDEN", "Invalid deletion token.")
			return
		}
		h.renderError(w, r, identity, http.StatusNotFound, "NOT_FOUND", "File not found or already deleted.")
		return
	}

	if isAPIRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  "deleted",
			"message": "File successfully deleted.",
		})
		return
	}

	http.Redirect(w, r, "/?deleted=1", http.StatusSeeOther)
}

// HandleHealth provides a Kubernetes or uptime monitoring endpoint.
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var pingErr error
	if h.engine != nil {
		pingErr = h.engine.Ping(ctx)
	}

	w.Header().Set("Content-Type", "application/json")
	if pingErr != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "degraded",
			"error":  pingErr.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":     "healthy",
		"onionguard": "active",
		"timestamp":  time.Now().UTC(),
	})
}

// HandleLogo serves the embedded OnionGuard logo.
func (h *Handler) HandleLogo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(views.LogoPNG)
}

func (h *Handler) renderError(w http.ResponseWriter, r *http.Request, identity og.ClientIdentity, code int, errCode, msg string) {
	if isAPIRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":   errCode,
			"message": msg,
		})
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	_, _ = io.WriteString(w, views.RenderErrorPage(h.cfg.SiteTitle, identity, code, errCode, msg))
}

func parseTTL(val string, defaultTTL time.Duration) time.Duration {
	switch val {
	case "1h":
		return 1 * time.Hour
	case "24h":
		return 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	default:
		return defaultTTL
	}
}

func isAPIRequest(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	userAgent := r.Header.Get("User-Agent")
	if strings.Contains(accept, "application/json") {
		return true
	}
	if strings.HasPrefix(userAgent, "curl/") || strings.HasPrefix(userAgent, "Wget/") {
		return true
	}
	return false
}

func getHostURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		host = "localhost:8080"
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}
