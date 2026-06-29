package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type requestResult struct {
	status  int
	latency time.Duration
	err     error
	detail  string
}

type stepSummary struct {
	Concurrency int
	Total       int
	Success     int
	Failed      int
	FailurePct  float64
	RPS         float64
	SuccessRPS  float64
	Avg         time.Duration
	P50         time.Duration
	P95         time.Duration
	P99         time.Duration
	Statuses    map[int]int
	Wall        time.Duration
	FirstError  string
}

type options struct {
	target          string
	path            string
	method          string
	body            string
	contentType     string
	concurrencyList string
	duration        time.Duration
	timeout         time.Duration
	failThreshold   float64
	maxP95          time.Duration
	loginEmail      string
	loginPassword   string
	loginPath       string
	cookieHeader    string
	insecureTLS     bool
}

func main() {
	if code := run(parseFlags(), os.Stdout, os.Stderr); code != 0 {
		os.Exit(code)
	}
}

func run(opts options, stdout, stderr io.Writer) int {
	steps, err := parseConcurrency(opts.concurrencyList)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	target, err := buildURL(opts.target, opts.path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	client := newHTTPClient(1, opts.timeout, opts.insecureTLS)
	cookieHeader := opts.cookieHeader
	if opts.loginEmail != "" || opts.loginPassword != "" {
		cookieHeader, err = login(client, opts.target, opts.loginPath, opts.loginEmail, opts.loginPassword)
		if err != nil {
			fmt.Fprintln(stderr, "login:", err)
			return 1
		}
	}

	fmt.Fprintf(stdout, "Target: %s %s\n", strings.ToUpper(opts.method), target)
	fmt.Fprintf(stdout, "Duration: %s per step, timeout: %s, failure threshold: %.2f%%", opts.duration, opts.timeout, opts.failThreshold)
	if opts.maxP95 > 0 {
		fmt.Fprintf(stdout, ", p95 threshold: %s", opts.maxP95)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "%8s %10s %10s %10s %8s %10s %10s %10s %10s %10s %s\n", "CONC", "REQS", "OK", "FAIL", "FAIL%", "RPS", "AVG", "P50", "P95", "P99", "STATUS")

	summaries := make([]stepSummary, 0, len(steps))
	for _, concurrency := range steps {
		stepClient := newHTTPClient(concurrency, opts.timeout, opts.insecureTLS)
		summary := runStep(stepClient, target, opts, cookieHeader, concurrency)
		stepClient.CloseIdleConnections()
		summaries = append(summaries, summary)
		fmt.Fprintf(stdout, "%8d %10d %10d %10d %7.2f%% %10.1f %10s %10s %10s %10s %s\n",
			summary.Concurrency,
			summary.Total,
			summary.Success,
			summary.Failed,
			summary.FailurePct,
			summary.RPS,
			formatDuration(summary.Avg),
			formatDuration(summary.P50),
			formatDuration(summary.P95),
			formatDuration(summary.P99),
			formatStatuses(summary.Statuses),
		)
	}

	if best, ok := bestCapacity(summaries, opts.failThreshold, opts.maxP95); ok {
		fmt.Fprintf(stdout, "\nSuggested capacity: %d concurrent requests under the configured threshold.\n", best)
	} else {
		fmt.Fprintln(stdout, "\nSuggested capacity: none of the tested levels met the configured threshold.")
	}
	return 0
}

func parseFlags() options {
	var opts options
	flag.StringVar(&opts.target, "target", "http://127.0.0.1:8000", "backend base URL")
	flag.StringVar(&opts.path, "path", "/v1/public/settings", "request path, or absolute URL")
	flag.StringVar(&opts.method, "method", http.MethodGet, "HTTP method")
	flag.StringVar(&opts.body, "body", "", "request body")
	flag.StringVar(&opts.contentType, "content-type", "application/json", "request Content-Type")
	flag.StringVar(&opts.concurrencyList, "concurrency", "1,5,10,25,50,100", "comma-separated concurrency levels")
	flag.DurationVar(&opts.duration, "duration", 10*time.Second, "duration for each concurrency level")
	flag.DurationVar(&opts.timeout, "timeout", 5*time.Second, "per-request timeout")
	flag.Float64Var(&opts.failThreshold, "fail-threshold", 1, "maximum acceptable failure percentage")
	flag.DurationVar(&opts.maxP95, "max-p95", 0, "optional maximum acceptable p95 latency; 0 disables this check")
	flag.StringVar(&opts.loginEmail, "login-email", "", "email used to log in before the test")
	flag.StringVar(&opts.loginPassword, "login-password", "", "password used to log in before the test")
	flag.StringVar(&opts.loginPath, "login-path", "/v1/auth/login", "login path")
	flag.StringVar(&opts.cookieHeader, "cookie", "", "Cookie header to send with every request")
	flag.BoolVar(&opts.insecureTLS, "insecure", false, "skip TLS certificate verification")
	flag.Parse()
	return opts
}

func parseConcurrency(raw string) ([]int, error) {
	parts := strings.Split(raw, ",")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid concurrency level %q", part)
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("at least one concurrency level is required")
	}
	return out, nil
}

func buildURL(base, requestPath string) (string, error) {
	if strings.HasPrefix(requestPath, "http://") || strings.HasPrefix(requestPath, "https://") {
		return requestPath, nil
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("target must include scheme and host")
	}
	req, err := url.Parse(requestPath)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(req.Path, "/")
	u.RawQuery = req.RawQuery
	u.Fragment = req.Fragment
	return u.String(), nil
}

func newHTTPClient(concurrency int, timeout time.Duration, insecureTLS bool) *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        concurrency * 4,
		MaxIdleConnsPerHost: concurrency * 4,
		MaxConnsPerHost:     concurrency,
		IdleConnTimeout:     30 * time.Second,
	}
	if insecureTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}

func login(client *http.Client, base, loginPath, email, password string) (string, error) {
	loginURL, err := buildURL(base, loginPath)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]string{"email": email, "password": password})
	if err != nil {
		return "", err
	}
	resp, err := client.Post(loginURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var cookies []string
	for _, cookie := range resp.Cookies() {
		cookies = append(cookies, cookie.Name+"="+cookie.Value)
	}
	if len(cookies) == 0 {
		return "", fmt.Errorf("login succeeded without cookies")
	}
	return strings.Join(cookies, "; "), nil
}

func runStep(client *http.Client, target string, opts options, cookieHeader string, concurrency int) stepSummary {
	results := make(chan requestResult, concurrency*32)
	var wg sync.WaitGroup
	start := time.Now()
	end := start.Add(opts.duration)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(end) {
				results <- doRequest(client, target, opts, cookieHeader)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	collected := make([]requestResult, 0, concurrency*128)
	for result := range results {
		collected = append(collected, result)
	}
	return summarize(concurrency, collected, time.Since(start))
}

func doRequest(client *http.Client, target string, opts options, cookieHeader string) requestResult {
	var body io.Reader
	if opts.body != "" {
		body = strings.NewReader(opts.body)
	}
	req, err := http.NewRequest(strings.ToUpper(opts.method), target, body)
	if err != nil {
		return requestResult{err: err}
	}
	if opts.body != "" {
		req.Header.Set("Content-Type", opts.contentType)
	}
	if cookieHeader != "" {
		req.Header.Set("Cookie", cookieHeader)
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return requestResult{latency: latency, err: err, detail: err.Error()}
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	_, _ = io.Copy(io.Discard, resp.Body)
	detail := ""
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		detail = strings.TrimSpace(string(data))
	}
	return requestResult{status: resp.StatusCode, latency: latency, detail: detail}
}

func summarize(concurrency int, results []requestResult, wall time.Duration) stepSummary {
	statuses := map[int]int{}
	latencies := make([]time.Duration, 0, len(results))
	var totalLatency time.Duration
	firstError := ""
	success := 0
	for _, result := range results {
		latencies = append(latencies, result.latency)
		totalLatency += result.latency
		if result.err == nil {
			statuses[result.status]++
		}
		if result.err == nil && result.status >= 200 && result.status < 400 {
			success++
		} else if firstError == "" {
			if result.detail != "" {
				firstError = result.detail
			} else if result.err != nil {
				firstError = result.err.Error()
			} else {
				firstError = fmt.Sprintf("status %d", result.status)
			}
		}
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	total := len(results)
	failed := total - success
	var avg time.Duration
	var failurePct float64
	var rps float64
	var successRPS float64
	if total > 0 {
		avg = totalLatency / time.Duration(total)
		failurePct = float64(failed) * 100 / float64(total)
	}
	if wall > 0 {
		rps = float64(total) / wall.Seconds()
		successRPS = float64(success) / wall.Seconds()
	}
	return stepSummary{
		Concurrency: concurrency,
		Total:       total,
		Success:     success,
		Failed:      failed,
		FailurePct:  failurePct,
		RPS:         rps,
		SuccessRPS:  successRPS,
		Avg:         avg,
		P50:         percentile(latencies, 50),
		P95:         percentile(latencies, 95),
		P99:         percentile(latencies, 99),
		Statuses:    statuses,
		Wall:        wall,
		FirstError:  firstError,
	}
}

func percentile(sorted []time.Duration, pct float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	index := int(math.Ceil((pct/100)*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func bestCapacity(summaries []stepSummary, failThreshold float64, maxP95 time.Duration) (int, bool) {
	best := 0
	for _, summary := range summaries {
		if summary.Total == 0 || summary.FailurePct > failThreshold {
			continue
		}
		if maxP95 > 0 && summary.P95 > maxP95 {
			continue
		}
		if summary.Concurrency > best {
			best = summary.Concurrency
		}
	}
	return best, best > 0
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "-"
	}
	if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

func formatStatuses(statuses map[int]int) string {
	if len(statuses) == 0 {
		return "errors-only"
	}
	keys := make([]int, 0, len(statuses))
	for key := range statuses {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%d:%d", key, statuses[key]))
	}
	return strings.Join(parts, ",")
}
