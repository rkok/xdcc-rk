package main

import (
	"bufio"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"xdcc-cli/pb"
	"xdcc-cli/proxy"
	"xdcc-cli/search"
	table "xdcc-cli/table"
	xdcc "xdcc-cli/xdcc"
)

var searchEngine *search.ProviderAggregator

func init() {
	searchEngine = search.NewProviderAggregator(
		&search.XdccEuProvider{},
		&search.SunXdccProvider{},
	)
}

var defaultColWidths []int = []int{100, 10, -1}

func FloatToString(value float64) string {
	if value-float64(int64(value)) > 0 {
		return strconv.FormatFloat(value, 'f', 2, 32)
	}
	return strconv.FormatFloat(value, 'f', 0, 32)
}

func formatSize(size int64) string {
	if size < 0 {
		return "--"
	}

	if size >= search.GigaByte {
		return FloatToString(float64(size)/float64(search.GigaByte)) + "GB"
	} else if size >= search.MegaByte {
		return FloatToString(float64(size)/float64(search.MegaByte)) + "MB"
	} else if size >= search.KiloByte {
		return FloatToString(float64(size)/float64(search.KiloByte)) + "KB"
	}
	return FloatToString(float64(size)) + "B"
}

type JSONSearchResult struct {
	FileName string  `json:"fileName"`
	Size     float64 `json:"size"`
	URL      string  `json:"url"`
}

type JSONSearchOutput struct {
	Results []JSONSearchResult `json:"results"`
}

// JSONL event types for transfer output
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
	FileName          string  `json:"fileName,omitempty"`
	FileSize          uint64  `json:"fileSize,omitempty"`
	FilePath          string  `json:"filePath,omitempty"`
	BytesTransferred  uint64  `json:"bytesTransferred,omitempty"`
	TotalBytes        uint64  `json:"totalBytes,omitempty"`
	Percentage        float64 `json:"percentage,omitempty"`
	TransferRate      float64 `json:"transferRate,omitempty"`
	Duration          float64 `json:"duration,omitempty"`
	AvgRate           float64 `json:"avgRate,omitempty"`

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

func outputSearchResultsJSON(results []search.XdccFileInfo) {
	jsonResults := make([]JSONSearchResult, 0, len(results))
	for _, fileInfo := range results {
		sizeInKB := float64(fileInfo.Size) / float64(search.KiloByte)
		jsonResults = append(jsonResults, JSONSearchResult{
			FileName: fileInfo.Name,
			Size:     sizeInKB,
			URL:      fileInfo.URL.String(),
		})
	}

	output := JSONSearchOutput{Results: jsonResults}
	jsonBytes, err := json.Marshal(output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting JSON: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(jsonBytes))
}

func execSearch(args []string) {
	searchCmd := flag.NewFlagSet("search", flag.ExitOnError)
	sortByFilename := searchCmd.Bool("s", false, "sort results by filename")
	proxyURL := searchCmd.String("proxy", "", "SOCKS5 proxy URL (e.g., socks5://localhost:1080)")
	format := searchCmd.String("format", "table", "output format (table, json)")

	args = parseFlags(searchCmd, args)

	// Initialize proxy
	if err := proxy.Initialize(*proxyURL); err != nil {
		log.Fatalf("Failed to initialize proxy: %v\n", err)
	}

	if len(args) < 1 {
		fmt.Println("search: no keyword provided.")
		os.Exit(1)
	}

	res, _ := searchEngine.Search(args)

	// Handle output format
	if *format == "json" {
		outputSearchResultsJSON(res)
		return
	}

	// Table output (default)
	printer := table.NewTablePrinter([]string{"File Name", "Size", "URL"})
	printer.SetMaxWidths(defaultColWidths)

	for _, fileInfo := range res {
		printer.AddRow(table.Row{fileInfo.Name, formatSize(fileInfo.Size), fileInfo.URL.String()})
	}

	sortColumn := 2
	if *sortByFilename {
		sortColumn = 0
	}
	printer.SortByColumn(sortColumn)

	printer.Print()
}

func emitJSONLEvent(event JSONLEvent) {
	event.Timestamp = time.Now().UTC().Format(time.RFC3339)
	jsonBytes, err := json.Marshal(event)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting JSONL: %v\n", err)
		return
	}
	fmt.Println(string(jsonBytes))
	os.Stdout.Sync() // Flush immediately for streaming
}

func transferLoopJSONL(transfer xdcc.Transfer, urlStr string) bool {
	evts := transfer.PollEvents()
	quit := false
	var totalBytes uint64

	for !quit {
		e := <-evts
		switch evtType := e.(type) {
		case *xdcc.TransferConnectingEvent:
			emitJSONLEvent(JSONLEvent{
				Type:    "connecting",
				URL:     evtType.URL,
				Network: evtType.Network,
				Channel: evtType.Channel,
				Bot:     evtType.Bot,
				Slot:    evtType.Slot,
				SSL:     evtType.SSL,
			})

		case *xdcc.TransferConnectedEvent:
			emitJSONLEvent(JSONLEvent{
				Type: "connected",
				URL:  evtType.URL,
			})

		case *xdcc.TransferStartedEvent:
			totalBytes = evtType.FileSize
			emitJSONLEvent(JSONLEvent{
				Type:     "started",
				URL:      urlStr,
				FileName: evtType.FileName,
				FileSize: evtType.FileSize,
				FilePath: evtType.FilePath,
			})

		case *xdcc.TransferProgessEvent:
			percentage := 0.0
			if totalBytes > 0 {
				percentage = (float64(evtType.TransferBytes) / float64(totalBytes)) * 100.0
			}
			emitJSONLEvent(JSONLEvent{
				Type:             "progress",
				URL:              urlStr,
				BytesTransferred: evtType.TransferBytes,
				TotalBytes:       totalBytes,
				Percentage:       percentage,
				TransferRate:     float64(evtType.TransferRate),
			})

		case *xdcc.TransferCompletedEvent:
			emitJSONLEvent(JSONLEvent{
				Type:     "completed",
				URL:      urlStr,
				FileName: evtType.FileName,
				FileSize: evtType.FileSize,
				FilePath: evtType.FilePath,
				Duration: evtType.Duration,
				AvgRate:  evtType.AvgRate,
			})
			quit = true
			return true

		case *xdcc.TransferErrorEvent:
			emitJSONLEvent(JSONLEvent{
				Type:      "error",
				URL:       evtType.URL,
				Error:     evtType.Error,
				ErrorType: evtType.ErrorType,
				Fatal:     evtType.Fatal,
			})

		case *xdcc.TransferAbortedEvent:
			emitJSONLEvent(JSONLEvent{
				Type:   "aborted",
				URL:    urlStr,
				Reason: evtType.Error,
			})
			quit = true
			return false

		case *xdcc.TransferRetryEvent:
			emitJSONLEvent(JSONLEvent{
				Type:        "retry",
				URL:         evtType.URL,
				Attempt:     evtType.Attempt,
				MaxAttempts: evtType.MaxAttempts,
				Reason:      evtType.Reason,
			})
		}
	}
	return false
}

func transferLoop(transfer xdcc.Transfer, format string) {
	var bar pb.ProgressBar
	if format == "cli" {
		bar = pb.NewProgressBar()
	}

	evts := transfer.PollEvents()
	quit := false
	var previousBytes uint64 = 0
	for !quit {
		e := <-evts
		switch evtType := e.(type) {
		case *xdcc.TransferStartedEvent:
			if format == "cli" {
				bar.SetTotal(int(evtType.FileSize))
				bar.SetFileName(evtType.FileName)
				bar.SetState(pb.ProgressStateDownloading)
			}
		case *xdcc.TransferProgessEvent:
			if format == "cli" {
				// TransferBytes is now cumulative, so calculate the increment
				increment := evtType.TransferBytes - previousBytes
				bar.Increment(int(increment))
				previousBytes = evtType.TransferBytes
			}
		case *xdcc.TransferCompletedEvent:
			if format == "cli" {
				bar.SetState(pb.ProgressStateCompleted)
			}
			quit = true
		}
	}
	// TODO: do clean-up operations here
}

func suggestUnknownAuthoritySwitch(err error) {
	if err.Error() == (x509.UnknownAuthorityError{}.Error()) {
		fmt.Println("use the --allow-unknown-authority flag to skip certificate verification")
	}
}

func doTransfer(transfer xdcc.Transfer, format string, urlStr string) bool {
	if format == "jsonl" {
		// Start event loop in goroutine before calling Start()
		// so we can capture connecting event
		resultChan := make(chan bool, 1)
		errChan := make(chan error, 1)

		go func() {
			resultChan <- transferLoopJSONL(transfer, urlStr)
		}()

		go func() {
			errChan <- transfer.Start()
		}()

		err := <-errChan
		if err != nil {
			emitJSONLEvent(JSONLEvent{
				Type:      "error",
				URL:       urlStr,
				Error:     err.Error(),
				ErrorType: "network",
				Fatal:     true,
			})
			return false
		}

		return <-resultChan
	}

	err := transfer.Start()
	if err != nil {
		fmt.Println(err)
		suggestUnknownAuthoritySwitch(err)
		return false
	}

	transferLoop(transfer, format)
	return true
}

func parseFlags(flagSet *flag.FlagSet, args []string) []string {
	flagIdx := findFirstFlag(args)
	if flagIdx < 0 {
		return args
	}
	flagSet.Parse(args[flagIdx:])
	return args[:flagIdx]
}

func findFirstFlag(args []string) int {
	for i, arg := range args {
		if strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "--") {
			return i
		}
	}
	return -1
}

func loadUrlListFile(filePath string) []string {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	urlList := make([]string, 0)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		urlList = append(urlList, line)
	}

	if err := scanner.Err(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return urlList
}

func printGetUsageAndExit(flagSet *flag.FlagSet) {
	fmt.Printf("usage: get url1 url2 ... [-o path] [-i file] [--ssl-only] [--proxy url]\n\nFlag set:\n")
	flagSet.PrintDefaults()
	os.Exit(0)
}

func execGet(args []string) {
	getCmd := flag.NewFlagSet("get", flag.ExitOnError)
	path := getCmd.String("o", ".", "output folder of dowloaded file")
	inputFile := getCmd.String("i", "", "input file containing a list of urls")
	proxyURL := getCmd.String("proxy", "", "SOCKS5 proxy URL (e.g., socks5://localhost:1080)")
	format := getCmd.String("format", "cli", "output format (cli, jsonl)")

	sslOnly := getCmd.Bool("ssl-only", false, "force the client to use TSL connection")

	urlList := parseFlags(getCmd, args)

	// Initialize proxy
	if err := proxy.Initialize(*proxyURL); err != nil {
		log.Fatalf("Failed to initialize proxy: %v\n", err)
	}

	if *inputFile != "" {
		urlList = append(urlList, loadUrlListFile(*inputFile)...)
	}

	if len(urlList) == 0 {
		printGetUsageAndExit(getCmd)
	}

	// Track transfer results for JSONL finished event
	var resultsMutex sync.Mutex
	totalTransfers := 0
	successful := 0
	failed := 0

	wg := sync.WaitGroup{}
	for _, urlStr := range urlList {
		url, err := xdcc.ParseURL(urlStr)
		if errors.Is(err, xdcc.ErrInvalidURL) {
			if *format == "jsonl" {
				emitJSONLEvent(JSONLEvent{
					Type:      "error",
					URL:       urlStr,
					Error:     "invalid IRC URL",
					ErrorType: "parse",
					Fatal:     true,
				})
			} else {
				fmt.Printf("no valid irc url: %s\n", urlStr)
			}
			continue
		}

		if err != nil {
			if *format == "jsonl" {
				emitJSONLEvent(JSONLEvent{
					Type:      "error",
					URL:       urlStr,
					Error:     err.Error(),
					ErrorType: "parse",
					Fatal:     true,
				})
			} else {
				fmt.Println(err.Error())
			}
			os.Exit(1)
		}

		transfer := xdcc.NewTransfer(xdcc.Config{
			File:    *url,
			OutPath: *path,
			SSLOnly: *sslOnly,
		})

		totalTransfers++
		wg.Add(1)
		go func(transfer xdcc.Transfer, fmt string, urlStr string) {
			success := doTransfer(transfer, fmt, urlStr)
			resultsMutex.Lock()
			if success {
				successful++
			} else {
				failed++
			}
			resultsMutex.Unlock()
			wg.Done()
		}(transfer, *format, urlStr)
	}
	wg.Wait()

	// Emit finished event for JSONL format
	if *format == "jsonl" {
		emitJSONLEvent(JSONLEvent{
			Type:           "finished",
			TotalTransfers: totalTransfers,
			Successful:     successful,
			Failed:         failed,
		})
	}
}

func printUsage() {
	fmt.Println("Usage: xdcc <command> [arguments]")
	fmt.Println()
	fmt.Println("Available commands:")
	fmt.Println("  search    Search for files on IRC XDCC networks")
	fmt.Println("  get       Download files from IRC XDCC networks")
	fmt.Println()
	fmt.Println("Use 'xdcc <command> --help' for more information about a command.")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "--help", "-h", "help":
		printUsage()
		os.Exit(0)
	case "search":
		execSearch(os.Args[2:])
	case "get":
		execGet(os.Args[2:])
	default:
		fmt.Println("no such command: ", os.Args[1])
		fmt.Println()
		printUsage()
		os.Exit(1)
	}
}
