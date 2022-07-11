# Synopsis

1. Call `NewProfiler` to create a profiler instance. At this time, specify the directory to store the result of pprof.

2. Start webui with the port to see the profile results: `go profiler.ListenAndServe(8080)`
3. Start profiling by `profiler.Start()`
4. Stop profiling by `profiler.Stop()`

You can see the profiling results by accessing `http://localhost:8080` in your browser.
If you profile again, the latest results will be automatically reflected.
If you want to see the previous result, you can also refer to the past result by specifying the number in the profiling order like `http://localhost:8080/1/` .

```go
package main

import (
  middlewaretools "github.com/goccy/echo-tools/middleware"
  profilertools "github.com/goccy/echo-tools/profiler"
  "github.com/labstack/echo/v4"
)

var (
  notifier = middlewaretools.NewBenchmarkFinishEventNotifier(onBenchmarkFinished)
  profiler = profilertools.NewProfiler("profile")
)

func startBenchmark(c echo.Context) error {
  notifier.Start()
  profiler.Start()
  return c.JSON(http.StatusOK, struct{}{})
}

func onBenchmarkFinished() {
  profiler.Stop()
}

func main() {
  go profiler.ListenAndServe(8080)
  e := echo.New()
  e.Use(notifier.Middleware())
}
```
