package client

import (
	"fmt"
	"net/http"
	"time"
)

// RetryableClient struct for retry mechanism.
type RetryableClient struct {
	client        *http.Client
	retryMax      int
	retryInterval time.Duration
}

// NewRetryableClient handles retrying client with intervel.
func NewRetryableClient(retryMax int, retryInterval time.Duration) *RetryableClient {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	return &RetryableClient{client: client, retryMax: retryMax, retryInterval: retryInterval}
}

// DoRequest handles http request and return response and error
func (r *RetryableClient) DoRequest(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error
	for i := 0; i < r.retryMax; i++ {
		resp, err = r.client.Do(req)
		if err == nil {
			return resp, nil
		}
		time.Sleep(r.retryInterval)
	}
	return nil, fmt.Errorf("request failed after %d retries: %v", r.retryMax, err)
}
