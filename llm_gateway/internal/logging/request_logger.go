package logging

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Helper functions for environment variables
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := fmt.Sscanf(value, "%d", new(int)); err == nil && intVal == 1 {
			var result int
			fmt.Sscanf(value, "%d", &result)
			return result
		}
	}
	return defaultValue
}

func getEnvByteSize(key string, defaultValue int) int {
	return getEnvInt(key, defaultValue)
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// RequestLog defines the JSON structure for a log entry.
type RequestLog struct {
	Timestamp  time.Time           `json:"timestamp"`
	Method     string              `json:"method"`
	URL        string              `json:"url"`
	Headers    map[string][]string `json:"headers"`
	RemoteAddr string              `json:"remote_addr"`
	Body       string              `json:"body"`
}

// RequestLogger implements asynchronous, buffered logging with rotation and periodic flush.
type RequestLogger struct {
	fileTemplate  string        // template for log file name e.g. "/var/log/api-gateway/requests-%s.jsonl"
	maxSize       int64         // maximum size in bytes before rotation
	maxFiles      int           // maximum number of rotated files to keep
	flushInterval time.Duration // flush the buffer every flushInterval if not empty

	mu          sync.Mutex
	currentFile string // current active file name (populated from fileTemplate)
	file        *os.File
	writer      *bufio.Writer
	currentSize int64

	logCh  chan RequestLog
	doneCh chan struct{}
	wg     sync.WaitGroup
}

// newFileName generates a new log filename by applying the current timestamp
// to the fileTemplate. The timestamp format used is "20060102150405".
func (logger *RequestLogger) newFileName() string {
	timestamp := time.Now().Format("20060102150405")
	return fmt.Sprintf(logger.fileTemplate, timestamp)
}

// openFile opens (or creates) the active log file using the file template and prepares the buffered writer.
// It also ensures that the directory for the log file exists.
func (logger *RequestLogger) openFile() error {
	logger.currentFile = logger.newFileName()
	// Ensure the directory exists
	dir := filepath.Dir(logger.currentFile)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	file, err := os.OpenFile(logger.currentFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	fi, err := file.Stat()
	if err != nil {
		file.Close()
		return err
	}
	logger.currentSize = fi.Size()
	logger.file = file
	logger.writer = bufio.NewWriter(file)
	return nil
}

// rotateIfNeeded checks if adding n bytes would exceed the max file size,
// and if so rotates the file by closing the current file and opening a new one.
func (logger *RequestLogger) rotateIfNeeded(n int) error {
	logger.mu.Lock()
	defer logger.mu.Unlock()

	// if we haven't reached the max size yet, nothing to do
	if logger.currentSize+int64(n) < logger.maxSize {
		return nil
	}

	// flush and close current file
	if err := logger.writer.Flush(); err != nil {
		return err
	}
	if err := logger.file.Close(); err != nil {
		return err
	}

	// Open a new file (which will have a new timestamp)
	if err := logger.openFile(); err != nil {
		return err
	}
	return nil
}

// cleanupOldFiles removes the oldest rotated files if more than maxFiles exist.
func (logger *RequestLogger) cleanupOldFiles() error {
	// Build the glob pattern by replacing "%s" with "*"
	pattern := fmt.Sprintf(logger.fileTemplate, "*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	// Sort files by modification time.
	sort.Slice(matches, func(i, j int) bool {
		fi, err1 := os.Stat(matches[i])
		fj, err2 := os.Stat(matches[j])
		if err1 != nil || err2 != nil {
			return false
		}
		return fi.ModTime().Before(fj.ModTime())
	})

	// Delete oldest files if there are more than maxFiles.
	excess := len(matches) - logger.maxFiles
	for i := 0; i < excess; i++ {
		_ = os.Remove(matches[i])
	}
	return nil
}

// run is the goroutine that listens for log entries and writes them to disk.
// It also uses a ticker to periodically flush the buffer.
func (logger *RequestLogger) run() {
	defer logger.wg.Done()
	ticker := time.NewTicker(logger.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case entry := <-logger.logCh:
			logger.writeEntry(entry)
		case <-ticker.C:
			// Flush periodically.
			logger.mu.Lock()
			_ = logger.writer.Flush()
			logger.mu.Unlock()
		case <-logger.doneCh:
			// Drain remaining log entries.
			for {
				select {
				case entry := <-logger.logCh:
					logger.writeEntry(entry)
				default:
					logger.mu.Lock()
					_ = logger.writer.Flush()
					_ = logger.file.Close()
					logger.mu.Unlock()
					return
				}
			}
		}
	}
}

// writeEntry serializes a RequestLog to JSON and writes it, rotating if needed.
func (logger *RequestLogger) writeEntry(entry RequestLog) {
	data, err := json.Marshal(entry)
	if err != nil {
		// If marshaling fails, skip the log entry.
		return
	}
	line := string(data) + "\n"
	n := len(line)
	// Check and perform rotation if needed.
	if err := logger.rotateIfNeeded(n); err != nil {
		// In a real system, you might want to log this error somewhere.
	}
	logger.mu.Lock()
	_, _ = logger.writer.WriteString(line)
	logger.currentSize += int64(n)
	logger.mu.Unlock()

	// Optionally, clean up rotated files.
	// This could be done periodically instead of on every write.
	_ = logger.cleanupOldFiles()
}

// LogRequest queues a request for logging. If the queue is full, the log entry is dropped.
func (logger *RequestLogger) LogRequest(r *http.Request) {
	headers := make(map[string][]string, len(r.Header))
	for k, v := range r.Header {
		// Skip Authorization header
		if k == "Authorization" {
			continue
		}
		headers[k] = v
	}

	// Read the request body and store it as a string.
	var bodyStr string
	if r.Body != nil {
		// Read the request body.
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil {
			bodyStr = string(bodyBytes)
		}
		// Reset the request body so the handler can read it.
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	entry := RequestLog{
		Timestamp:  time.Now(),
		Method:     r.Method,
		URL:        r.URL.String(),
		Headers:    headers,
		RemoteAddr: r.RemoteAddr,
		Body:       bodyStr,
	}
	select {
	case logger.logCh <- entry:
	default:
		// Queue full; dropping log entry.
	}
}

// Shutdown signals the logger to flush its buffer and close the file.
// Call Shutdown() from your application’s graceful shutdown handler.
func (logger *RequestLogger) Shutdown() {
	close(logger.doneCh)
	logger.wg.Wait()
}

// NewLogger creates a new RequestLogger.
// bufferSize determines how many log entries can be queued before writes block.
// flushInterval defines how often the logger should flush its buffer.
func NewLogger(fileTemplate string, maxSize int64, maxFiles, bufferSize int, flushInterval time.Duration) (*RequestLogger, error) {
	logger := &RequestLogger{
		fileTemplate:  fileTemplate,
		maxSize:       maxSize,
		maxFiles:      maxFiles,
		flushInterval: flushInterval,
		logCh:         make(chan RequestLog, bufferSize),
		doneCh:        make(chan struct{}),
	}

	if err := logger.openFile(); err != nil {
		return nil, err
	}

	logger.wg.Add(1)
	go logger.run()

	return logger, nil
}

var RLogger *RequestLogger

func init() {
	// Read configuration from environment variables
	// API_GATEWAY_LOG_FILE_PATH_TEMPLATE is now a template, e.g. "/var/log/requests-%s.jsonl"
	fileTemplate := getEnv("LLM_GATEWAY_LOG_FILE_PATH_TEMPLATE", "/var/log/llm-gateway/requests-%s.jsonl")
	maxSize := getEnvByteSize("LLM_GATEWAY_LOG_MAX_SIZE", 10_485_760)                 // default 10 MB
	maxFiles := getEnvInt("LLM_GATEWAY_LOG_MAX_FILES", 5)                             // default 5
	bufferSize := getEnvInt("LLM_GATEWAY_LOG_BUFFER_SIZE", 100)                       // default 100
	flushInterval := getEnvDuration("LLM_GATEWAY_LOG_FLUSH_INTERVAL", 60*time.Second) // default 60 seconds

	// Create the logger using the file template.
	var err error
	RLogger, err = NewLogger(fileTemplate, int64(maxSize), maxFiles, bufferSize, flushInterval)
	if err != nil {
		// Errorf is assumed to be a helper that logs errors.
		Errorf("Failed to create request logger: %v", err)
		// If logger creation failed, we schedule shutdown (if it was partially created).
		if RLogger != nil {
			defer RLogger.Shutdown()
		}
	}
}
