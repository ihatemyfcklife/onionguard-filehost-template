package views

import (
	_ "embed"
	"encoding/base64"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	og "github.com/ihatemyfcklife/onionguard"
	"onionguard-filehost/internal/model"
)

//go:embed assets/logo.png
var LogoPNG []byte

// LogoDataURI contains the base64 encoded PNG for instant zero-JS embedding without extra requests.
var LogoDataURI string

func init() {
	LogoDataURI = "data:image/png;base64," + base64.StdEncoding.EncodeToString(LogoPNG)
}

// CommonCSS returns the clean, minimalist stylesheet ensuring zero-JS compatibility with OnionGuard's CSP.
const CommonCSS = `
:root {
  --bg: #0b0e14;
  --card: #151921;
  --input: #0d1117;
  --border: #262c36;
  --text: #e6edf3;
  --muted: #7d8590;
  --accent: #238636;
  --accent-hover: #2ea043;
  --danger: #da3633;
  --danger-hover: #b92b27;
  --code-bg: #10141c;
}
* { box-sizing: border-box; margin: 0; padding: 0; }
body {
  background: var(--bg);
  color: var(--text);
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
  line-height: 1.5;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}
.container {
  max-width: 680px;
  margin: 0 auto;
  padding: 1.25rem;
  width: 100%;
}
header {
  border-bottom: 1px solid var(--border);
  padding: 0.85rem 0;
  margin-bottom: 2rem;
}
.header-inner {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.brand {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  text-decoration: none;
  color: var(--text);
}
.brand-logo {
  height: 28px;
  width: auto;
  display: block;
}
.brand-title {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text);
  letter-spacing: 0.5px;
}
.status-pill {
  font-size: 0.75rem;
  font-family: monospace;
  color: var(--muted);
  background: var(--card);
  border: 1px solid var(--border);
  padding: 0.25rem 0.6rem;
  border-radius: 999px;
  display: flex;
  align-items: center;
  gap: 0.4rem;
}
.status-dot {
  color: #00ff80;
  font-weight: bold;
}
.card {
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 1.5rem;
  margin-bottom: 1.25rem;
}
h1, h2, h3 {
  color: #fff;
  font-size: 1.25rem;
  margin-bottom: 1rem;
}
p {
  color: var(--muted);
  font-size: 0.9rem;
  margin-bottom: 1rem;
}
.form-group {
  margin-bottom: 1.25rem;
}
label {
  display: block;
  font-weight: 500;
  margin-bottom: 0.35rem;
  font-size: 0.85rem;
  color: var(--text);
}
input[type="file"], select, input[type="text"] {
  width: 100%;
  padding: 0.6rem 0.75rem;
  background: var(--input);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text);
  font-size: 0.9rem;
  font-family: inherit;
}
input[type="file"] {
  border-style: dashed;
  cursor: pointer;
}
.checkbox-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  cursor: pointer;
  font-size: 0.85rem;
  color: var(--text);
}
.btn {
  display: inline-block;
  background: var(--accent);
  color: #fff;
  border: none;
  border-radius: 6px;
  padding: 0.65rem 1.25rem;
  font-size: 0.9rem;
  font-weight: 600;
  text-decoration: none;
  cursor: pointer;
  text-align: center;
}
.btn:hover {
  background: var(--accent-hover);
}
.btn-block {
  width: 100%;
}
.btn-danger {
  background: var(--danger);
}
.btn-danger:hover {
  background: var(--danger-hover);
}
.btn-outline {
  background: transparent;
  border: 1px solid var(--border);
  color: var(--text);
}
.btn-outline:hover {
  background: var(--input);
}
.meta-table {
  width: 100%;
  border-collapse: collapse;
  margin-top: 0.75rem;
  font-size: 0.85rem;
}
.meta-table th, .meta-table td {
  padding: 0.5rem 0.6rem;
  border-bottom: 1px solid var(--border);
  text-align: left;
}
.meta-table th {
  color: var(--muted);
  width: 35%;
  font-weight: 500;
}
.meta-table td {
  color: var(--text);
  font-family: monospace;
}
.code-box {
  background: var(--code-bg);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 0.6rem 0.85rem;
  font-family: monospace;
  font-size: 0.8rem;
  word-break: break-all;
  color: #58a6ff;
  margin-top: 0.35rem;
}
.alert {
  padding: 0.75rem 1rem;
  border-radius: 6px;
  margin-bottom: 1rem;
  font-size: 0.85rem;
  background: rgba(35, 134, 54, 0.15);
  border: 1px solid var(--accent);
  color: #7ee787;
}
.preview-box {
  margin-top: 0.75rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 0.85rem;
  background: var(--input);
  overflow: auto;
  max-height: 320px;
}
.preview-box pre {
  font-family: monospace;
  font-size: 0.8rem;
  white-space: pre-wrap;
  word-break: break-all;
}
.preview-box img {
  max-width: 100%;
  height: auto;
  display: block;
  margin: 0 auto;
}
.disclaimer-banner {
  background: rgba(218, 54, 51, 0.1);
  border: 1px solid rgba(218, 54, 51, 0.3);
  color: #ff7b72;
  padding: 0.6rem 0.85rem;
  border-radius: 6px;
  font-size: 0.75rem;
  margin-bottom: 1.25rem;
  text-align: center;
  line-height: 1.4;
}
.disclaimer-banner a {
  color: #58a6ff;
  text-decoration: underline;
}
footer {
  margin-top: auto;
  border-top: 1px solid var(--border);
  padding: 1.25rem 0;
  text-align: center;
  color: var(--muted);
  font-size: 0.75rem;
}
`

// HeaderHTML renders a clean, minimal header with logo and session status.
func HeaderHTML(title string, identity og.ClientIdentity) string {
	sessBadge := "anonymous"
	if identity.SessionID != "" {
		sessBadge = og.MaskSessionID(identity.SessionID)
	}

	return fmt.Sprintf(`
<header>
  <div class="container header-inner">
    <a href="/" class="brand">
      <img src="%s" alt="OnionGuard" class="brand-logo">
      <span class="brand-title">File Host</span>
    </a>
    <div class="status-pill">
      <span class="status-dot">•</span>
      <span>%s</span>
    </div>
  </div>
</header>
`, LogoDataURI, html.EscapeString(sessBadge))
}

// FooterHTML renders a subtle one-line footer with link to OnionGuard.
func FooterHTML() string {
	return `
<footer>
  <div class="container">
    <p>Reference example for <a href="https://github.com/ihatemyfcklife/onionguard" style="color:#58a6ff;text-decoration:none;">OnionGuard</a> • Zero-JS • Not production-ready or security-audited</p>
  </div>
</footer>
`
}

// RenderHomePage generates a minimalist file upload interface.
func RenderHomePage(title string, identity og.ClientIdentity, maxUploadMB int, sessionFiles []*model.FileMetadata, alertMsg string) string {
	var filesTable strings.Builder
	if len(sessionFiles) > 0 {
		filesTable.WriteString(`
<div class="card">
  <h2>Your Uploads</h2>
  <table class="meta-table">
    <thead>
      <tr>
        <th>File</th>
        <th>Size</th>
        <th>Expires</th>
        <th>Action</th>
      </tr>
    </thead>
    <tbody>
`)
		for _, f := range sessionFiles {
			expiresStr := "Permanent"
			if !f.ExpiresAt.IsZero() {
				expiresStr = f.ExpiresAt.UTC().Format("01-02 15:04")
			}
			filesTable.WriteString(fmt.Sprintf(`
      <tr>
        <td><a href="/v/%s" style="color:#58a6ff;text-decoration:none;">%s</a></td>
        <td>%s</td>
        <td>%s</td>
        <td>
          <a href="/d/%s" class="btn btn-outline" style="padding:0.2rem 0.5rem;font-size:0.75rem;">Get</a>
          <a href="/v/%s" class="btn btn-outline" style="padding:0.2rem 0.5rem;font-size:0.75rem;">Manage</a>
        </td>
      </tr>
`, html.EscapeString(f.ID), html.EscapeString(f.OriginalName), formatBytes(f.Size), expiresStr, html.EscapeString(f.ID), html.EscapeString(f.ID)))
		}
		filesTable.WriteString(`
    </tbody>
  </table>
</div>
`)
	}

	var alertHTML string
	if alertMsg != "" {
		alertHTML = fmt.Sprintf(`<div class="alert">%s</div>`, html.EscapeString(alertMsg))
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s</title>
  <style>%s</style>
</head>
<body>
  %s
  <main class="container">
    <div class="disclaimer-banner">
      Notice: This project is a simple reference demonstration of <a href="https://github.com/ihatemyfcklife/onionguard">OnionGuard</a>. It is NOT production-ready, NOT audited, and NOT safe for production use.
    </div>
    %s
    <div class="card">
      <h2>Upload File</h2>
      <form method="POST" action="/upload" enctype="multipart/form-data">
        <div class="form-group">
          <label for="file">File (max %d MB)</label>
          <input type="file" id="file" name="file" required>
        </div>

        <div class="form-group">
          <label for="ttl">Retention</label>
          <select id="ttl" name="ttl">
            <option value="24h" selected>24 hours</option>
            <option value="1h">1 hour</option>
            <option value="7d">7 days</option>
            <option value="30d">30 days</option>
          </select>
        </div>

        <div class="form-group">
          <label class="checkbox-row">
            <input type="checkbox" name="burn" value="1">
            <span>Burn after read (delete after first download)</span>
          </label>
        </div>

        <button type="submit" class="btn btn-block">Upload</button>
      </form>
    </div>

    %s

    <div class="card">
      <label>cURL Upload</label>
      <div class="code-box">curl -F "file=@sample.txt" http://localhost:8080/upload</div>
    </div>
  </main>
  %s
</body>
</html>`,
		html.EscapeString(title),
		CommonCSS,
		HeaderHTML(title, identity),
		alertHTML,
		maxUploadMB,
		filesTable.String(),
		FooterHTML(),
	)
}

// RenderFileView generates the details and download page for an uploaded file.
func RenderFileView(title string, identity og.ClientIdentity, meta *model.FileMetadata, deleteToken string, previewContent string, hostURL string) string {
	downloadURL := fmt.Sprintf("/d/%s", meta.ID)
	fullDownloadURL := fmt.Sprintf("%s%s", hostURL, downloadURL)

	expiresStr := "Permanent"
	if !meta.ExpiresAt.IsZero() {
		expiresStr = meta.ExpiresAt.UTC().Format("2006-01-02 15:04:05 UTC")
	}

	burnBadge := "No"
	if meta.BurnAfterRead {
		burnBadge = "Yes (burn after read)"
	}

	var previewHTML string
	if previewContent != "" {
		if strings.HasPrefix(meta.ContentType, "image/") {
			previewHTML = fmt.Sprintf(`
<div class="card">
  <h3>Preview</h3>
  <div class="preview-box">
    <img src="/d/%s" alt="%s">
  </div>
</div>
`, html.EscapeString(meta.ID), html.EscapeString(meta.OriginalName))
		} else if strings.HasPrefix(meta.ContentType, "text/") {
			previewHTML = fmt.Sprintf(`
<div class="card">
  <h3>Preview</h3>
  <div class="preview-box">
    <pre>%s</pre>
  </div>
</div>
`, html.EscapeString(previewContent))
		}
	}

	var deletionForm string
	activeToken := deleteToken
	if activeToken == "" {
		activeToken = meta.DeleteToken
	}

	if activeToken != "" {
		deletionForm = fmt.Sprintf(`
<div class="card">
  <h3>Delete File</h3>
  <form method="POST" action="/delete/%s">
    <input type="hidden" name="token" value="%s">
    <button type="submit" class="btn btn-danger btn-block">Delete Now</button>
  </form>
</div>
`, html.EscapeString(meta.ID), html.EscapeString(activeToken))
	}

	var alertBanner string
	if deleteToken != "" {
		alertBanner = `<div class="alert">File uploaded successfully. Save your download link below.</div>`
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s - %s</title>
  <style>%s</style>
</head>
<body>
  %s
  <main class="container">
    %s
    <div class="card">
      <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:1rem; flex-wrap:wrap; gap:0.5rem;">
        <h2>%s</h2>
        <a href="%s" class="btn">Download</a>
      </div>

      <table class="meta-table">
        <tbody>
          <tr><th>Size</th><td>%s (%d bytes)</td></tr>
          <tr><th>MIME</th><td>%s</td></tr>
          <tr><th>SHA-256</th><td>%s</td></tr>
          <tr><th>Uploaded</th><td>%s</td></tr>
          <tr><th>Expires</th><td>%s</td></tr>
          <tr><th>Burn</th><td>%s</td></tr>
          <tr><th>Downloads</th><td>%d</td></tr>
        </tbody>
      </table>

      <div style="margin-top:1.25rem;">
        <label>Download URL</label>
        <div class="code-box">%s</div>
      </div>
    </div>

    %s
    %s

    <p style="text-align:center;"><a href="/" class="btn btn-outline">Upload Another</a></p>
  </main>
  %s
</body>
</html>`,
		html.EscapeString(meta.OriginalName),
		html.EscapeString(title),
		CommonCSS,
		HeaderHTML(title, identity),
		alertBanner,
		html.EscapeString(meta.OriginalName),
		html.EscapeString(downloadURL),
		formatBytes(meta.Size), meta.Size,
		html.EscapeString(meta.ContentType),
		html.EscapeString(meta.SHA256),
		meta.UploadedAt.UTC().Format("2006-01-02 15:04:05 UTC"),
		expiresStr,
		burnBadge,
		meta.DownloadCount,
		html.EscapeString(fullDownloadURL),
		previewHTML,
		deletionForm,
		FooterHTML(),
	)
}

// RenderErrorPage renders a friendly zero-JS error page matching the minimalist design.
func RenderErrorPage(title string, identity og.ClientIdentity, statusCode int, errorCode, message string) string {
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Error %d - %s</title>
  <style>%s</style>
</head>
<body>
  %s
  <main class="container">
    <div class="card" style="text-align:center; padding:2.5rem 1rem;">
      <h2 style="color:var(--danger); margin-bottom:0.5rem;">Error %d</h2>
      <p style="margin-bottom:1.5rem;">%s</p>
      <a href="/" class="btn btn-outline">Back</a>
    </div>
  </main>
  %s
</body>
</html>`,
		statusCode, html.EscapeString(title),
		CommonCSS,
		HeaderHTML(title, identity),
		statusCode,
		html.EscapeString(message),
		FooterHTML(),
	)
}

// CustomWaitRoomHTML provides a minimalist, clean waiting room.
func CustomWaitRoomHTML(r *http.Request, retry time.Duration) string {
	sec := int(retry.Seconds())
	if sec < 1 {
		sec = 1
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta http-equiv="refresh" content="%d">
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data:; style-src 'unsafe-inline'; base-uri 'none'; frame-ancestors 'none'">
  <title>Please Wait</title>
  <style>
    body { background:#0b0e14; color:#e6edf3; font-family:system-ui,-apple-system,sans-serif; display:flex; justify-content:center; align-items:center; height:100vh; margin:0; }
    .card { background:#151921; border:1px solid #262c36; border-radius:8px; padding:2rem; max-width:360px; width:90%%; text-align:center; }
    .logo { height:32px; width:auto; display:block; margin:0 auto 1.25rem auto; }
    h2 { margin:0 0 0.5rem 0; font-size:1.15rem; color:#fff; }
    p { color:#7d8590; font-size:0.85rem; margin:0 0 1rem 0; }
    .timer { font-size:2rem; font-weight:700; color:#00ff80; font-family:monospace; margin:0.75rem 0; }
  </style>
</head>
<body>
  <div class="card">
    <img src="%s" alt="OnionGuard" class="logo">
    <h2>Please Wait</h2>
    <div class="timer">%d s</div>
    <p>Redirecting automatically...</p>
    <p style="font-size:0.75rem; color:#7d8590; margin-top:0.75rem;"><a href="https://github.com/ihatemyfcklife/onionguard" style="color:#58a6ff; text-decoration:none;">OnionGuard</a> PoC - Not production-ready</p>
  </div>
</body>
</html>`, sec, LogoDataURI, sec)
}

// CustomChallengeHTML provides a minimalist, clean anti-bot challenge.
func CustomChallengeHTML(r *http.Request, ch *og.CaptchaChallenge, cfg og.CaptchaConfig) string {
	target := "/"
	if r != nil {
		if r.URL != nil && r.URL.Path != cfg.EndpointPath {
			target = r.RequestURI
		} else {
			target = r.FormValue("target")
		}
	}
	if target == "" {
		target = "/"
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data:; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'">
  <title>Verification</title>
  <style>
    body { background:#0b0e14; color:#e6edf3; font-family:system-ui,-apple-system,sans-serif; display:flex; justify-content:center; align-items:center; height:100vh; margin:0; }
    .card { background:#151921; border:1px solid #262c36; border-radius:8px; padding:2rem; max-width:360px; width:90%%; text-align:center; }
    .logo { height:32px; width:auto; display:block; margin:0 auto 1.25rem auto; }
    h2 { margin:0 0 0.35rem 0; font-size:1.15rem; color:#fff; }
    p { color:#7d8590; font-size:0.85rem; margin:0 0 1rem 0; }
    form { display:flex; flex-direction:column; gap:0.85rem; align-items:center; }
    img.captcha { border:1px solid #262c36; border-radius:6px; background:#10141c; display:block; }
    input[type="text"] { width:100%%; max-width:220px; padding:0.55rem; background:#0d1117; border:1px solid #262c36; border-radius:6px; color:#00ff80; font-family:monospace; font-size:1.1rem; text-align:center; letter-spacing:2px; }
    button { background:#238636; color:#fff; border:none; border-radius:6px; padding:0.6rem 1.25rem; font-weight:600; cursor:pointer; width:100%%; max-width:220px; font-size:0.9rem; }
    button:hover { background:#2ea043; }
  </style>
</head>
<body>
  <div class="card">
    <img src="%s" alt="OnionGuard" class="logo">
    <h2>Verification</h2>
    <p>Enter the code shown below</p>
    <form method="POST" action="%s">
      <input type="hidden" name="target" value="%s">
      <img class="captcha" src="%s" alt="CAPTCHA" width="%d" height="%d">
      <input type="text" name="answer" maxlength="%d" placeholder="Code" autofocus required autocomplete="off">
      <button type="submit">Continue</button>
    </form>
    <p style="font-size:0.75rem; color:#7d8590; margin-top:0.75rem;"><a href="https://github.com/ihatemyfcklife/onionguard" style="color:#58a6ff; text-decoration:none;">OnionGuard</a> PoC - Not production-ready</p>
  </div>
</body>
</html>`,
		LogoDataURI,
		html.EscapeString(cfg.EndpointPath),
		html.EscapeString(target),
		ch.ImageDataURI,
		cfg.Width, cfg.Height,
		cfg.Length,
	)
}

// CustomErrorHTML provides OnionGuard with styled error responses.
func CustomErrorHTML(r *http.Request, adm *og.AdmissionError) string {
	retryMsg := ""
	if adm.RetryAfter > 0 {
		retryMsg = fmt.Sprintf("<p>Please wait %d second(s) before trying again.</p>", int(adm.RetryAfter.Seconds()))
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Access Denied</title>
  <style>
    body { background:#0b0e14; color:#f85149; font-family:system-ui,-apple-system,sans-serif; display:flex; justify-content:center; align-items:center; height:100vh; margin:0; }
    .card { background:#151921; border:1px solid #da3633; border-radius:8px; padding:2rem; max-width:360px; text-align:center; }
    h2 { color:#f85149; margin-bottom:0.75rem; font-size:1.15rem; }
    p { color:#7d8590; margin-bottom:1rem; font-size:0.85rem; }
    a { color:#58a6ff; text-decoration:none; }
  </style>
</head>
<body>
  <div class="card">
    <h2>Access Denied (%d)</h2>
    <p>%s</p>
    %s
    <a href="/">Back</a>
  </div>
</body>
</html>`, adm.StatusCode, html.EscapeString(adm.Message), retryMsg)
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
