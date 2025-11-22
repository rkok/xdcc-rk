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
	"xdcc-cli/cmd/output"
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

// transferLoop runs the main event loop for a transfer using the provided formatter
func transferLoop(transfer xdcc.Transfer, formatter output.TransferOutputFormatter) bool {
	evts := transfer.PollEvents()
	var totalBytes uint64

	for {
		e := <-evts
		switch evt := e.(type) {
		case *xdcc.TransferConnectingEvent:
			formatter.OnConnecting(evt)

		case *xdcc.TransferConnectedEvent:
			formatter.OnConnected(evt)

		case *xdcc.TransferStartedEvent:
			totalBytes = evt.FileSize
			formatter.OnStarted(evt)

		case *xdcc.TransferProgessEvent:
			formatter.OnProgress(evt, totalBytes)

		case *xdcc.TransferCompletedEvent:
			formatter.OnCompleted(evt)
			return true

		case *xdcc.TransferErrorEvent:
			formatter.OnError(evt)

		case *xdcc.TransferAbortedEvent:
			formatter.OnAborted(evt)
			return false

		case *xdcc.TransferRetryEvent:
			formatter.OnRetry(evt)
		}
	}
}

func suggestUnknownAuthoritySwitch(err error) {
	if err.Error() == (x509.UnknownAuthorityError{}.Error()) {
		fmt.Println("use the --allow-unknown-authority flag to skip certificate verification")
	}
}

func doTransfer(transfer xdcc.Transfer, format string, urlStr string) bool {
	// Create the appropriate formatter based on format
	var formatter output.TransferOutputFormatter
	if format == "jsonl" {
		formatter = output.NewJSONLFormatter(urlStr)
	} else {
		formatter = output.NewCLIFormatter()
	}

	// For JSONL, start event loop in goroutine before calling Start()
	// so we can capture connecting event
	if format == "jsonl" {
		resultChan := make(chan bool, 1)
		errChan := make(chan error, 1)

		go func() {
			resultChan <- transferLoop(transfer, formatter)
		}()

		go func() {
			errChan <- transfer.Start()
		}()

		err := <-errChan
		if err != nil {
			// Emit error using JSONL formatter
			jsonlFormatter := formatter.(*output.JSONLFormatter)
			jsonlFormatter.OnError(&xdcc.TransferErrorEvent{
				URL:       urlStr,
				Error:     err.Error(),
				ErrorType: "network",
				Fatal:     true,
			})
			return false
		}

		return <-resultChan
	}

	// For CLI format, start transfer first
	err := transfer.Start()
	if err != nil {
		fmt.Println(err)
		suggestUnknownAuthoritySwitch(err)
		return false
	}

	return transferLoop(transfer, formatter)
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

// emitJSONLEvent is a helper to emit standalone JSONL events (for errors and finished events)
func emitJSONLEvent(event output.JSONLEvent) {
	formatter := output.NewJSONLFormatter("")
	formatter.EmitEvent(event)
}

func execGet(args []string) {
	getCmd := flag.NewFlagSet("get", flag.ExitOnError)
	path := getCmd.String("o", ".", "output folder of dowloaded file")
	inputFile := getCmd.String("i", "", "input file containing a list of urls")
	proxyURL := getCmd.String("proxy", "", "SOCKS5 proxy URL (e.g., socks5://localhost:1080)")
	format := getCmd.String("format", "cli", "output format (cli, jsonl)")
	sanitizeFilenames := getCmd.Bool("sanitize-filenames", false, "sanitize filenames to ASCII-only safe characters")

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
				emitJSONLEvent(output.JSONLEvent{
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
				emitJSONLEvent(output.JSONLEvent{
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
			File:              *url,
			OutPath:           *path,
			SSLOnly:           *sslOnly,
			SanitizeFilenames: *sanitizeFilenames,
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
		emitJSONLEvent(output.JSONLEvent{
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
