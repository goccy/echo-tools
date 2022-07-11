package middleware

import (
	"log"
	"time"

	"github.com/labstack/echo/v4"
)

const (
	defaultBenchmarkFinishEventCheckInterval = 3 * time.Second
)

type BenchmarkFinishEventNotifierOption func(*BenchmarkFinishEventNotifier)

type BenchmarkFinishEventNotifier struct {
	onFinishedFunc func()
	interval       time.Duration
	startCh        chan struct{}
}

func BenchmarkFinishEventCheckInterval(interval time.Duration) BenchmarkFinishEventNotifierOption {
	return func(n *BenchmarkFinishEventNotifier) {
		n.interval = interval
	}
}

func NewBenchmarkFinishEventNotifier(onFinished func(), opts ...BenchmarkFinishEventNotifierOption) *BenchmarkFinishEventNotifier {
	ret := &BenchmarkFinishEventNotifier{
		interval:       defaultBenchmarkFinishEventCheckInterval,
		onFinishedFunc: onFinished,
		startCh:        make(chan struct{}),
	}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

func (n *BenchmarkFinishEventNotifier) Start() {
	n.startCh <- struct{}{}
}

func (n *BenchmarkFinishEventNotifier) Middleware() echo.MiddlewareFunc {
	ticker := time.NewTicker(n.interval)
	var lastReceivedRequestTime time.Time
	go func() {
		for {
			log.Print("[benchmark-finish-event-notifier] wait for starting bechmark")
			<-n.startCh // wait start benchmark
			log.Print("[benchmark-finish-event-notifier] start benchmark")
			for {
				select {
				case t := <-ticker.C:
					duration := t.Sub(lastReceivedRequestTime)
					if duration > n.interval {
						goto END_BENCHMARK
					}
				}
			}
		END_BENCHMARK:
			log.Print("[benchmark-finish-event-notifier] finished benchmark")
			n.onFinishedFunc()
		}
	}()
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			lastReceivedRequestTime = time.Now()
			return next(c)
		}
	}
}
