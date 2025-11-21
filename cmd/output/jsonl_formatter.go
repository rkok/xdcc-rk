package output

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
	"xdcc-cli/xdcc"
)

// JSONLEvent represents a JSONL event for transfer output
type JSONLEvent struct {
	Type      string  `json:"type"`
	URL       string  `json:"url,omitempty"`
	Timestamp string  `json:"timestamp"`

	// Connecting event fields
	Network string `json:"network,omitempty"`
	Channel string `json:"channel,omitempty"`
	Bot     string `json:"bot,omitempty"`
	Slot    int    `json:"slot,omitempty"`
	SSL     bool   `json:"ssl,omitempty"`

	// Started/Progress/Completed event fields
	FileName         string  `json:"fileName,omitempty"`
	FileSize         uint64  `json:"fileSize,omitempty"`
	FilePath         string  `json:"filePath,omitempty"`
	BytesTransferred uint64  `json:"bytesTransferred,omitempty"`
	TotalBytes       uint64  `json:"totalBytes,omitempty"`
	Percentage       float64 `json:"percentage,omitempty"`
	TransferRate     float64 `json:"transferRate,omitempty"`
	Duration         float64 `json:"duration,omitempty"`
	AvgRate          float64 `json:"avgRate,omitempty"`

	// Error event fields
	Error     string `json:"error,omitempty"`
	ErrorType string `json:"errorType,omitempty"`
	Fatal     bool   `json:"fatal,omitempty"`

	// Retry event fields
	Attempt     int    `json:"attempt,omitempty"`
	MaxAttempts int    `json:"maxAttempts,omitempty"`
	Reason      string `json:"reason,omitempty"`

	// Finished event fields
	TotalTransfers int `json:"totalTransfers,omitempty"`
	Successful     int `json:"successful,omitempty"`
	Failed         int `json:"failed,omitempty"`
}

// JSONLFormatter implements TransferOutputFormatter for JSONL output
type JSONLFormatter struct {
	urlStr string
}

// NewJSONLFormatter creates a new JSONL formatter
func NewJSONLFormatter(urlStr string) *JSONLFormatter {
	return &JSONLFormatter{
		urlStr: urlStr,
	}
}

// EmitEvent emits a JSONL event to stdout (exported for standalone event emission)
func (f *JSONLFormatter) EmitEvent(event JSONLEvent) {
	event.Timestamp = time.Now().UTC().Format(time.RFC3339)
	jsonBytes, err := json.Marshal(event)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting JSONL: %v\n", err)
		return
	}
	fmt.Println(string(jsonBytes))
	os.Stdout.Sync() // Flush immediately for streaming
}

// emitEvent is a convenience wrapper for internal use
func (f *JSONLFormatter) emitEvent(event JSONLEvent) {
	f.EmitEvent(event)
}

func (f *JSONLFormatter) OnConnecting(event *xdcc.TransferConnectingEvent) {
	f.emitEvent(JSONLEvent{
		Type:    "connecting",
		URL:     event.URL,
		Network: event.Network,
		Channel: event.Channel,
		Bot:     event.Bot,
		Slot:    event.Slot,
		SSL:     event.SSL,
	})
}

func (f *JSONLFormatter) OnConnected(event *xdcc.TransferConnectedEvent) {
	f.emitEvent(JSONLEvent{
		Type: "connected",
		URL:  event.URL,
	})
}

func (f *JSONLFormatter) OnStarted(event *xdcc.TransferStartedEvent) {
	f.emitEvent(JSONLEvent{
		Type:     "started",
		URL:      f.urlStr,
		FileName: event.FileName,
		FileSize: event.FileSize,
		FilePath: event.FilePath,
	})
}

func (f *JSONLFormatter) OnProgress(event *xdcc.TransferProgessEvent, totalBytes uint64) {
	percentage := 0.0
	if totalBytes > 0 {
		percentage = (float64(event.TransferBytes) / float64(totalBytes)) * 100.0
	}
	f.emitEvent(JSONLEvent{
		Type:             "progress",
		URL:              f.urlStr,
		BytesTransferred: event.TransferBytes,
		TotalBytes:       totalBytes,
		Percentage:       percentage,
		TransferRate:     float64(event.TransferRate),
	})
}

func (f *JSONLFormatter) OnCompleted(event *xdcc.TransferCompletedEvent) {
	f.emitEvent(JSONLEvent{
		Type:     "completed",
		URL:      f.urlStr,
		FileName: event.FileName,
		FileSize: event.FileSize,
		FilePath: event.FilePath,
		Duration: event.Duration,
		AvgRate:  event.AvgRate,
	})
}

func (f *JSONLFormatter) OnError(event *xdcc.TransferErrorEvent) {
	f.emitEvent(JSONLEvent{
		Type:      "error",
		URL:       event.URL,
		Error:     event.Error,
		ErrorType: event.ErrorType,
		Fatal:     event.Fatal,
	})
}

func (f *JSONLFormatter) OnAborted(event *xdcc.TransferAbortedEvent) {
	f.emitEvent(JSONLEvent{
		Type:   "aborted",
		URL:    f.urlStr,
		Reason: event.Error,
	})
}

func (f *JSONLFormatter) OnRetry(event *xdcc.TransferRetryEvent) {
	f.emitEvent(JSONLEvent{
		Type:        "retry",
		URL:         event.URL,
		Attempt:     event.Attempt,
		MaxAttempts: event.MaxAttempts,
		Reason:      event.Reason,
	})
}

