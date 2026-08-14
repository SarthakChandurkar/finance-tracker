package remote

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
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
const TodoServerTimeout = 5 * time.Minute

// maxAttempts and retryBackoff only govern transport-level failures - DNS
// lookup timeouts (the flaky-local-resolver case this was written for),
// connection refused, connection reset, etc. A real HTTP response from
// the server, even an error status like 500, is never retried: that
// means the server is clearly reachable, so retrying wouldn't help and
// would just make a genuine error slower to surface.
const maxAttempts = 3

var retryBackoff = []time.Duration{250 * time.Millisecond, 750 * time.Millisecond}

func NewTodoClient(baseURL string, timeout time.Duration) *TodoClient {
	return &TodoClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		// No client-level Timeout here - Forward derives one shared
		// context deadline (below) that covers the whole operation,
		// retries included, instead of giving each attempt its own full
		// timeout budget (which would let 3 attempts take up to 3x as
		// long as configured).
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

	// Buffer the body up front (if any) so every retry attempt can
	// replay it - the original reader (typically an in-flight request
	// body from the incoming HTTP request) can only be read once.
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

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(retryBackoff[attempt-1]):
			case <-ctx.Done():
				return nil, fmt.Errorf("reaching task server: %w", ctx.Err())
			}
		}

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
			lastErr = err
			if !isRetryable(err) {
				return nil, fmt.Errorf("reaching task server: %w", err)
			}
			continue
		}

		// A real response came back - even an error status code is not
		// retried, since the server is clearly up and answering.
		data, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
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

	return nil, fmt.Errorf("reaching task server after %d attempts: %w", maxAttempts, lastErr)
}

// isRetryable reports whether err looks like a transient, transport-level
// failure worth retrying - a DNS lookup timeout (the systemd-resolved
// case this was written for), connection refused, connection reset, or
// a timeout establishing the connection - as opposed to something a
// retry won't fix, like a canceled request.
func isRetryable(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}

	return false
}
