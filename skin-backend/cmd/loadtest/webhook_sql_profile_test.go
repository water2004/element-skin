package main

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	webhookservice "element-skin/backend/internal/service/webhook"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type webhookSQLPhaseKey struct{}
type webhookSQLTraceKey struct{}

type webhookSQLTraceStart struct {
	Phase   string
	Query   string
	Started time.Time
}

type webhookSQLTraceStat struct {
	Phase     string
	Query     string
	Calls     int
	Errors    int
	Total     time.Duration
	Average   time.Duration
	P95       time.Duration
	Maximum   time.Duration
	Durations []time.Duration
}

type webhookSQLTracer struct {
	mu    sync.Mutex
	stats map[string]*webhookSQLTraceStat
	now   func() time.Time
}

func TestWebhookWorkerSQLProfile(t *testing.T) {
	if os.Getenv("WEBHOOK_SQL_PROFILE_ENABLE") != "1" {
		t.Skip("set WEBHOOK_SQL_PROFILE_ENABLE=1 to profile webhook worker SQL")
	}
	events, err := webhookLoadPositiveInt("WEBHOOK_SQL_PROFILE_EVENTS", 1000, 100_000)
	if err != nil {
		t.Fatal(err)
	}

	db, _, redis := testutil.NewTestAppWithMaxConnectionsAndRedisTB(t, 20)
	var receiverRequests atomic.Int64
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		_, _ = io.Copy(io.Discard, request.Body)
		receiverRequests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(receiver.Close)

	seed := seedWebhookLoadData(t, db, 1, receiver.URL)
	tracer := &webhookSQLTracer{stats: make(map[string]*webhookSQLTraceStat)}
	workerDB := openWebhookLoadWorkerDB(t, db, tracer, suggestedWebhookWorkerMaxDBConns, &permissiondb.RedisPermCache{Store: redis})
	worker := webhookservice.Worker{
		DB:         workerDB,
		Config:     testutil.TestConfig(),
		HTTPClient: receiver.Client(),
		Now:        time.Now,
	}
	result := benchmarkWebhookWorker(t, db, worker, seed, events, &receiverRequests)
	report := webhookSQLProfileReport(events, result, tracer.Snapshot(), time.Now())
	path := webhookSQLProfileReportPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create webhook SQL profile directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(report), 0o644); err != nil {
		t.Fatalf("write webhook SQL profile: %v", err)
	}
}

func openWebhookLoadWorkerDB(t *testing.T, siteDB *database.DB, tracer pgx.QueryTracer, maxConnections int32, permissionCache permissiondb.PermissionCache) *database.DB {
	t.Helper()
	poolConfig, err := pgxpool.ParseConfig(siteDB.Pool.Config().ConnConfig.ConnString())
	if err != nil {
		t.Fatalf("parse isolated webhook worker database config: %v", err)
	}
	poolConfig.MaxConns = maxConnections
	poolConfig.ConnConfig.Tracer = tracer
	pool, err := pgxpool.NewWithConfig(t.Context(), poolConfig)
	if err != nil {
		t.Fatalf("open isolated webhook worker database pool: %v", err)
	}
	workerDB := database.New(pool)
	workerDB.Permissions.Cache = permissionCache
	t.Cleanup(workerDB.Close)
	return workerDB
}

func withWebhookSQLPhase(ctx context.Context, phase string) context.Context {
	return context.WithValue(ctx, webhookSQLPhaseKey{}, phase)
}

func (t *webhookSQLTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	phase, _ := ctx.Value(webhookSQLPhaseKey{}).(string)
	if phase == "" {
		phase = "unlabelled"
	}
	return context.WithValue(ctx, webhookSQLTraceKey{}, webhookSQLTraceStart{
		Phase:   phase,
		Query:   webhookSQLQueryName(data.SQL),
		Started: t.nowTime(),
	})
}

func (t *webhookSQLTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	started, ok := ctx.Value(webhookSQLTraceKey{}).(webhookSQLTraceStart)
	if !ok {
		return
	}
	duration := t.nowTime().Sub(started.Started)
	key := started.Phase + "\x00" + started.Query
	t.mu.Lock()
	defer t.mu.Unlock()
	stat := t.stats[key]
	if stat == nil {
		stat = &webhookSQLTraceStat{Phase: started.Phase, Query: started.Query}
		t.stats[key] = stat
	}
	stat.Calls++
	stat.Total += duration
	stat.Durations = append(stat.Durations, duration)
	if duration > stat.Maximum {
		stat.Maximum = duration
	}
	if data.Err != nil {
		stat.Errors++
	}
}

func (t *webhookSQLTracer) nowTime() time.Time {
	if t.now == nil {
		return time.Now()
	}
	return t.now()
}

func (t *webhookSQLTracer) Snapshot() []webhookSQLTraceStat {
	t.mu.Lock()
	defer t.mu.Unlock()
	rows := make([]webhookSQLTraceStat, 0, len(t.stats))
	for _, source := range t.stats {
		row := *source
		row.Durations = append([]time.Duration(nil), source.Durations...)
		sort.Slice(row.Durations, func(left, right int) bool { return row.Durations[left] < row.Durations[right] })
		row.Average = row.Total / time.Duration(row.Calls)
		row.P95 = row.Durations[int(math.Ceil(float64(len(row.Durations))*0.95))-1]
		rows = append(rows, row)
	}
	sort.Slice(rows, func(left, right int) bool {
		if rows[left].Phase != rows[right].Phase {
			return rows[left].Phase < rows[right].Phase
		}
		if rows[left].Total != rows[right].Total {
			return rows[left].Total > rows[right].Total
		}
		return rows[left].Query < rows[right].Query
	})
	return rows
}

func webhookSQLQueryName(sql string) string {
	normalized := strings.Join(strings.Fields(sql), " ")
	switch {
	case strings.Contains(normalized, "WITH completion_input AS"):
		return "expansions.complete_batch"
	case strings.Contains(normalized, "FROM webhook_events") && strings.Contains(normalized, "FOR UPDATE SKIP LOCKED"):
		return "events.claim_pending"
	case strings.Contains(normalized, "FROM webhook_endpoint_events AS subscription"):
		return "endpoints.list_subscribed"
	case strings.Contains(normalized, "FROM delegated_permission_grants AS grant_record") && strings.Contains(normalized, "owned_granted.permission_id"):
		return "grants.authorization_permission_state"
	case strings.Contains(normalized, "FROM delegated_clients AS client") && strings.Contains(normalized, "requested.permission_id"):
		return "permissions.client_requested_one"
	case strings.Contains(normalized, "FROM delegated_client_permissions WHERE client_id=$1"):
		return "permissions.client_requested"
	case strings.Contains(normalized, "FROM delegated_permission_grants WHERE user_id=$1"):
		return "grants.active_by_user_client"
	case strings.Contains(normalized, "SELECT EXISTS(SELECT 1 FROM permission_subjects"):
		return "permissions.ensure_user_subject"
	case strings.Contains(normalized, "FROM subject_roles sr"):
		return "permissions.subject_roles"
	case strings.Contains(normalized, "FROM subject_permission_overrides spo"):
		return "permissions.subject_overrides"
	case strings.Contains(normalized, "FROM session_permission_policies spp"):
		return "permissions.session_policy"
	case strings.Contains(normalized, "JOIN delegated_grant_permissions gp"):
		return "permissions.delegation_policy"
	case strings.Contains(normalized, "WITH picked AS") && strings.Contains(normalized, "FROM webhook_deliveries"):
		return "deliveries.claim_due"
	case strings.Contains(normalized, "UPDATE webhook_deliveries AS delivery") && strings.Contains(normalized, "input.lease_token"):
		return "deliveries.complete_batch"
	case strings.Contains(normalized, "UPDATE webhook_deliveries SET status='succeeded'"):
		return "deliveries.complete"
	case normalized == "begin":
		return "transaction.begin"
	case normalized == "commit":
		return "transaction.commit"
	case normalized == "rollback":
		return "transaction.rollback"
	}
	if len(normalized) > 80 {
		normalized = normalized[:80] + "..."
	}
	return "other: " + normalized
}

func webhookSQLProfileReport(events int, worker webhookWorkerResult, rows []webhookSQLTraceStat, generatedAt time.Time) string {
	var report strings.Builder
	fmt.Fprintf(&report, "# Webhook Worker SQL Profile\n\n")
	fmt.Fprintf(&report, "- 生成时间：`%s`\n", generatedAt.Format(time.RFC3339))
	fmt.Fprintf(&report, "- 命令：`WEBHOOK_SQL_PROFILE_ENABLE=1 go test ./cmd/loadtest -run TestWebhookWorkerSQLProfile -count=1 -v`\n")
	fmt.Fprintf(&report, "- 事件：`%d`；Worker 独立数据库连接池：`%d`；接收端：进程内零延迟 `204`\n", events, suggestedWebhookWorkerMaxDBConns)
	fmt.Fprintf(&report, "- 紧循环展开：`%s`；紧循环投递：`%s`；生产轮询端到端：`%s`\n", formatDuration(worker.DispatchDuration), formatDuration(worker.DeliveryDuration), formatDuration(worker.SustainedDuration))
	fmt.Fprintf(&report, "- 说明：累计 SQL 耗时按调用求和，并发查询可能重叠，因此不能直接等同于墙钟时间。\n\n")
	fmt.Fprintf(&report, "| 阶段 | SQL | 调用 | 累计 | 平均 | P95 | 最大 | 错误 |\n")
	fmt.Fprintf(&report, "| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, row := range rows {
		fmt.Fprintf(&report, "| `%s` | `%s` | %d | %s | %s | %s | %s | %d |\n",
			row.Phase,
			row.Query,
			row.Calls,
			formatDuration(row.Total),
			formatDuration(row.Average),
			formatDuration(row.P95),
			formatDuration(row.Maximum),
			row.Errors,
		)
	}
	return report.String()
}

func webhookSQLProfileReportPath() string {
	if path := strings.TrimSpace(os.Getenv("WEBHOOK_SQL_PROFILE_REPORT")); path != "" {
		return path
	}
	return filepath.Clean(filepath.Join("..", "..", "..", "reports", "webhook-worker-sql-profile.md"))
}
