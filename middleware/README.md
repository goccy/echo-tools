# Synopsis

## BenchmarkFinishEventNotifier

After starting the benchmark, the middleware that calls an arbitrary function when there are no more requests.

```
package main

import (
    middlewaretools "github.com/goccy/echo-tools/middleware"
    "github.com/labstack/echo/v4"
)

var (
	benchmarkFinishEventNotifier = middlewaretools.NewBenchmarkFinishEventNotifier(onBenchmarkFinished)
)

func startBenchmark(c echo.Context) error {
	benchmarkFinishEventNotifier.Start()
	return c.JSON(http.StatusOK, struct{}{})
}

func onBenchmarkFinished() {
	fmt.Println("benchmark finished")
}

func main() {
    e := echo.New()
    e.POST("/initialize", startBenchmark)
    e.Use(benchmarkFinishEventNotifier.Middleware())
}
```