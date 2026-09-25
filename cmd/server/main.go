package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"image/color"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	og "github.com/ihatemyfcklife/onionguard"
	"github.com/ihatemyfcklife/onionguard/middleware"
	"onionguard-filehost/internal/config"
	"onionguard-filehost/internal/handler"
	"onionguard-filehost/internal/storage"
	"onionguard-filehost/internal/views"
)

func main() {
	log.Println("[INFO] Initializing OnionGuard File Host...")

	appCfg := config.LoadConfig()

	// 1. Initialize Storage
	store, err := storage.NewStorage(appCfg.UploadDir, appCfg.MetaFilePath)
	if err != nil {
		log.Fatalf("failed to initialize storage: %v", err)
	}

	janitorCtx, cancelJanitor := context.WithCancel(context.Background())
	defer cancelJanitor()
	store.StartJanitor(janitorCtx, appCfg.CleanupInterval)

	// 2. Configure OnionGuard Admission Control Engine
	ogCfg := og.DefaultConfig()

	// Adjust MaxBodyBytes for file uploads (cap within OnionGuard limit)
	if appCfg.MaxUploadBytes > 100*1024*1024 {
		ogCfg.MaxBodyBytes = 100 * 1024 * 1024
	} else {
		// Add 1 MiB headroom for multipart form boundary and headers
		ogCfg.MaxBodyBytes = appCfg.MaxUploadBytes + 1024*1024
	}

	ogCfg.WaitRoom.Enabled = appCfg.WaitRoomEnabled
	ogCfg.WaitRoom.WaitTime = appCfg.WaitRoomSeconds
	ogCfg.Captcha.Enabled = appCfg.CaptchaEnabled

	// Configure dark theme for CAPTCHA PNG
	ogCfg.Captcha.Visual = og.CaptchaVisualConfig{
		BackgroundColor: color.RGBA{R: 14, G: 17, B: 23, A: 255},  // Dark slate
		TextColor:       color.RGBA{R: 0, G: 255, B: 128, A: 255}, // Neon emerald
		LineColor:       color.RGBA{R: 56, G: 139, B: 253, A: 180}, // Cyan noise
		NoiseLines:      3,
		NoiseRatio:      0.02,
		JitterPixels:    2,
	}

	// Attach Zero-JS custom HTML templates
	ogCfg.CustomWaitRoomHTML = views.CustomWaitRoomHTML
	ogCfg.CustomChallengeHTML = views.CustomChallengeHTML
	ogCfg.CustomErrorHTML = views.CustomErrorHTML

	// Custom Content-Security-Policy to allow inline image previews
	ogCfg.SecurityHeaders.ContentSecurityPolicy = "default-src 'none'; img-src 'self' data:; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'"

	// Engine Options (e.g. Bearer API Token Validator)
	var engineOpts []og.EngineOption
	if appCfg.APIToken != "" {
		log.Printf("[INFO] API Bearer Token authentication enabled (CLI upload bypass active)")
		engineOpts = append(engineOpts, og.WithTokenValidator(og.TokenValidatorFunc(func(ctx context.Context, rawToken string) (string, bool, error) {
			if subtle.ConstantTimeCompare([]byte(rawToken), []byte(appCfg.APIToken)) == 1 {
				return "api-bearer-authorized", true, nil
			}
			return "", false, nil
		})))
	}

	engine, err := og.New(ogCfg, engineOpts...)
	if err != nil {
		log.Fatalf("failed to initialize OnionGuard engine: %v", err)
	}
	defer engine.Close()

	// 3. Register HTTP Routes
	mux := http.NewServeMux()
	h := handler.NewHandler(appCfg, store, engine)
	h.RegisterRoutes(mux)

	// Wrap entire application mux with OnionGuard admission control middleware
	protectedHandler := middleware.Middleware(engine)(mux)

	server := &http.Server{
		Addr:              appCfg.ListenAddr,
		Handler:           protectedHandler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 * 1024,
	}

	// 4. Graceful shutdown handler
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[INFO] OnionGuard File Host listening on %s (Zero-JS mode)", server.Addr)
		log.Printf("[INFO] WaitRoom: %t (%v) | CAPTCHA: %t | Max Body: %d MB",
			ogCfg.WaitRoom.Enabled, ogCfg.WaitRoom.WaitTime, ogCfg.Captcha.Enabled, ogCfg.MaxBodyBytes/(1024*1024))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	<-shutdownChan
	log.Println("\n[INFO] Shutting down gracefully...")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown warning: %v", err)
	}
	if err := engine.Shutdown(shutdownCtx); err != nil {
		log.Printf("onionguard engine shutdown warning: %v", err)
	}

	log.Println("[INFO] OnionGuard File Host stopped successfully.")
}
