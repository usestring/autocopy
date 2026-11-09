// Package jev implements the autocopy classifier with the TypeSafe JEV API.
// Callers provide the key and choose when to send a message.
package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/usestring/autocopy"
)

const (
	Endpoint = "https://api.typesafe.ai/v1/systemone"
	Model    = "jev-1.13.0"
)

type Client struct {
	Key        string
	Endpoint   string
	Model      string
	HTTPClient *http.Client
}

var defaultHTTPClient = &http.Client{
	Timeout:       6 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func (c *Client) Classify(ctx context.Context, input autocopy.Input) (map[string]float64, error) {
	if c.Key == "" {
		return nil, fmt.Errorf("jev: API key is required")
	}
	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = Endpoint
	}
	model := c.Model
	if model == "" {
		model = Model
	}
	questions := make(map[string]any, len(input.Questions))
	for name, instructions := range input.Questions {
		questions[name] = map[string]string{"type": "noul", "instructions": instructions}
	}
	body, err := json.Marshal(map[string]any{
		"model": model, "state": map[string]string{"message": input.Message}, "questions": questions,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Key)
	client := c.HTTPClient
	if client == nil {
		client = defaultHTTPClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jev: HTTP %d", resp.StatusCode)
	}
	var reply struct {
		Model   string `json:"model"`
		Answers map[string]struct {
			Type string   `json:"type"`
			Noul *float64 `json:"noul"`
		} `json:"answers"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&reply); err != nil {
		return nil, err
	}
	if reply.Model != model {
		return nil, fmt.Errorf("jev: unexpected response model")
	}
	out := make(map[string]float64, len(input.Questions))
	for name := range input.Questions {
		a, ok := reply.Answers[name]
		if !ok || a.Type != "noul" || a.Noul == nil || *a.Noul < 0 || *a.Noul > 1 {
			return nil, fmt.Errorf("jev: invalid or missing answer %s", name)
		}
		out[name] = *a.Noul
	}
	return out, nil
}
