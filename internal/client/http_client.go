package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HttpClient struct {
	BaseURL string
	Client  *http.Client
}

func NewHttpClient(baseURL string) *HttpClient {
	return &HttpClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Client:  &http.Client{Timeout: 30 * time.Second},
	}
}

type RequestOption struct {
	Headers map[string]string
	Query   map[string]string
	Body    interface{}
}

func (c *HttpClient) Do(ctx context.Context, method, path string, opt *RequestOption) ([]byte, error) {
	fullURL := c.BaseURL + path
	if opt != nil && len(opt.Query) > 0 {
		values := url.Values{}
		for k, v := range opt.Query {
			values.Set(k, v)
		}
		if strings.Contains(fullURL, "?") {
			fullURL += "&" + values.Encode()
		} else {
			fullURL += "?" + values.Encode()
		}
	}
	var bodyReader io.Reader
	if opt != nil && opt.Body != nil {
		jsonData, err := json.Marshal(opt.Body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(jsonData)
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if opt != nil && opt.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if opt != nil && opt.Headers != nil {
		for k, v := range opt.Headers {
			req.Header.Set(k, v)
		}
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HttpError{StatusCode: resp.StatusCode, Body: respBody}
	}
	return respBody, nil
}

type HttpError struct {
	StatusCode int
	Body       []byte
}

func (e *HttpError) Error() string {
	return "http error: " + http.StatusText(e.StatusCode) + ", body: " + string(e.Body)
}
