package imports

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"element-skin/backend/internal/util"
)

func (s RemoteYggService) doJSON(ctx context.Context, method, rawURL string, payload any, out any) error {
	if err := util.ValidateOutboundURL(rawURL); err != nil {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "api_url", Operation: "validate", Reason: "invalid"}
	}
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "api_url", Operation: "validate", Reason: "invalid"}
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	resp, err := remoteYggHTTPClient(s.HTTPClient).Do(req)
	if err != nil {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "remote_profile", Operation: "fetch", Reason: "unavailable"}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "remote_profile", Operation: "authenticate", Reason: "denied"}
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := decoder.Decode(out); err != nil {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "remote_profile", Operation: "decode", Reason: "invalid"}
	}
	return nil
}

func remoteYggHTTPClient(base *http.Client) *http.Client {
	if base == nil {
		base = util.NewSecureOutboundHTTPClient(10 * time.Second)
	}
	client := *base
	if client.Timeout == 0 {
		client.Timeout = 10 * time.Second
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &client
}

func remoteYggURL(apiURL string, parts ...string) string {
	base := strings.TrimRight(strings.TrimSpace(apiURL), "/")
	all := append([]string{base}, parts...)
	joined, err := url.JoinPath(all[0], all[1:]...)
	if err != nil {
		return base
	}
	return joined
}
