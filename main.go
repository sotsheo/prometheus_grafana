package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type contextKey string

const requestIDKey contextKey = "requestID"

func generateRequestID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano()%1000000)
	}
	return hex.EncodeToString(b)
}

type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

type RequestLog struct {
	Timestamp  string  `json:"@timestamp"`
	RequestID  string  `json:"request_id"`
	LogType    string  `json:"log_type"`
	Method     string  `json:"method"`
	Path       string  `json:"path"`
	RemoteAddr string  `json:"remote_addr"`
	Status     int     `json:"status"`
	DurationMs float64 `json:"duration_ms"`
	UserAgent  string  `json:"client_user_agent"`
	Bytes      int     `json:"bytes"`
}

func writeRequestLog(reqLog RequestLog) {
	f, err := os.OpenFile("requests.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Error opening requests.log: %v\n", err)
		return
	}
	defer f.Close()

	data, err := json.Marshal(reqLog)
	if err == nil {
		f.WriteString(string(data) + "\n")
	}
}

func requestLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := generateRequestID()
		ctx := context.WithValue(r.Context(), requestIDKey, reqID)
		r = r.WithContext(ctx)

		w.Header().Set("X-Request-ID", reqID)

		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)
		durationMs := float64(duration.Microseconds()) / 1000.0

		reqLog := RequestLog{
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
			RequestID:  reqID,
			LogType:    "request",
			Method:     r.Method,
			Path:       r.URL.Path,
			RemoteAddr: r.RemoteAddr,
			Status:     rw.statusCode,
			DurationMs: durationMs,
			UserAgent:  r.UserAgent(),
			Bytes:      rw.bytesWritten,
		}

		writeRequestLog(reqLog)

		fmt.Printf("[REQUEST] %s | req_id=%s | %d | %s %s | %s | %.2fms\n",
			time.Now().Format("2006-01-02 15:04:05"), reqID, rw.statusCode, r.Method, r.URL.Path, r.RemoteAddr, durationMs)
	})
}

func logAlert(w http.ResponseWriter, r *http.Request) {
	reqID, _ := r.Context().Value(requestIDKey).(string)
	if reqID == "" {
		reqID = generateRequestID()
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed. Use POST.", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	f, err := os.OpenFile("alerts.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("[%s] Error opening alerts.log: %v\n", reqID, err)
		http.Error(w, "Failed to write log", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	if _, err := f.WriteString(string(body) + "\n"); err != nil {
		fmt.Printf("[%s] Error writing to alerts.log: %v\n", reqID, err)
		http.Error(w, "Failed to write alert", http.StatusInternalServerError)
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("[ALERT] %s | req_id=%s | Logged alert: %s\n", timestamp, reqID, string(body))

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Alert received successfully\n"))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/log-alert", logAlert)
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			if r.Method == http.MethodPost {
				logAlert(w, r)
				return
			}
			w.Write([]byte("Alert Logger service is running. Send POST to /log-alert to log alerts.\n"))
			return
		}
		http.NotFound(w, r)
	})

	handler := requestLoggerMiddleware(mux)

	fmt.Println("🚀 Alert & Request logger server running on :5001 (Writing to alerts.log & requests.log)")
	if err := http.ListenAndServe(":5001", handler); err != nil {
		fmt.Printf("Server failed to start: %v\n", err)
	}
}
