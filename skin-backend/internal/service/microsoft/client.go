package microsoft

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type UpstreamHTTPError struct {
	StatusCode int
	Body       string
}

func (e *UpstreamHTTPError) Error() string {
	return fmt.Sprintf("microsoft request failed: status=%d body=%s", e.StatusCode, e.Body)
}

func IsUnauthorized(err error) bool {
	var upstreamErr *UpstreamHTTPError
	return errors.As(err, &upstreamErr) && upstreamErr.StatusCode == http.StatusUnauthorized
}

type MicrosoftHTTPClient struct {
	Client       *http.Client
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

func (c MicrosoftHTTPClient) httpClient() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func (c MicrosoftHTTPClient) postJSON(ctx context.Context, endpoint string, body any, bearer string, out any) error {
	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(body); err != nil {
		return err
	}
	return c.do(ctx, "POST", endpoint, &b, "application/json", bearer, out)
}

func (c MicrosoftHTTPClient) do(ctx context.Context, method, endpoint string, body io.Reader, contentType, bearer string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if bearer != "" {
		req.Header.Set("Authorization", bearer)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 && method == "GET" && strings.Contains(endpoint, "/minecraft/profile") {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &UpstreamHTTPError{StatusCode: resp.StatusCode, Body: string(b)}
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
