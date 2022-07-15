package profiler

import (
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

	"github.com/goccy/echo-tools/gist"
	"github.com/goccy/echo-tools/notifier"
	"github.com/labstack/echo/v4"
)

const (
	nginxAccessLog    = "/var/log/nginx/access.log"
	accessLogEndpoint = "/debug/accessLog"
)

type AccessLogProfiler struct {
	echo              *echo.Echo
	hostAddr          string
	accessLogFileName string
	botName           string
	githubToken       string
	discordWebhookURL string
}

type AccessLogProfilerOption func(*AccessLogProfiler)

func AccessLogDiscordNotifierOption(botName, webhookURL, githubToken string) AccessLogProfilerOption {
	return func(p *AccessLogProfiler) {
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
	cmdMv := exec.Command(
		"sh", "-c", fmt.Sprintf("sudo mv %s %s.`date +%%Y%%m%%d-%%H%%M%%S`", nginxAccessLog, nginxAccessLog),
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
	FileName          string `json:"filename"`
	BotName           string `json:"botName"`
	GitHubToken       string `json:"githubToken"`
	DiscordWebhookURL string `json:"discordWebhookURL"`
}

func (p *AccessLogProfiler) requestURL() string {
	addr := strings.TrimLeft(p.hostAddr, "http://")
	return fmt.Sprintf("http://%s%s", addr, accessLogEndpoint)
}

func (p *AccessLogProfiler) Stop() error {
	b, err := json.Marshal(&AccessLogRequest{
		FileName:          p.accessLogFileName,
		BotName:           p.botName,
		GitHubToken:       p.githubToken,
		DiscordWebhookURL: p.discordWebhookURL,
	})
	if err != nil {
		return fmt.Errorf("failed to encode access log: %w", err)
	}
	url := p.requestURL()
	fmt.Printf("exec: %s\n", url)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(b))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Content-Type", "application/json")
	httpClient := new(http.Client)
	if _, err := httpClient.Do(req); err != nil {
		return fmt.Errorf("failed to post %s: %w", url, err)
	}
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
	kataribeCommandTmpl = `cat %s | kataribe > %s`
)

func (h *AccessLogHandler) handle(ctx context.Context, body io.Reader) error {
	var req AccessLogRequest
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		return fmt.Errorf("failed to decode access log request: %w", err)
	}
	if req.FileName == "" {
		return fmt.Errorf("failed to find access-log filename")
	}
	tempDir := os.TempDir()
	kataribeFile := filepath.Join(tempDir, "kataribe.log")
	cmd := exec.Command(
		"sh", "-c", fmt.Sprintf(kataribeCommandTmpl, req.FileName, kataribeFile),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to exec kataribe: %s: %w", string(out), err)
	}
	if req.GitHubToken == "" || req.DiscordWebhookURL == "" {
		log.Println("github token or discord webhook url is not found")
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
