package profiler_test

import (
	"database/sql"
	"net/http/httptest"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	profilertools "github.com/goccy/echo-tools/profiler"
	"github.com/labstack/echo/v4"
	mysqltest "github.com/lestrrat-go/test-mysqld"
)

func TestSlowQueryLog(t *testing.T) {
	mysqld, err := mysqltest.NewMysqld(mysqltest.NewConfig())
	if err != nil {
		t.Fatalf("Failed to start mysqld: %s", err)
	}
	defer mysqld.Stop()

	profilertools.ReplaceQueryDigestCommandTemplate()

	db, err := sql.Open("mysql", mysqld.Datasource("test", "", "", 0))
	e := echo.New()
	server := httptest.NewServer(e)
	defer server.Close()
	p := profilertools.NewMySQLSlowQueryLogProfiler(e, server.URL, db)
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	if err := p.Stop(); err != nil {
		t.Fatal(err)
	}
}
