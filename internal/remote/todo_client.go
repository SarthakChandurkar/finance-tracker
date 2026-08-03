package remote

import (
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
}

const DefaultTodoServerURL = ""
const TodoServerTimeout = 90 * time.Second

func NewTodoClient(baseURL string, timeout time.Duration) *TodoClient {
	return &TodoClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: timeout},
	}
}

type Response struct {
	StatusCode  int
	Body        []byte
	ContentType string
}

func (c *TodoClient) Forward(ctx context.Context, method, pathSuffix string, body io.Reader, headers http.Header) (*Response, error) {

	fullURL := c.baseURL + "/tasks" + pathSuffix

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, err
	}

	req.Header = headers.Clone()
	req.Header.Del("Accept-Encoding")
	// ...
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reaching task server: %w", err)
	}

	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading task server response: %w", err)
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
