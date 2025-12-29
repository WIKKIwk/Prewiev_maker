package main

import (
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"pro-banana-ai-bot/internal/gemini"
	"pro-banana-ai-bot/internal/httpclient"
	"pro-banana-ai-bot/internal/preview"
)

//go:embed static/*
var staticFS embed.FS

type server struct {
	gem *gemini.Client
}

type apiError struct {
	Error string `json:"error"`
}

type previewResponse struct {
	Images  []string `json:"images"`
	Warning string   `json:"warning,omitempty"`
}

func main() {
	_ = godotenv.Load()

	apiKey := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	if apiKey == "" {
		panic("GEMINI_API_KEY is required")
	}

	addr := strings.TrimSpace(getEnv("WEB_ADDR", ":8080"))

	httpTimeout := time.Duration(getEnvInt("HTTP_TIMEOUT_SECONDS", 180)) * time.Second
	if httpTimeout <= 0 {
		httpTimeout = 180 * time.Second
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	httpClient := httpclient.New(httpclient.Options{
		PreferIPv4: getEnvBool("PREFER_IPV4", true),
		Timeout:    httpTimeout,
	})

	gem := gemini.New(gemini.Options{
		APIKey:     apiKey,
		BaseURL:    strings.TrimSpace(getEnv("GEMINI_BASE_URL", "https://generativelanguage.googleapis.com")),
		APIVersion: strings.TrimSpace(getEnv("GEMINI_API_VERSION", "v1beta")),
		HTTPClient: httpClient,
		Logger:     logger,
	})

	s := &server{gem: gem}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/preview", s.handlePreview)

	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticSub)))

	srv := &http.Server{
		Addr:              addr,
		Handler:           withLogging(mux, logger),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       90 * time.Second,
	}

	logger.Info("web started", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server error", "err", err)
	}
}

func (s *server) handlePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	const maxUploadBytes = 25 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)

	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid multipart form"})
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "missing image"})
		return
	}
	defer file.Close()

	imgBytes, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "failed to read image"})
		return
	}

	mimeType := strings.TrimSpace(header.Header.Get("Content-Type"))
	if strings.Contains(mimeType, ";") {
		mimeType = strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0])
	}
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = http.DetectContentType(imgBytes)
	}
	if strings.Contains(mimeType, ";") {
		mimeType = strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0])
	}
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = "image/jpeg"
	}

	opts := preview.Options{
		Mode:          strings.TrimSpace(r.FormValue("mode")),
		GridPreset:    strings.TrimSpace(r.FormValue("grid_preset")),
		VerticalCount: strings.TrimSpace(r.FormValue("vertical_count")),
		AspectRatio:   strings.TrimSpace(r.FormValue("aspect_ratio")),
		ProductType:   strings.TrimSpace(r.FormValue("product_type")),
		VisualStyle:   strings.TrimSpace(r.FormValue("visual_style")),
		Custom:        strings.TrimSpace(r.FormValue("custom")),
		HumanUsage:    parseBool(r.FormValue("human_usage")),
	}

	if raw := strings.TrimSpace(r.FormValue("frame_ids")); raw != "" {
		var ids []string
		if err := json.Unmarshal([]byte(raw), &ids); err == nil {
			opts.FrameIDs = ids
		} else {
			opts.FrameIDs = splitCSV(raw)
		}
	}

	prompt, out := preview.BuildPrompt(opts)

	timeout := time.Duration(getEnvInt("REQUEST_TIMEOUT_SECONDS", 240)) * time.Second
	if timeout <= 0 {
		timeout = 240 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	resp, err := s.gem.Chat(ctx, nil, prompt, []gemini.ImageInput{
		{
			DataBase64: base64.StdEncoding.EncodeToString(imgBytes),
			MimeType:   mimeType,
		},
	}, gemini.ChatOptions{WantImage: true, AspectRatio: out.AspectRatio})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}

	outResp := previewResponse{
		Images: resp.Images,
	}
	if len(resp.Images) != out.Count {
		outResp.Warning = "model returned different image count"
	}

	writeJSON(w, http.StatusOK, outResp)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func parseBool(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return value == "1" || value == "true" || value == "yes" || value == "on" || value == "use"
}

func splitCSV(value string) []string {
	var out []string
	for _, p := range strings.Split(value, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func withLogging(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("http", "method", r.Method, "path", r.URL.Path, "dur_ms", time.Since(start).Milliseconds())
	})
}
