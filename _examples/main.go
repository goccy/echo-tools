package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	jsontools "github.com/goccy/echo-tools/json"
	middlewaretools "github.com/goccy/echo-tools/middleware"
	profilertools "github.com/goccy/echo-tools/profiler"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
)

type User struct {
	Name  string `json:"name"  query:"name"`
	Email string `json:"email" query:"email"`
}

type InitializeResponse struct{}

var (
	isDebug, _                   = strconv.ParseBool(os.Getenv("DEBUG"))
	benchmarkFinishEventNotifier = middlewaretools.NewBenchmarkFinishEventNotifier(
		onBenchmarkFinished,
		middlewaretools.BenchmarkFinishEventCheckInterval(10*time.Second),
	)
	profiler = profilertools.NewProfiler(os.TempDir())
)

func startBenchmark(c echo.Context) error {
	if isDebug {
		benchmarkFinishEventNotifier.Start()
		if err := profiler.Start(); err != nil {
			log.Print(err)
			return err
		}
	}
	return c.JSON(http.StatusOK, InitializeResponse{})
}

func onBenchmarkFinished() {
	fmt.Println("benchmark finished")
	profiler.Stop()
}

var db *sql.DB

func main() {
	e := echo.New()
	e.JSONSerializer = jsontools.NewSerializer()
	e.POST("/initialize", startBenchmark)
	e.POST("/users", func(c echo.Context) error {
		u := new(User)
		if err := c.Bind(u); err != nil {
			return err
		}
		return c.JSON(http.StatusCreated, u)
	})
	e.Use(middleware.Recover())
	e.Use(benchmarkFinishEventNotifier.Middleware())

	if isDebug {
		slowLogProfiler := profilertools.NewMySQLSlowQueryLogProfiler(
			e,
			fmt.Sprintf(
				"%s:%s",
				os.Getenv("MYSQL_HOST"),
				os.Getenv("HTTP_PORT"),
			),
			db,
			profilertools.MySQLSlowQueryLogDiscordNotifierOption(
				os.Getenv("BOT_ACCOUNT_NAME"),
				os.Getenv("DISCORD_WEBHOOK_URL"),
				os.Getenv("GITHUB_TOKEN"),
			),
		)
		accessLogProfiler := profilertools.NewAccessLogProfiler(
			e,
			fmt.Sprintf(
				"%s:%s",
				os.Getenv("REVERSE_PROXY_HOST"),
				os.Getenv("HTTP_PORT"),
			),
			profilertools.AccessLogDiscordNotifierOption(
				os.Getenv("KATARIBE_CONF_FILE_PATH"),
				os.Getenv("BOT_ACCOUNT_NAME"),
				os.Getenv("DISCORD_WEBHOOK_URL"),
				os.Getenv("GITHUB_TOKEN"),
			),
		)
		profiler.AddProfiler(slowLogProfiler)
		profiler.AddProfiler(accessLogProfiler)
		go profiler.ListenAndServe(8080)
	} else {
		if _, err := db.Exec("SET GLOBAL slow_query_log = 0"); err != nil {
			log.Fatalf("failed to disable slow_query_log: %v", err)
		}
	}

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", os.Getenv("HTTP_PORT"))))
}
