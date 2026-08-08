package main

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestWebhookLoadConfigUsesBalancedDefaults(t *testing.T) {
	clearWebhookLoadEnv(t)
	got, err := webhookLoadConfigFromEnv()
	want := webhookLoadConfig{
		Concurrency:          50,
		Duration:             3 * time.Second,
		Repeats:              4,
		WorkerEvents:         1000,
		MaxDBConns:           20,
		WorkerMaxDBConns:     2,
		WorkerActiveInterval: 3 * time.Second,
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
	for repeat := range webhookLoadModes {
		positions := map[string]int{}
		for order := range webhookLoadModes {
			positions[webhookLoadModeFor(repeat, order).Name] = order
		}
		if distance := positions["enqueue-only"] - positions["worker-running"]; distance != 1 && distance != -1 {
			t.Fatalf("repeat=%d enqueue/worker positions=%v are not adjacent", repeat+1, positions)
		}
		if repeat%2 == 0 && positions["enqueue-only"] > positions["worker-running"] {
			t.Fatalf("repeat=%d expected enqueue before worker: %v", repeat+1, positions)
		}
		if repeat%2 == 1 && positions["worker-running"] > positions["enqueue-only"] {
			t.Fatalf("repeat=%d expected worker before enqueue: %v", repeat+1, positions)
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
	t.Setenv("WEBHOOK_LOADTEST_WORKER_DB_MAX_CONNECTIONS", "4")
	t.Setenv("WEBHOOK_LOADTEST_WORKER_ACTIVE_INTERVAL", "750ms")
	got, err := webhookLoadConfigFromEnv()
	want := webhookLoadConfig{
		Concurrency:          40,
		Duration:             2 * time.Second,
		Repeats:              5,
		WorkerEvents:         2500,
		MaxDBConns:           30,
		WorkerMaxDBConns:     4,
		WorkerActiveInterval: 750 * time.Millisecond,
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
		{name: "excess worker db pool", key: "WEBHOOK_LOADTEST_WORKER_DB_MAX_CONNECTIONS", value: "101"},
		{name: "invalid duration", key: "WEBHOOK_LOADTEST_DURATION", value: "61s"},
		{name: "invalid worker active interval", key: "WEBHOOK_LOADTEST_WORKER_ACTIVE_INTERVAL", value: "61s"},
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
	if aggregates[3].MedianRPS != 80 || math.Abs(aggregates[3].MedianRelativePct-(-20)) > 0.000001 || aggregates[3].MedianDeliveries != 21 {
		t.Fatalf("worker aggregate=%#v", aggregates[3])
	}

	report := webhookLoadReport(
		webhookLoadConfig{
			Concurrency:          50,
			Duration:             time.Second,
			Repeats:              3,
			WorkerEvents:         100,
			MaxDBConns:           20,
			WorkerMaxDBConns:     2,
			WorkerActiveInterval: 3 * time.Second,
		},
		results,
		webhookWorkerResult{
			Events:                    100,
			DispatchBatches:           1,
			DispatchDuration:          200 * time.Millisecond,
			DeliveryBatches:           2,
			DeliveryDuration:          400 * time.Millisecond,
			ReceiverRequests:          100,
			SucceededDelivery:         100,
			SustainedDuration:         2 * time.Second,
			SustainedReceiverRequests: 100,
			SustainedDelivery:         100,
		},
		time.Date(2026, time.August, 8, 16, 0, 0, 0, time.FixedZone("CST", 8*60*60)),
	)
	for _, fragment := range []string{
		"- 生成时间：`2026-08-08T16:00:00+08:00`",
		"主站连接池：`20`；独立 Worker 连接池：`2`",
		"Worker 调度：有工作批次间协作等待 `3s`；空闲轮询 `500ms`",
		"- 预热：正式轮次前以关闭触发器模式运行最多 `250ms`，不计入结果",
		"- 阶段隔离：每阶段前清空 outbox 并对测试 profile 表执行 `VACUUM ANALYZE`",
		"| 启用触发器，无订阅 | 95.0 | -5.0% | 11.0ms | +1.0ms | 0.00% | 0.000 | 0 |",
		"| 有订阅，仅写 outbox | 90.0 | -10.0% | 12.0ms | +2.0ms | 0.00% | 1.000 | 0 |",
		"`worker-running` 相对同轮 `enqueue-only` 的成功吞吐变化中位数为 `-11.1%`",
		"| outbox 展开（紧循环批处理能力） | 100 | 1 | 200.0ms | 500.0 events/s |",
		"| HTTP 投递并落成功状态（紧循环批处理能力） | 100 | 2 | 400.0ms | 250.0 deliveries/s |",
		"| 生产轮询循环端到端 | 100 | - | 2.00s | 50.0 events/s |",
		"生产轮询阶段分别为 `100` 和 `100`",
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

func TestWebhookSQLProfileNamesAndReportAreExact(t *testing.T) {
	queries := map[string]string{
		"WITH picked AS (SELECT id FROM webhook_events WHERE expanded_at IS NULL FOR UPDATE SKIP LOCKED) UPDATE webhook_events SET expansion_lease_until=$2":               "events.claim_pending",
		"WITH completion_input AS (SELECT event_id) UPDATE webhook_events SET expanded_at=$3 INSERT INTO webhook_deliveries":                                               "expansions.complete_batch",
		"SELECT endpoint.id FROM webhook_endpoint_events AS subscription JOIN webhook_endpoints AS endpoint":                                                               "endpoints.list_subscribed",
		"SELECT EXISTS (SELECT 1 FROM delegated_permission_grants AS grant_record JOIN delegated_grant_permissions AS owned_granted WHERE owned_granted.permission_id=$4)": "grants.authorization_permission_state",
		"SELECT EXISTS (SELECT 1 FROM delegated_clients AS client JOIN delegated_client_permissions AS requested WHERE requested.permission_id=$2)":                        "permissions.client_requested_one",
		"SELECT permission_id FROM delegated_client_permissions WHERE client_id=$1 ORDER BY permission_id":                                                                 "permissions.client_requested",
		"SELECT id FROM delegated_permission_grants WHERE user_id=$1 AND client_id=$2 AND status='active'":                                                                 "grants.active_by_user_client",
		"SELECT EXISTS(SELECT 1 FROM permission_subjects WHERE id=$1)":                                                                                                     "permissions.ensure_user_subject",
		"SELECT rp.permission_id FROM subject_roles sr JOIN role_permissions rp ON rp.role_id=sr.role_id":                                                                  "permissions.subject_roles",
		"SELECT spo.permission_id FROM subject_permission_overrides spo WHERE spo.subject_id=$1":                                                                           "permissions.subject_overrides",
		"SELECT spp.permission_id FROM session_permission_policies spp WHERE spp.session_kind=$1":                                                                          "permissions.session_policy",
		"SELECT gp.permission_id FROM delegated_permission_grants g JOIN delegated_grant_permissions gp ON TRUE":                                                           "permissions.delegation_policy",
		"WITH picked AS (SELECT id FROM webhook_deliveries) UPDATE webhook_deliveries SET status='processing'":                                                             "deliveries.claim_due",
		"UPDATE webhook_deliveries AS delivery SET status=input.status FROM unnest($1::TEXT[]) AS input WHERE delivery.lease_token=input.lease_token":                      "deliveries.complete_batch",
		"UPDATE webhook_deliveries SET status='succeeded' WHERE id=$1":                                                                                                     "deliveries.complete",
		"begin":                    "transaction.begin",
		"SELECT current_timestamp": "other: SELECT current_timestamp",
	}
	for query, want := range queries {
		if got := webhookSQLQueryName(query); got != want {
			t.Fatalf("query name=%q want=%q query=%q", got, want, query)
		}
	}

	report := webhookSQLProfileReport(
		100,
		webhookWorkerResult{
			DispatchDuration:  200 * time.Millisecond,
			DeliveryDuration:  400 * time.Millisecond,
			SustainedDuration: 2 * time.Second,
		},
		[]webhookSQLTraceStat{{
			Phase:   "tight-dispatch",
			Query:   "permissions.client_requested",
			Calls:   100,
			Total:   500 * time.Millisecond,
			Average: 5 * time.Millisecond,
			P95:     8 * time.Millisecond,
			Maximum: 10 * time.Millisecond,
		}},
		time.Date(2026, time.August, 8, 23, 30, 0, 0, time.FixedZone("CST", 8*60*60)),
	)
	for _, fragment := range []string{
		"- 生成时间：`2026-08-08T23:30:00+08:00`",
		"事件：`100`；Worker 独立数据库连接池：`2`",
		"| `tight-dispatch` | `permissions.client_requested` | 100 | 500.0ms | 5.0ms | 8.0ms | 10.0ms | 0 |",
	} {
		if !strings.Contains(report, fragment) {
			t.Fatalf("SQL profile report missing %q:\n%s", fragment, report)
		}
	}
}

func TestWebhookSQLTracerRecordsExactPhaseQueryAndError(t *testing.T) {
	times := []time.Time{
		time.Unix(0, 100*time.Millisecond.Nanoseconds()),
		time.Unix(0, 107*time.Millisecond.Nanoseconds()),
	}
	timeIndex := 0
	tracer := &webhookSQLTracer{
		stats: make(map[string]*webhookSQLTraceStat),
		now: func() time.Time {
			value := times[timeIndex]
			timeIndex++
			return value
		},
	}
	ctx := withWebhookSQLPhase(context.Background(), "profile-phase")
	ctx = tracer.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{
		SQL: "SELECT permission_id FROM delegated_client_permissions WHERE client_id=$1",
	})
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: errors.New("profile error")})

	rows := tracer.Snapshot()
	if len(rows) != 1 {
		t.Fatalf("SQL trace rows=%#v want exactly one", rows)
	}
	row := rows[0]
	if row.Phase != "profile-phase" || row.Query != "permissions.client_requested" || row.Calls != 1 || row.Errors != 1 {
		t.Fatalf("SQL trace row=%#v", row)
	}
	if row.Total != 7*time.Millisecond || row.Average != row.Total || row.P95 != row.Total || row.Maximum != row.Total {
		t.Fatalf("SQL trace durations=%#v", row)
	}
}

func TestWebhookSQLProfileReportPathUsesExactOverride(t *testing.T) {
	t.Setenv("WEBHOOK_SQL_PROFILE_REPORT", "reports/custom-webhook-sql-profile.md")
	if got := webhookSQLProfileReportPath(); got != "reports/custom-webhook-sql-profile.md" {
		t.Fatalf("webhook SQL profile path=%q", got)
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
		"WEBHOOK_LOADTEST_WORKER_DB_MAX_CONNECTIONS",
		"WEBHOOK_LOADTEST_WORKER_ACTIVE_INTERVAL",
	} {
		t.Setenv(name, "")
	}
}
