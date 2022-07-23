package profiler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/goccy/echo-tools/alp"
	"github.com/goccy/echo-tools/gist"
	"github.com/goccy/echo-tools/notifier"
	"github.com/labstack/echo/v4"
)

const (
	nginxAccessLog      = "/var/log/nginx/access.log"
	accessLogEndpoint   = "/debug/accessLog"
	accessLogFileFormat = "2006_01_02_15_04_05"
)

var (
	kataribeAccessLog string
	kataribeLogFile   string
	alpAccessLog      string
	alpLogFile        string
)

type AccessLogProfiler struct {
	echo              *echo.Echo
	hostAddr          string
	kataribeConfPath  string
	alpOption         string
	accessLogFileName string
	botName           string
	githubToken       string
	discordWebhookURL string
}

type AccessLogProfilerOption func(*AccessLogProfiler)

func AccessLogOption(kataribeFile, alpOption, botName, webhookURL, githubToken string) AccessLogProfilerOption {
	return func(p *AccessLogProfiler) {
		p.kataribeConfPath = kataribeFile
		p.alpOption = alpOption
		p.botName = botName
		p.discordWebhookURL = webhookURL
		p.githubToken = githubToken
	}
}

func NewAccessLogProfiler(e *echo.Echo, hostAddr string, opts ...AccessLogProfilerOption) *AccessLogProfiler {
	p := &AccessLogProfiler{
		echo:              e,
		hostAddr:          hostAddr,
		accessLogFileName: nginxAccessLog,
	}
	e.POST(accessLogEndpoint, echo.WrapHandler(&AccessLogHandler{}))
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *AccessLogProfiler) Start() error {
	log.Print("[benchmark-access-log-profiler] Start")

	now := time.Now().Format(accessLogFileFormat)
	alpAccessLog = nginxAccessLog
	alpLogFile = fmt.Sprintf("%s.%s", "alp.log", now)
	kataribeAccessLog = fmt.Sprintf("%s.%s.%s", nginxAccessLog, "kataribe", now)
	kataribeLogFile = fmt.Sprintf("%s.%s", "katarib.log", now)

	cmdMv := exec.Command(
		"sh", "-c", fmt.Sprintf("sudo mv %s %s", nginxAccessLog, alpAccessLog),
	)
	mvOut, err := cmdMv.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to mv access.log: %s: %w", string(mvOut), err)
	}

	cmdRot := exec.Command(
		"sh", "-c", "sudo nginx -s reopen",
	)
	rotOut, err := cmdRot.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to nginx reopen: %s: %w", string(rotOut), err)
	}
	return nil
}

type AccessLogRequest struct {
	FileName          string   `json:"filename"`
	KataribeConfPath  string   `json:"kataribeConfPath"`
	ALPOption         string   `json:"alpOption"`
	Routes            []string `json:"routes"`
	BotName           string   `json:"botName"`
	GitHubToken       string   `json:"githubToken"`
	DiscordWebhookURL string   `json:"discordWebhookURL"`
}

func (p *AccessLogProfiler) requestURL() string {
	addr := strings.TrimLeft(p.hostAddr, "http://")
	return fmt.Sprintf("http://%s%s", addr, accessLogEndpoint)
}

func (p *AccessLogProfiler) Stop() error {
	routes := make([]string, 0, len(p.echo.Routes()))
	for _, r := range p.echo.Routes() {
		routes = append(routes, r.Path)
	}
	b, err := json.Marshal(&AccessLogRequest{
		FileName:          p.accessLogFileName,
		KataribeConfPath:  p.kataribeConfPath,
		ALPOption:         p.alpOption,
		Routes:            routes,
		BotName:           p.botName,
		GitHubToken:       p.githubToken,
		DiscordWebhookURL: p.discordWebhookURL,
	})
	if err != nil {
		return fmt.Errorf("failed to encode access log: %w", err)
	}
	url := p.requestURL()
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(b))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Content-Type", "application/json")
	httpClient := new(http.Client)
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post %s: %w", url, err)
	}
	buf, _ := io.ReadAll(resp.Body)
	fmt.Printf("result: %s\n", string(buf))
	return nil
}

type AccessLogHandler struct{}

func (h *AccessLogHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h.handle(r.Context(), r.Body); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err.Error())
	}
}

var (
	alpCommandTmpl      = `sudo alp ltsv --file %s -r -m "%s" %s > %s`
	kataribeCommandTmpl = `sudo cat %s | kataribe -conf %s > %s`
)

func (h *AccessLogHandler) handle(ctx context.Context, body io.Reader) error {
	var req AccessLogRequest
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		return fmt.Errorf("failed to decode access log request: %w", err)
	}

	if req.FileName == "" {
		return fmt.Errorf("failed to find access-log filename")
	}

	if err := execALP(ctx, req); err != nil {
		return err
	}

	ltsvToWithTime(alpAccessLog, kataribeAccessLog)

	return execKataribe(ctx, AccessLogRequest{
		FileName:          kataribeAccessLog,
		KataribeConfPath:  req.KataribeConfPath,
		ALPOption:         req.ALPOption,
		Routes:            req.Routes,
		BotName:           req.BotName,
		GitHubToken:       req.GitHubToken,
		DiscordWebhookURL: req.DiscordWebhookURL,
	})
}

/*
log_format with_time '$remote_addr - $remote_user [$time_local] '
            '"$request" $status $body_bytes_sent '
            '"$http_referer" "$http_user_agent" $request_time';

log_format ltsv "time:$time_local"
            "\thost:$remote_addr"
            "\treq:$request"
            "\tstatus:$status"
            "\tmethod:$request_method"
            "\turi:$request_uri"
            "\tsize:$body_bytes_sent"
            "\treferer:$http_referer"
            "\tua:$http_user_agent"
            "\treqtime:$request_time"
            "\tcache:$upstream_http_x_cache"
            "\truntime:$upstream_http_x_runtime"
            "\tapptime:$upstream_response_time";
*/

type timeFormat struct {
	remoteAddr    string
	remoteUser    string
	timeLocal     string
	request       string
	status        string
	bodyBytesSent string
	httpReferer   string
	httpUserAgent string
	requestTime   string
}

func getValue(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return s[i+1:]
		}
	}
	return s
}

func ltsv2Time(ltsv string) string {
	if ltsv == "" {
		return ""
	}
	strs := strings.Split(ltsv, "\t")
	tfmt := timeFormat{
		remoteAddr:    getValue(strs[1]),
		remoteUser:    "-",
		timeLocal:     getValue(strs[0]),
		request:       getValue(strs[2]),
		status:        getValue(strs[3]),
		bodyBytesSent: getValue(strs[6]),
		httpReferer:   getValue(strs[7]),
		httpUserAgent: getValue(strs[8]),
		requestTime:   getValue(strs[9]),
	}
	return fmt.Sprintf("%s - %s [%s] \"%s\" %s %s \"%s\" \"%s\" %s",
		tfmt.remoteAddr,
		tfmt.remoteUser,
		tfmt.timeLocal,
		tfmt.request,
		tfmt.status,
		tfmt.bodyBytesSent,
		tfmt.httpReferer,
		tfmt.httpUserAgent,
		tfmt.requestTime,
	)
}

func ltsvToWithTime(inputPath, outputPath string) error {
	log.Printf("%s -> %s", inputPath, outputPath)
	rfp, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer rfp.Close()

	wfp, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer wfp.Close()

	i := 0
	reader := bufio.NewReaderSize(rfp, 4096)
	for {
		line, _, err := reader.ReadLine()
		withTime := ltsv2Time(string(line))
		wfp.Write([]byte(withTime + "\n"))

		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		i++
		if i%100 == 0 {
			wfp.Sync()
		}
	}
	return nil
}

func execKataribe(ctx context.Context, req AccessLogRequest) error {
	tempDir := os.TempDir()
	kataribeFile := filepath.Join(tempDir, kataribeLogFile)
	cmd := exec.Command(
		"sh", "-c", fmt.Sprintf(kataribeCommandTmpl, req.FileName, req.KataribeConfPath, kataribeFile),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to exec kataribe: %s: %w", string(out), err)
	}
	log.Print("[benchmark-access-log-profiler] send to gist")
	if req.GitHubToken == "" || req.DiscordWebhookURL == "" {
		return nil
	}
	client, err := gist.NewClient(ctx, req.GitHubToken)
	if err != nil {
		return fmt.Errorf("failed to create gist client: %w", err)
	}
	url, err := client.UploadFile(ctx, req.FileName, kataribeFile)
	if err != nil {
		return fmt.Errorf("failed to upload access-log: %w", err)
	}
	botName := req.BotName
	if botName == "" {
		botName = "bot"
	}
	discordClient := notifier.NewDiscordClient(req.DiscordWebhookURL)
	if err := discordClient.Post(&notifier.DiscordMessage{
		Username: botName,
		Content:  fmt.Sprintf("access-log kataribe: %s", url),
	}); err != nil {
		return fmt.Errorf("failed to post message to discord: %w", err)
	}
	return nil
}

func execALP(ctx context.Context, req AccessLogRequest) error {
	tempDir := os.TempDir()
	alpFile := filepath.Join(tempDir, alpLogFile)
	matchingGroups := alp.ConvertEchoRoutes(req.Routes)
	cmd := exec.Command(
		"sh", "-c", fmt.Sprintf(alpCommandTmpl, req.FileName, matchingGroups, req.ALPOption, alpFile),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to exec alp: %s: %w", string(out), err)
	}
	log.Print("[benchmark-access-log-profiler] send to gist")
	if req.GitHubToken == "" || req.DiscordWebhookURL == "" {
		return nil
	}
	client, err := gist.NewClient(ctx, req.GitHubToken)
	if err != nil {
		return fmt.Errorf("failed to create gist client: %w", err)
	}
	url, err := client.UploadFile(ctx, req.FileName, alpFile)
	if err != nil {
		return fmt.Errorf("failed to upload access-log: %w", err)
	}
	botName := req.BotName
	if botName == "" {
		botName = "bot"
	}
	discordClient := notifier.NewDiscordClient(req.DiscordWebhookURL)
	if err := discordClient.Post(&notifier.DiscordMessage{
		Username: botName,
		Content:  fmt.Sprintf("access-log alp: %s", url),
	}); err != nil {
		return fmt.Errorf("failed to post message to discord: %w", err)
	}
	return nil
}
