package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/GinKuReNai/scythe/internal/analysis"
	"github.com/GinKuReNai/scythe/internal/decision"
)

const Endpoint = "https://jev-ai.org/api/v1/systemone/"

type Client struct {
	http                 *http.Client
	endpoint, model, key string
}

func NewClient(client *http.Client, endpoint, model, key string) *Client {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	copyClient := *client
	// Never forward authentication or evidence to a redirected destination.
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if endpoint == "" {
		endpoint = Endpoint
	}
	return &Client{&copyClient, endpoint, model, key}
}
func (c *Client) Decide(ctx context.Context, e analysis.Evidence) (decision.Result, error) {
	if c.key == "" {
		return decision.Result{}, errors.New("JEV_API_KEY is missing")
	}
	body, err := json.Marshal(request{c.model, e, questions()})
	if err != nil {
		return decision.Result{}, fmt.Errorf("encode Jev evidence: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return decision.Result{}, errors.New("invalid Jev endpoint")
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return decision.Result{}, ctx.Err()
		}
		// Transport errors can echo request details. Only emit a bounded safe class.
		var networkError net.Error
		if errors.As(err, &networkError) && networkError.Timeout() {
			return decision.Result{}, errors.New("Jev request timed out")
		}
		return decision.Result{}, errors.New("Jev HTTP request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return decision.Result{}, fmt.Errorf("Jev returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
	if err != nil {
		return decision.Result{}, errors.New("read Jev response failed")
	}
	if len(data) > 1024*1024 {
		return decision.Result{}, errors.New("Jev response exceeds size limit")
	}
	result, err := convert(data)
	if err != nil {
		return decision.Result{}, err
	}
	// The server must never cause a key to enter a persisted response/report.
	if bytes.Contains(data, []byte(c.key)) {
		return decision.Result{}, errors.New("Jev response contains credentials")
	}
	return result, nil
}
