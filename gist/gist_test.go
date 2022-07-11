package gist_test

import (
	"context"
	"testing"

	"github.com/goccy/echo-tools/gist"
)

func TestGistClient(t *testing.T) {
	if _, err := gist.NewClient(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
}
