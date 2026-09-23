package moderation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Result struct {
	Class        string  `json:"class"`
	Score        float64 `json:"score"`
	ModelVersion string  `json:"modelVersion"`
}

type Client interface {
	Moderate(ctx context.Context, contentType string, image []byte) (Result, error)
}

type HTTPClient struct {
	endpoint string
	client   *http.Client
}

func NewHTTPClient(endpoint string) (*HTTPClient, error) {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" {
		return nil, fmt.Errorf("moderation endpoint is required")
	}
	return &HTTPClient{endpoint: endpoint, client: &http.Client{Timeout: 10 * time.Second}}, nil
}

func (c *HTTPClient) Moderate(ctx context.Context, contentType string, image []byte) (Result, error) {
	if c == nil || c.client == nil || len(image) == 0 || (contentType != "image/jpeg" && contentType != "image/png") {
		return Result{}, fmt.Errorf("invalid moderation request")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/v1/moderate", bytes.NewReader(image))
	if err != nil {
		return Result{}, fmt.Errorf("create moderation request: %w", err)
	}
	request.Header.Set("Content-Type", contentType)
	response, err := c.client.Do(request)
	if err != nil {
		return Result{}, fmt.Errorf("call moderation service: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("moderation service returned %d", response.StatusCode)
	}
	var result Result
	if err = json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(&result); err != nil {
		return Result{}, fmt.Errorf("decode moderation response: %w", err)
	}
	if result.Class == "" || result.Score < 0 || result.Score > 1 || result.ModelVersion == "" {
		return Result{}, fmt.Errorf("invalid moderation response")
	}
	return result, nil
}
