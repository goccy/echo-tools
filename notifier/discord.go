package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type DiscordClient struct {
	webhookURL string
}

func NewDiscordClient(webhookURL string) *DiscordClient {
	return &DiscordClient{webhookURL: webhookURL}
}

type DiscordMessage struct {
	Username string `json:"username"`
	Content  string `json:"content"`
}

func (c *DiscordClient) Post(msg *DiscordMessage) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}
	req, err := http.NewRequest("POST", c.webhookURL, bytes.NewBuffer(b))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Content-Type", "application/json")
	httpClient := new(http.Client)
	if _, err := httpClient.Do(req); err != nil {
		return fmt.Errorf("failed to post message: %w", err)
	}
	return nil
}
