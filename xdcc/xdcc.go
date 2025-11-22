package xdcc

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
	"xdcc-cli/proxy"

	irc "github.com/fluffle/goirc/client"
)

const IRCClientUserName = "xdcc-cli"

type CTCPRequest interface {
	String() string
}

type CTCPResponse interface {
	Parse(args []string) error
	Name() string
}

type XdccSendReq struct {
	Slot int
}

func (send *XdccSendReq) String() string {
	return fmt.Sprintf("xdcc send #%d", send.Slot)
}

type XdccSendRes struct {
	FileName string
	IP       net.IP
	Port     int
	FileSize int
}

func uint32ToIP(n int) net.IP {
	a := byte((n >> 24) & 255)
	b := byte((n >> 16) & 255)
	c := byte((n >> 8) & 255)
	d := byte(n & 255)
	return net.IPv4(a, b, c, d)
}

const XdccSendResArgs = 4

func (send *XdccSendRes) Name() string {
	return SEND
}

func (send *XdccSendRes) Parse(args []string) error {
	if len(args) != XdccSendResArgs {
		return errors.New("invalid number of arguments")
	}

	send.FileName = args[0]

	ipUint32, err := strconv.Atoi(args[1])

	if err != nil {
		return err
	}

	send.IP = uint32ToIP(ipUint32)
	send.Port, err = strconv.Atoi(args[2])

	if err != nil {
		return err
	}

	send.FileSize, err = strconv.Atoi(args[3])

	if err != nil {
		return err
	}
	return nil
}

const (
	SEND    = "SEND"
	VERSION = "\x01VERSION\x01"
)

func parseCTCPRes(text string) (CTCPResponse, error) {
	fields := strings.Fields(text)

	var resp CTCPResponse = nil

	switch strings.TrimSpace(fields[0]) {
	case SEND:
		resp = &XdccSendRes{}
	case VERSION:
		return nil, nil
	}

	if resp == nil {
		return nil, errors.New("invalid command: " + fields[0])
	}

	err := resp.Parse(fields[1:])
	if err != nil {
		return nil, err
	}
	return resp, nil
}

const defaultEventChanSize = 1024

func (transfer *XdccTransfer) emitConnectingEvent() {
	transfer.notifyEvent(&TransferConnectingEvent{
		URL:     transfer.url.String(),
		Network: transfer.url.Network,
		Channel: transfer.url.Channel,
		Bot:     transfer.url.UserName,
		Slot:    transfer.url.Slot,
		SSL:     transfer.sslEnabled,
	})
}

func (transfer *XdccTransfer) Start() error {
	transfer.emitConnectingEvent()
	transfer.startTime = time.Now()
	return transfer.conn.Connect()
}

type TransferEvent interface{}

type TransferConnectingEvent struct {
	URL     string
	Network string
	Channel string
	Bot     string
	Slot    int
	SSL     bool
}

type TransferConnectedEvent struct {
	URL string
}

type TransferErrorEvent struct {
	URL       string
	Error     string
	ErrorType string
	Fatal     bool
}

type TransferRetryEvent struct {
	URL         string
	Attempt     int
	MaxAttempts int
	Reason      string
}

type TransferAbortedEvent struct {
	Error string
}

const maxConnAttempts = 5

type Transfer interface {
	Start() error
	PollEvents() chan TransferEvent
}

type retryTransfer struct {
	*XdccTransfer
	conf Config
}

func (t *retryTransfer) Start() error {
	// t.XdccTransfer is already initialized in NewTransfer
	if err := t.XdccTransfer.Start(); err == nil {
		return nil
	}

	t2 := newXdccTransfer(t.conf, true, true)
	// Reuse event channel from first transfer
	t2.events = t.XdccTransfer.events
	t.XdccTransfer = t2
	if err := t2.Start(); err == nil {
		return nil
	}

	t3 := newXdccTransfer(t.conf, false, false)
	// Reuse event channel
	t3.events = t2.events
	t.XdccTransfer = t3
	return t3.Start()
}

func (t *retryTransfer) PollEvents() chan TransferEvent {
	return t.XdccTransfer.PollEvents()
}

type XdccTransfer struct {
	filePath          string
	url               IRCFile
	conn              *irc.Conn
	connAttempts      int
	started           bool
	events            chan TransferEvent
	sslEnabled        bool
	startTime         time.Time
	sanitizeFilenames bool
}

type Config struct {
	File              IRCFile
	OutPath           string
	SSLOnly           bool
	SanitizeFilenames bool
}

func NewTransfer(c Config) Transfer {
	if c.SSLOnly {
		return newXdccTransfer(c, true, false)
	}

	// Initialize with first transfer so events can be polled
	return &retryTransfer{
		XdccTransfer: newXdccTransfer(c, true, false),
		conf:         c,
	}
}

func newXdccTransfer(c Config, enableSSL bool, skipCertificateCheck bool) *XdccTransfer {
	rand.Seed(time.Now().UTC().UnixNano())
	nick := IRCClientUserName + strconv.Itoa(int(rand.Uint32()))

	file := c.File

	config := irc.NewConfig(nick)
	config.SSL = enableSSL
	config.SSLConfig = &tls.Config{ServerName: file.Network, InsecureSkipVerify: skipCertificateCheck}
	config.Server = file.Network
	config.NewNick = func(nick string) string {
		return nick + "" + strconv.Itoa(int(rand.Uint32()))
	}
	// Set proxy if configured
	config.Proxy = proxy.ProxyURL()

	conn := irc.Client(config)

	t := &XdccTransfer{
		conn:              conn,
		url:               file,
		filePath:          c.OutPath,
		started:           false,
		connAttempts:      0,
		events:            make(chan TransferEvent, defaultEventChanSize),
		sslEnabled:        enableSSL,
		sanitizeFilenames: c.SanitizeFilenames,
	}
	t.setupHandlers(file.Channel, file.UserName, file.Slot)

	return t
}

func (transfer *XdccTransfer) send(req CTCPRequest) {
	transfer.conn.Privmsg(transfer.url.UserName, req.String())
}

func (transfer *XdccTransfer) setupHandlers(channel string, userName string, slot int) {
	conn := transfer.conn

	// e.g. join channel on connect.
	conn.HandleFunc(irc.CONNECTED,
		func(conn *irc.Conn, line *irc.Line) {
			transfer.connAttempts = 0
			transfer.notifyEvent(&TransferConnectedEvent{
				URL: transfer.url.String(),
			})
			conn.Join(channel)
		})

	conn.HandleFunc(irc.ERROR, func(conn *irc.Conn, line *irc.Line) {
		transfer.notifyEvent(&TransferErrorEvent{
			URL:       transfer.url.String(),
			Error:     line.Text(),
			ErrorType: "irc",
			Fatal:     false,
		})
	})

	// send xdcc send on successfull join
	conn.HandleFunc(irc.JOIN,
		func(conn *irc.Conn, line *irc.Line) {
			if strings.EqualFold(line.Args[0], channel) && !transfer.started {
				transfer.send(&XdccSendReq{Slot: slot})
			}
		})

	conn.HandleFunc(irc.PRIVMSG, func(conn *irc.Conn, line *irc.Line) {})

	conn.HandleFunc(irc.CTCP,
		func(conn *irc.Conn, line *irc.Line) {
			res, err := parseCTCPRes(line.Text())
			if err != nil {
				fmt.Println(err.Error())
				os.Exit(1) // TODO: correct clean up
			}
			transfer.handleCTCPRes(res)
		})

	conn.HandleFunc(irc.DISCONNECTED,
		func(conn *irc.Conn, line *irc.Line) {
			var err error = nil

			if transfer.connAttempts < maxConnAttempts {
				transfer.notifyEvent(&TransferRetryEvent{
					URL:         transfer.url.String(),
					Attempt:     transfer.connAttempts + 1,
					MaxAttempts: maxConnAttempts,
					Reason:      "disconnected",
				})
				time.Sleep(time.Second)

				err = conn.Connect()
			}

			if (err != nil || transfer.connAttempts >= maxConnAttempts) && !transfer.started {
				errMsg := "max connection attempts exceeded"
				if err != nil {
					errMsg = err.Error()
				}
				transfer.notifyEvent(&TransferAbortedEvent{Error: errMsg})
			}

			transfer.connAttempts++
		})
}

func (transfer *XdccTransfer) PollEvents() chan TransferEvent {
	return transfer.events
}

type TransferProgessEvent struct {
	TransferBytes uint64
	TransferRate  float32
}

const downloadBufSize = 1024

type TransferStartedEvent struct {
	FileName string
	FileSize uint64
	FilePath string
}

type TransferCompletedEvent struct {
	FileName string
	FileSize uint64
	FilePath string
	Duration float64
	AvgRate  float64
}

func (transfer *XdccTransfer) notifyEvent(e TransferEvent) {
	select {
	case transfer.events <- e:
	default:
		break
	}
}

type SpeedMonitorReader struct {
	reader         io.Reader
	elapsedTime    time.Duration
	currValue      uint64
	totalBytesRead uint64
	currentSpeed   float64
	onUpdate       func(cumulativeBytes int, speed float64)
}

func NewSpeedMonitorReader(reader io.Reader, onUpdate func(int, float64)) *SpeedMonitorReader {
	return &SpeedMonitorReader{
		reader:         reader,
		elapsedTime:    time.Duration(0),
		currValue:      0,
		totalBytesRead: 0,
		currentSpeed:   0,
		onUpdate:       onUpdate,
	}
}

func (monitor *SpeedMonitorReader) Read(buf []byte) (int, error) {
	now := time.Now()
	n, err := monitor.reader.Read(buf)
	elapsedTime := time.Since(now)
	monitor.currValue += uint64(n)
	monitor.totalBytesRead += uint64(n)
	monitor.elapsedTime += elapsedTime

	if monitor.elapsedTime > time.Second {
		monitor.currentSpeed = float64(monitor.currValue) / monitor.elapsedTime.Seconds()
		monitor.onUpdate(int(monitor.totalBytesRead), monitor.currentSpeed)
		monitor.currValue = 0
		monitor.elapsedTime = time.Duration(0)
	}
	return n, err
}

func (transfer *XdccTransfer) handleXdccSendRes(send *XdccSendRes) {
	go func() {
		// Use proxy-aware dialer for file transfer
		address := fmt.Sprintf("%s:%d", send.IP.String(), send.Port)
		conn, err := proxy.DialContext(context.Background(), "tcp", address)
		if err != nil {
			log.Fatalf("unable to reach host %s:%d", send.IP.String(), send.Port)
			return
		}

		filename := send.FileName
		if transfer.sanitizeFilenames {
			filename = SanitizeFilename(filename)
		}

		filePath := transfer.filePath + "/" + filename
		file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		fileWriter := bufio.NewWriter(file)

		if err != nil {
			log.Fatal(err.Error())
			return
		}

		downloadStartTime := time.Now()
		transfer.notifyEvent(&TransferStartedEvent{
			FileName: filename,
			FileSize: uint64(send.FileSize),
			FilePath: filePath,
		})
		transfer.started = true

		reader := NewSpeedMonitorReader(conn, func(dowloadedAmount int, speed float64) {
			transfer.notifyEvent(&TransferProgessEvent{
				TransferRate:  float32(speed),
				TransferBytes: uint64(dowloadedAmount),
			})
		})

		// download loop
		downloadedBytesTotal := 0
		buf := make([]byte, downloadBufSize)
		for downloadedBytesTotal < send.FileSize {
			n, err := reader.Read(buf)

			if err != nil {
				log.Fatal(err.Error())
				return
			}

			if _, err := fileWriter.Write(buf[:n]); err != nil {
				log.Fatal(err.Error())
				return
			}

			downloadedBytesTotal += n
		}
		fileWriter.Flush()

		duration := time.Since(downloadStartTime).Seconds()
		avgRate := float64(send.FileSize) / duration
		transfer.notifyEvent(&TransferCompletedEvent{
			FileName: filename,
			FileSize: uint64(send.FileSize),
			FilePath: filePath,
			Duration: duration,
			AvgRate:  avgRate,
		})
	}()
}

func (transfer *XdccTransfer) handleCTCPRes(resp CTCPResponse) {
	switch r := resp.(type) {
	case *XdccSendRes:
		transfer.handleXdccSendRes(r)
	}
}
