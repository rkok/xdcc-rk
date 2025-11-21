# JSONL Output Format Proposal for `xdcc get`

## Overview

This document specifies a JSON Lines (JSONL) streaming output format for the `xdcc get` command. The format emits one JSON object per line, allowing a web frontend to parse each line as it arrives and update the UI in real-time. Each line represents a discrete event in the download lifecycle.

## Event Types and Schema

### 1. Connection Event
Emitted when connecting to IRC network.

```json
{"type":"connecting","url":"irc://irc.rizon.net/#news/XDCC|Bot/42","network":"irc.rizon.net","channel":"#news","bot":"XDCC|Bot","slot":42,"ssl":true,"timestamp":"2025-11-21T10:30:00Z"}
```

### 2. Connected Event
Emitted when successfully connected to IRC and joined channel.

```json
{"type":"connected","url":"irc://irc.rizon.net/#news/XDCC|Bot/42","timestamp":"2025-11-21T10:30:01Z"}
```

### 3. Transfer Started Event
Emitted when file transfer begins (corresponds to `TransferStartedEvent`).

```json
{"type":"started","url":"irc://irc.rizon.net/#news/XDCC|Bot/42","fileName":"ubuntu-22.04.iso","fileSize":3221225472,"filePath":"/downloads/ubuntu-22.04.iso","timestamp":"2025-11-21T10:30:02Z"}
```

### 4. Progress Event
Emitted periodically during download (corresponds to `TransferProgessEvent`).

```json
{"type":"progress","url":"irc://irc.rizon.net/#news/XDCC|Bot/42","fileName":"ubuntu-22.04.iso","bytesTransferred":104857600,"totalBytes":3221225472,"percentage":3.25,"transferRate":10485760,"timestamp":"2025-11-21T10:30:12Z"}
```

**Fields:**
- `bytesTransferred`: Cumulative bytes downloaded so far
- `totalBytes`: Total file size
- `percentage`: Progress percentage (0-100)
- `transferRate`: Current transfer rate in bytes/second

### 5. Completed Event
Emitted when download completes successfully (corresponds to `TransferCompletedEvent`).

```json
{"type":"completed","url":"irc://irc.rizon.net/#news/XDCC|Bot/42","fileName":"ubuntu-22.04.iso","fileSize":3221225472,"filePath":"/downloads/ubuntu-22.04.iso","duration":307.5,"avgRate":10475520,"timestamp":"2025-11-21T10:35:09Z"}
```

**Fields:**
- `duration`: Total download time in seconds
- `avgRate`: Average transfer rate in bytes/second

### 6. Error Event
Emitted when an error occurs at any stage.

```json
{"type":"error","url":"irc://irc.rizon.net/#news/XDCC|Bot/42","error":"connection timeout","errorType":"network","fatal":true,"timestamp":"2025-11-21T10:30:07Z"}
```

**Fields:**
- `error`: Human-readable error message (concise)
- `errorType`: Category of error (`network`, `irc`, `file`, `parse`, `ssl`, `unknown`)
- `fatal`: Whether this error terminates the transfer

### 7. Aborted Event
Emitted when transfer is aborted (corresponds to `TransferAbortedEvent`).

```json
{"type":"aborted","url":"irc://irc.rizon.net/#news/XDCC|Bot/42","reason":"max connection attempts exceeded","timestamp":"2025-11-21T10:30:27Z"}
```

### 8. Retry Event
Emitted when retrying connection (useful for showing retry attempts).

```json
{"type":"retry","url":"irc://irc.rizon.net/#news/XDCC|Bot/42","attempt":2,"maxAttempts":5,"reason":"disconnected","timestamp":"2025-11-21T10:30:05Z"}
```

### 9. Process Finished Event
Emitted once at the very end when all transfers are complete (for multi-file downloads).

```json
{"type":"finished","totalTransfers":3,"successful":2,"failed":1,"timestamp":"2025-11-21T10:40:00Z"}
```

## Error Handling Strategy

### Concise Error Messages
- Keep error messages short and actionable
- Use `errorType` field for categorization
- Examples:
  - `"connection timeout"` instead of full stack trace
  - `"invalid IRC URL"` instead of detailed parsing error
  - `"SSL certificate error"` with suggestion in separate field

### Error Types
- `network`: Connection/network issues
- `irc`: IRC protocol errors
- `file`: File I/O errors
- `parse`: URL/response parsing errors
- `ssl`: SSL/TLS certificate issues
- `unknown`: Uncategorized errors

## Multi-File Download Support

When downloading multiple files, each file gets its own stream of events identified by the `url` field. The web frontend can track multiple downloads simultaneously.

Example sequence for 2 files:
```json
{"type":"connecting","url":"irc://server1/channel/bot/1",...}
{"type":"connecting","url":"irc://server2/channel/bot/5",...}
{"type":"connected","url":"irc://server1/channel/bot/1",...}
{"type":"started","url":"irc://server1/channel/bot/1","fileName":"file1.zip",...}
{"type":"connected","url":"irc://server2/channel/bot/5",...}
{"type":"progress","url":"irc://server1/channel/bot/1",...}
{"type":"started","url":"irc://server2/channel/bot/5","fileName":"file2.zip",...}
{"type":"progress","url":"irc://server2/channel/bot/5",...}
{"type":"completed","url":"irc://server1/channel/bot/1",...}
{"type":"completed","url":"irc://server2/channel/bot/5",...}
{"type":"finished","totalTransfers":2,"successful":2,"failed":0,...}
```

## Usage

Add a `--format=jsonl` flag to the `get` command:

```bash
xdcc get "irc://server/channel/bot/42" --format=jsonl
```

## Benefits for Web Frontend

1. **Streaming**: Parse line-by-line as data arrives
2. **Real-time Updates**: Update UI immediately on each event
3. **Multi-file Tracking**: Track multiple downloads using `url` as identifier
4. **Error Handling**: Clear error types and messages for user feedback
5. **Progress Bars**: `progress` events provide all data needed for progress bars
6. **Completion Detection**: `finished` event signals when to stop listening
7. **Parseable**: Standard JSON format, easy to parse in JavaScript/any language

## Implementation Considerations

1. **Timestamps**: ISO 8601 format for easy parsing
2. **Buffering**: Flush after each line to ensure immediate delivery
3. **Stderr**: Keep error output separate or include in JSONL stream
4. **Backward Compatibility**: Keep existing progress bar output as default
5. **File Paths**: Include both relative and absolute paths where relevant
6. **Transfer Rate**: Bytes per second (can be formatted in frontend)
7. **Percentage**: Pre-calculated for convenience (0-100 range)

## Event Mapping to Current Code

### Current Transfer Events (xdcc/xdcc.go)
- `TransferStartedEvent` → `started` event
- `TransferProgessEvent` → `progress` event
- `TransferCompletedEvent` → `completed` event
- `TransferAbortedEvent` → `aborted` event

### IRC Connection Events (from setupHandlers)
- `irc.CONNECTED` → `connected` event
- `irc.DISCONNECTED` → `retry` event (if retrying) or `error` event (if fatal)
- `irc.JOIN` → Part of connection flow
- `irc.ERROR` → `error` event

### Additional Events to Add
- Connection attempt → `connecting` event
- Retry logic → `retry` event
- Overall completion → `finished` event

## Data Available from Current Implementation

From `TransferStartedEvent`:
- `FileName` (string)
- `FileSize` (uint64)

From `TransferProgessEvent`:
- `TransferBytes` (uint64) - cumulative bytes transferred
- `TransferRate` (float32) - bytes per second

From `TransferAbortedEvent`:
- `Error` (string)

From `IRCFile` (url.go):
- `Network` (string)
- `Channel` (string)
- `UserName` (bot name, string)
- `Slot` (int)

From `Config`:
- `OutPath` (string)
- `SSLOnly` (bool)

