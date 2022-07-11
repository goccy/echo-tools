package notifier_test

import (
	"testing"

	"github.com/goccy/echo-tools/notifier"
)

func TestDiscordClient(t *testing.T) {
	_ = notifier.NewDiscordClient("")
}
