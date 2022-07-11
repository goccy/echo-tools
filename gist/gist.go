package gist

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/go-github/v45/github"
	"golang.org/x/oauth2"
)

type Client struct {
	cli *github.Client
}

func NewClient(ctx context.Context, token string) (*Client, error) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	cli := github.NewClient(oauth2.NewClient(ctx, ts))
	return &Client{cli: cli}, nil
}

func (c *Client) UploadFile(ctx context.Context, title, path string) (string, error) {
	fileMap := map[github.GistFilename]github.GistFile{}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", path, err)
	}
	fileMap[github.GistFilename(filepath.Base(path))] = github.GistFile{
		Content: github.String(string(content)),
	}
	g, _, err := c.cli.Gists.Create(ctx, &github.Gist{
		Description: github.String(title),
		Files:       fileMap,
		Public:      github.Bool(false),
	})
	if err != nil {
		return "", fmt.Errorf("failed to create gist: %w", err)
	}
	return g.GetHTMLURL(), nil
}
