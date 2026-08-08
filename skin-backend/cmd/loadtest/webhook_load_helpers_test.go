package main

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestWebhookLoadConfigUsesBalancedDefaults(t *testing.T) {
	clearWebhookLoadEnv(t)
	got, err := webhookLoadConfigFromEnv()
	want := webhookLoadConfig{
		Concurrency:  50,
		Duration:     time.Second,
		Repeats:      4,
		WorkerEvents: 1000,
		MaxDBConns:   20,
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("default webhook load config=%#v err=%v want=%#v", got, err, want)
	}

	for order := range webhookLoadModes {
		counts := make(map[string]int, len(webhookLoadModes))
		for repeat := range webhookLoadModes {
			counts[webhookLoadModeFor(repeat, order).Name]++
		}
		for _, mode := range webhookLoadModes {
			if counts[mode.Name] != 1 {
				t.Fatalf("order=%d mode=%s count=%d want=1", order+1, mode.Name, counts[mode.Name])
			}
		}
	}
}

func TestWebhookLoadConfigReadsExactValuesAndRejectsInvalidInputs(t *testing.T) {
	clearWebhookLoadEnv(t)
	t.Setenv("WEBHOOK_LOADTEST_CONCURRENCY", "40")
	t.Setenv("WEBHOOK_LOADTEST_DURATION", "2s")
	t.Setenv("WEBHOOK_LOADTEST_REPEATS", "5")
	t.Setenv("WEBHOOK_LOADTEST_EVENTS", "2500")
	t.Setenv("WEBHOOK_LOADTEST_DB_MAX_CONNECTIONS", "30")
	got, err := webhookLoadConfigFromEnv()
	want := webhookLoadConfig{
		Concurrency:  40,
		Duration:     2 * time.Second,
		Repeats:      5,
		WorkerEvents: 2500,
		MaxDBConns:   30,
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("webhook load config=%#v err=%v want=%#v", got, err, want)
	}

	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "zero concurrency", key: "WEBHOOK_LOADTEST_CONCURRENCY", value: "0"},
		{name: "excess repeats", key: "WEBHOOK_LOADTEST_REPEATS", value: "21"},
		{name: "invalid events", key: "WEBHOOK_LOADTEST_EVENTS", value: "many"},
		{name: "excess db pool", key: "WEBHOOK_LOADTEST_DB_MAX_CONNECTIONS", value: "501"},
		{name: "invalid duration", key: "WEBHOOK_LOADTEST_DURATION", value: "61s"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearWebhookLoadEnv(t)
			t.Setenv(test.key, test.value)
			_, err := webhookLoadConfigFromEnv()
			if err == nil || !strings.Contains(err.Error(), test.key) {
				t.Fatalf("config error=%v want key %s", err, test.key)
			}
		})
	}
}

func TestWebhookLoadAggregateAndReportUseMedianComparisonsExactly(t *testing.T) {
	results := make([]webhookWriteResult, 0, len(webhookLoadModes)*3)
	rpsByMode := [][]float64{
		{90, 100, 110},
		{85, 95, 105},
		{80, 90, 100},
		{70, 80, 90},
	}
	p95ByMode := [][]time.Duration{
		{9 * time.Millisecond, 10 * time.Millisecond, 11 * time.Millisecond},
		{10 * time.Millisecond, 11 * time.Millisecond, 12 * time.Millisecond},
		{11 * time.Millisecond, 12 * time.Millisecond, 13 * time.Millisecond},
		{14 * time.Millisecond, 15 * time.Millisecond, 16 * time.Millisecond},
	}
	for modeIndex, mode := range webhookLoadModes {
		for repeat := 0; repeat < 3; repeat++ {
			events := 0
			delivered := 0
			if mode.Subscribed {
				events = 100
			}
			if mode.WorkerRunning {
				delivered = 20 + repeat
			}
			results = append(results, webhookWriteResult{
				Mode:           mode,
				Repeat:         repeat + 1,
				ExecutionOrder: modeIndex + 1,
				Summary: stepSummary{
					Total:      100,
					Success:    100,
					SuccessRPS: rpsByMode[modeIndex][repeat],
					P50:        p95ByMode[modeIndex][repeat] - time.Millisecond,
					P95:        p95ByMode[modeIndex][repeat],
					P99:        p95ByMode[modeIndex][repeat] + time.Millisecond,
					FailurePct: 0,
				},
				EventRows:         events,
				SucceededDelivery: delivered,
			})
		}
	}

	aggregates := aggregateWebhookModes(results)
	if len(aggregates) != 4 || aggregates[0].MedianRPS != 100 || aggregates[0].MedianP95 != 10*time.Millisecond {
		t.Fatalf("baseline aggregate=%#v", aggregates)
	}
	if aggregates[2].MedianRPS != 90 || aggregates[2].MedianEventRatio != 1 {
		t.Fatalf("enqueue aggregate=%#v", aggregates[2])
	}
	if aggregates[3].MedianRPS != 80 || aggregates[3].MedianDeliveries != 21 {
		t.Fatalf("worker aggregate=%#v", aggregates[3])
	}

	report := webhookLoadReport(
		webhookLoadConfig{Concurrency: 50, Duration: time.Second, Repeats: 3, WorkerEvents: 100, MaxDBConns: 20},
		results,
		webhookWorkerResult{
			Events:            100,
			DispatchBatches:   1,
			DispatchDuration:  200 * time.Millisecond,
			DeliveryBatches:   2,
			DeliveryDuration:  400 * time.Millisecond,
			ReceiverRequests:  100,
			SucceededDelivery: 100,
		},
		time.Date(2026, time.August, 8, 16, 0, 0, 0, time.FixedZone("CST", 8*60*60)),
	)
	for _, fragment := range []string{
		"- 生成时间：`2026-08-08T16:00:00+08:00`",
		"- 预热：正式轮次前以关闭触发器模式运行最多 `250ms`，不计入结果",
		"| 启用触发器，无订阅 | 95.0 | -5.0% | 11.0ms | +1.0ms | 0.00% | 0.000 | 0 |",
		"| 有订阅，仅写 outbox | 90.0 | -10.0% | 12.0ms | +2.0ms | 0.00% | 1.000 | 0 |",
		"| outbox 展开 | 100 | 1 | 200.0ms | 500.0 events/s |",
		"| HTTP 投递并落成功状态 | 100 | 2 | 400.0ms | 250.0 deliveries/s |",
		"接收端收到 `100` 个请求，数据库记录 `100` 个成功投递",
	} {
		if !strings.Contains(report, fragment) {
			t.Fatalf("report missing %q:\n%s", fragment, report)
		}
	}
}

func TestWebhookLoadReportPathUsesExactOverride(t *testing.T) {
	t.Setenv("WEBHOOK_LOADTEST_REPORT", "reports/custom-webhook-load.md")
	if got := webhookLoadReportPath(); got != "reports/custom-webhook-load.md" {
		t.Fatalf("webhook report path=%q", got)
	}
}

func clearWebhookLoadEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"WEBHOOK_LOADTEST_CONCURRENCY",
		"WEBHOOK_LOADTEST_DURATION",
		"WEBHOOK_LOADTEST_REPEATS",
		"WEBHOOK_LOADTEST_EVENTS",
		"WEBHOOK_LOADTEST_DB_MAX_CONNECTIONS",
	} {
		t.Setenv(name, "")
	}
}
