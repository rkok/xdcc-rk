package output

import "xdcc-cli/xdcc"

// TransferOutputFormatter defines the interface for formatting transfer events
// Different implementations can provide CLI, JSONL, or other output formats
type TransferOutputFormatter interface {
	// OnConnecting is called when the transfer starts connecting to IRC
	OnConnecting(event *xdcc.TransferConnectingEvent)

	// OnConnected is called when successfully connected to IRC
	OnConnected(event *xdcc.TransferConnectedEvent)

	// OnStarted is called when the file transfer begins
	OnStarted(event *xdcc.TransferStartedEvent)

	// OnProgress is called periodically during file transfer
	// totalBytes is passed separately as it may not be in the event
	OnProgress(event *xdcc.TransferProgessEvent, totalBytes uint64)

	// OnCompleted is called when the transfer finishes successfully
	OnCompleted(event *xdcc.TransferCompletedEvent)

	// OnError is called when a non-fatal error occurs
	OnError(event *xdcc.TransferErrorEvent)

	// OnAborted is called when the transfer is aborted
	OnAborted(event *xdcc.TransferAbortedEvent)

	// OnRetry is called when the transfer is retrying after a failure
	OnRetry(event *xdcc.TransferRetryEvent)
}

