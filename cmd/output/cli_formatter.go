package output

import (
	"xdcc-cli/pb"
	"xdcc-cli/xdcc"
)

// CLIFormatter implements TransferOutputFormatter for CLI progress bar output
type CLIFormatter struct {
	bar           pb.ProgressBar
	previousBytes uint64
}

// NewCLIFormatter creates a new CLI formatter with a progress bar
func NewCLIFormatter() *CLIFormatter {
	return &CLIFormatter{
		bar:           pb.NewProgressBar(),
		previousBytes: 0,
	}
}

func (f *CLIFormatter) OnConnecting(event *xdcc.TransferConnectingEvent) {
	// CLI formatter doesn't display connecting events
}

func (f *CLIFormatter) OnConnected(event *xdcc.TransferConnectedEvent) {
	// CLI formatter doesn't display connected events
}

func (f *CLIFormatter) OnStarted(event *xdcc.TransferStartedEvent) {
	f.bar.SetTotal(int(event.FileSize))
	f.bar.SetFileName(event.FileName)
	f.bar.SetState(pb.ProgressStateDownloading)
	f.previousBytes = 0
}

func (f *CLIFormatter) OnProgress(event *xdcc.TransferProgessEvent, totalBytes uint64) {
	// TransferBytes is cumulative, so calculate the increment
	increment := event.TransferBytes - f.previousBytes
	f.bar.Increment(int(increment))
	f.previousBytes = event.TransferBytes
}

func (f *CLIFormatter) OnCompleted(event *xdcc.TransferCompletedEvent) {
	f.bar.SetState(pb.ProgressStateCompleted)
}

func (f *CLIFormatter) OnError(event *xdcc.TransferErrorEvent) {
	// CLI formatter doesn't display non-fatal errors
}

func (f *CLIFormatter) OnAborted(event *xdcc.TransferAbortedEvent) {
	f.bar.SetState(pb.ProgressStateAborted)
}

func (f *CLIFormatter) OnRetry(event *xdcc.TransferRetryEvent) {
	// CLI formatter doesn't display retry events
}

