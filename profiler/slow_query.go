package profiler

import (
	"bytes"
	"context"
	"database/sql"
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

	"github.com/goccy/echo-tools/gist"
	"github.com/goccy/echo-tools/notifier"
	"github.com/labstack/echo/v4"
)

const (
	slowQueryLogFileFormat = "2006_01_02_15_04_05"
	slowQueryLogEndpoint   = "/debug/slowQueryLog"
)

type MySQLSlowQueryLogProfiler struct {
	echo                 *echo.Echo
	hostAddr             string
	db                   *sql.DB
	slowQueryLogFileName string
	botName              string
	discordWebhookURL    string
	githubToken          string
}

type MySQLSlowQueryLogProfilerOption func(*MySQLSlowQueryLogProfiler)

func MySQLSlowQueryLogDiscordNotifierOption(botName, webhookURL, githubToken string) MySQLSlowQueryLogProfilerOption {
	return func(p *MySQLSlowQueryLogProfiler) {
		p.botName = botName
		p.discordWebhookURL = webhookURL
		p.githubToken = githubToken
	}
}

func NewMySQLSlowQueryLogProfiler(e *echo.Echo, hostAddr string, db *sql.DB, opts ...MySQLSlowQueryLogProfilerOption) *MySQLSlowQueryLogProfiler {
	slowQueryLogFileName := fmt.Sprintf(
		"%s_slow_query.log",
		time.Now().Format(slowQueryLogFileFormat),
	)
	p := &MySQLSlowQueryLogProfiler{
		echo:                 e,
		hostAddr:             hostAddr,
		db:                   db,
		slowQueryLogFileName: slowQueryLogFileName,
	}
	e.POST(slowQueryLogEndpoint, echo.WrapHandler(&MySQLSlowQueryLogHandler{}))
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *MySQLSlowQueryLogProfiler) Start() error {
	log.Printf("slow query log: %s", p.slowQueryLogFileName)
	if _, err := p.db.Exec(
		fmt.Sprintf(
			"SET GLOBAL slow_query_log_file = `%s`",
			p.slowQueryLogFileName,
		),
	); err != nil {
		return fmt.Errorf("failed to set slow query log file: %w", err)
	}
	if _, err := p.db.Exec("SET GLOBAL long_query_time = 0"); err != nil {
		return fmt.Errorf("failed to set long_query_time: %w", err)
	}
	return nil
}

func (p *MySQLSlowQueryLogProfiler) requestURL() string {
	addr := strings.TrimLeft(p.hostAddr, "http://")
	return fmt.Sprintf("http://%s%s", addr, slowQueryLogEndpoint)
}

type MySQLSlowQueryLogRequest struct {
	FileName          string `json:"filename"`
	BotName           string `json:"botName"`
	GitHubToken       string `json:"githubToken"`
	DiscordWebhookURL string `json:"discordWebhookURL"`
}

func (p *MySQLSlowQueryLogProfiler) Stop() error {
	b, err := json.Marshal(&MySQLSlowQueryLogRequest{
		FileName:          p.slowQueryLogFileName,
		BotName:           p.botName,
		GitHubToken:       p.githubToken,
		DiscordWebhookURL: p.discordWebhookURL,
	})
	if err != nil {
		return fmt.Errorf("failed to encode slow query log: %w", err)
	}
	url := p.requestURL()
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

type MySQLSlowQueryLogHandler struct{}

func (h *MySQLSlowQueryLogHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h.handle(r.Context(), r.Body); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err.Error())
	}
}

var (
	queryDigestCommandTmpl = `sudo pt-query-digest /var/lib/mysql/%s > %s`
)

func (h *MySQLSlowQueryLogHandler) handle(ctx context.Context, body io.Reader) error {
	var req MySQLSlowQueryLogRequest
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		return fmt.Errorf("failed to decode slow query log request: %w", err)
	}
	if req.FileName == "" {
		return fmt.Errorf("failed to find slow-query-log filename")
	}
	tempDir := os.TempDir()
	digestFile := filepath.Join(tempDir, fmt.Sprintf("digest_%s", req.FileName))
	cmd := exec.Command(
		"sh", "-c", fmt.Sprintf(queryDigestCommandTmpl, req.FileName, digestFile),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to exec pt-query-digest: %s: %w", string(out), err)
	}
	if req.GitHubToken == "" || req.DiscordWebhookURL == "" {
		log.Println("github token or discord webhook url is not found")
		return nil
	}
	client, err := gist.NewClient(ctx, req.GitHubToken)
	if err != nil {
		return fmt.Errorf("failed to create gist client: %w", err)
	}
	url, err := client.UploadFile(ctx, req.FileName, digestFile)
	if err != nil {
		return fmt.Errorf("failed to upload slow-query-log: %w", err)
	}
	botName := req.BotName
	if botName == "" {
		botName = "bot"
	}
	discordClient := notifier.NewDiscordClient(req.DiscordWebhookURL)
	if err := discordClient.Post(&notifier.DiscordMessage{
		Username: botName,
		Content:  fmt.Sprintf("slow-query-log digest: %s", url),
	}); err != nil {
		return fmt.Errorf("failed to post message to discord: %w", err)
	}
	return nil
}
