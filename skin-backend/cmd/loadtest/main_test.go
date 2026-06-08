package main

import (
	"net/http"
	"reflect"
	"testing"
	"time"
)

func TestParseConcurrency(t *testing.T) {
	got, err := parseConcurrency("1, 5,10")
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{1, 5, 10}; !reflect.DeepEqual(got, want) {
		t.Fatalf("parseConcurrency mismatch: got=%v want=%v", got, want)
	}
	if _, err := parseConcurrency("1, nope"); err == nil {
		t.Fatal("invalid concurrency should fail")
	}
	if _, err := parseConcurrency("0"); err == nil {
		t.Fatal("zero concurrency should fail")
	}
}

func TestBuildURL(t *testing.T) {
	got, err := buildURL("http://127.0.0.1:8000/api", "/public/settings")
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://127.0.0.1:8000/api/public/settings" {
		t.Fatalf("unexpected URL: %s", got)
	}
	got, err = buildURL("http://127.0.0.1:8000/api", "/admin/users?limit=20&q=Load")
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://127.0.0.1:8000/api/admin/users?limit=20&q=Load" {
		t.Fatalf("query string should stay as query, got: %s", got)
	}
	got, err = buildURL("http://ignored", "https://example.com/me")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://example.com/me" {
		t.Fatalf("absolute URL should pass through: %s", got)
	}
	if _, err := buildURL("127.0.0.1:8000", "/me"); err == nil {
		t.Fatal("target without scheme should fail")
	}
}

func TestBestCapacity(t *testing.T) {
	summaries := []stepSummary{
		{Concurrency: 10, Total: 100, FailurePct: 0.5, P95: 90 * time.Millisecond},
		{Concurrency: 25, Total: 100, FailurePct: 1.0, P95: 150 * time.Millisecond},
		{Concurrency: 50, Total: 100, FailurePct: 3.0, P95: 100 * time.Millisecond},
	}
	best, ok := bestCapacity(summaries, 1, 200*time.Millisecond)
	if !ok || best != 25 {
		t.Fatalf("best capacity mismatch: best=%d ok=%v", best, ok)
	}
	best, ok = bestCapacity(summaries, 1, 100*time.Millisecond)
	if !ok || best != 10 {
		t.Fatalf("p95 threshold should lower capacity: best=%d ok=%v", best, ok)
	}
	if _, ok = bestCapacity(summaries, 0.1, 50*time.Millisecond); ok {
		t.Fatal("no capacity should pass strict thresholds")
	}
}

func TestNewHTTPClientCapsConnectionsPerHost(t *testing.T) {
	client := newHTTPClient(37, time.Second, false)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("unexpected transport type: %T", client.Transport)
	}
	if transport.MaxConnsPerHost != 37 {
		t.Fatalf("MaxConnsPerHost mismatch: %d", transport.MaxConnsPerHost)
	}
	if transport.TLSClientConfig != nil {
		t.Fatal("TLS config should only be set for insecure mode")
	}

	insecure := newHTTPClient(1, time.Second, true)
	insecureTransport := insecure.Transport.(*http.Transport)
	if insecureTransport.TLSClientConfig == nil || !insecureTransport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("insecure mode should skip TLS verification")
	}
}

func TestSummarize(t *testing.T) {
	summary := summarize(2, []requestResult{
		{status: 200, latency: 10 * time.Millisecond},
		{status: 204, latency: 20 * time.Millisecond},
		{status: 500, latency: 30 * time.Millisecond},
	}, time.Second)
	if summary.Total != 3 || summary.Success != 2 || summary.Failed != 1 {
		t.Fatalf("unexpected counts: %#v", summary)
	}
	if summary.RPS != 3 {
		t.Fatalf("unexpected rps: %f", summary.RPS)
	}
	if summary.SuccessRPS != 2 {
		t.Fatalf("unexpected success rps: %f", summary.SuccessRPS)
	}
	if summary.P50 != 20*time.Millisecond || summary.P95 != 30*time.Millisecond {
		t.Fatalf("unexpected percentiles: p50=%s p95=%s", summary.P50, summary.P95)
	}
}

func TestSummarizeCapacities(t *testing.T) {
	cfg := capacityConfig{
		Levels:        []int{1, 5, 10},
		FailThreshold: 1,
		MaxP95:        100 * time.Millisecond,
	}
	results := []scenarioResult{
		{Scenario: loadScenario{Name: "a"}, Concurrency: 1, Summary: stepSummary{Concurrency: 1, Total: 10, FailurePct: 0, P95: 10 * time.Millisecond, SuccessRPS: 10}},
		{Scenario: loadScenario{Name: "a"}, Concurrency: 5, Summary: stepSummary{Concurrency: 5, Total: 10, FailurePct: 0, P95: 90 * time.Millisecond, SuccessRPS: 50}},
		{Scenario: loadScenario{Name: "a"}, Concurrency: 10, Summary: stepSummary{Concurrency: 10, Total: 10, FailurePct: 2, P95: 90 * time.Millisecond, SuccessRPS: 98}},
		{Scenario: loadScenario{Name: "b"}, Concurrency: 10, Summary: stepSummary{Concurrency: 10, Total: 10, FailurePct: 0, P95: 50 * time.Millisecond, SuccessRPS: 100}},
		{Scenario: loadScenario{Name: "c"}, Concurrency: 1, Summary: stepSummary{Concurrency: 1, Total: 10, FailurePct: 0, P95: 150 * time.Millisecond, SuccessRPS: 10}},
	}
	got := summarizeCapacities(results, cfg)
	if len(got) != 3 {
		t.Fatalf("unexpected capacity count: %#v", got)
	}
	if got[0].Scenario.Name != "a" || got[0].Best.Concurrency != 5 || got[0].HitTestCeiling {
		t.Fatalf("scenario a capacity mismatch: %#v", got[0])
	}
	if got[1].Scenario.Name != "b" || got[1].Best.Concurrency != 10 || !got[1].HitTestCeiling {
		t.Fatalf("scenario b capacity mismatch: %#v", got[1])
	}
	if got[2].Scenario.Name != "c" || got[2].Pass {
		t.Fatalf("scenario c should have no passing level: %#v", got[2])
	}
}
