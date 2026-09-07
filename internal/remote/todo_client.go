package remote

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type TodoClient struct {
	baseURL string
	http    *http.Client
	timeout time.Duration
}

const DefaultTodoServerURL = ""
const TodoServerTimeout = 10 * time.Minute

func NewTodoClient(baseURL string, timeout time.Duration) *TodoClient {
	return &TodoClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{},
		timeout: timeout,
	}
}

type Response struct {
	StatusCode  int
	Body        []byte
	ContentType string
}

func (c *TodoClient) Forward(ctx context.Context, method, pathSuffix string, body io.Reader, headers http.Header) (*Response, error) {
	fullURL := c.baseURL + "/tasks" + pathSuffix

	var bodyBytes []byte
	if body != nil {
		b, err := io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("reading request body: %w", err)
		}
		bodyBytes = b
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var reqBody io.Reader
	if bodyBytes != nil {
		reqBody = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header = headers.Clone()
	req.Header.Del("Accept-Encoding")
	if bodyBytes != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reaching task server: %w", err)
	}
	defer resp.Body.Close()

	data, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("reading task server response: %w", readErr)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}

	return &Response{
		StatusCode:  resp.StatusCode,
		Body:        data,
		ContentType: contentType,
	}, nil
}
