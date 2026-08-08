package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	webhookservice "element-skin/backend/internal/service/webhook"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

const (
	webhookLoadClientID   = "webhook-load-client"
	webhookLoadEndpointID = "webhook-load-endpoint"
	webhookLoadGrantID    = "webhook-load-grant"
)

type webhookLoadConfig struct {
	Concurrency  int
	Duration     time.Duration
	Repeats      int
	WorkerEvents int
	MaxDBConns   int
}

type webhookLoadMode struct {
	Name          string
	Label         string
	Trigger       bool
	Subscribed    bool
	WorkerRunning bool
}

type webhookWriteResult struct {
	Mode              webhookLoadMode
	Repeat            int
	ExecutionOrder    int
	Summary           stepSummary
	EventRows         int
	SucceededDelivery int
}

type webhookWorkerResult struct {
	Events            int
	DispatchBatches   int
	DispatchDuration  time.Duration
	DeliveryBatches   int
	DeliveryDuration  time.Duration
	ReceiverRequests  int64
	SucceededDelivery int
}

type webhookLoadSeed struct {
	User       model.User
	ProfileIDs []string
}

type webhookModeAggregate struct {
	Mode             webhookLoadMode
	MedianRPS        float64
	MedianP95        time.Duration
	MedianFailurePct float64
	MedianEventRatio float64
	MedianDeliveries float64
}

var webhookLoadModes = []webhookLoadMode{
	{Name: "trigger-disabled", Label: "关闭触发器（功能前基线）"},
	{Name: "no-subscriber", Label: "启用触发器，无订阅", Trigger: true},
	{Name: "enqueue-only", Label: "有订阅，仅写 outbox", Trigger: true, Subscribed: true},
	{Name: "worker-running", Label: "有订阅，worker 同时运行", Trigger: true, Subscribed: true, WorkerRunning: true},
}

func TestWebhookLoadImpact(t *testing.T) {
	if os.Getenv("WEBHOOK_LOADTEST_ENABLE") != "1" {
		t.Skip("set WEBHOOK_LOADTEST_ENABLE=1 to run the isolated webhook load test")
	}
	loadConfig, err := webhookLoadConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	db, handler, _ := testutil.NewTestAppWithMaxConnectionsAndRedisTB(t, int32(loadConfig.MaxDBConns))
	loadConfig.MaxDBConns = int(db.Pool.Stat().MaxConns())
	if err := db.Settings.Set(t.Context(), "rate_limit_enabled", false); err != nil {
		t.Fatalf("disable load-test auth rate limit: %v", err)
	}

	var receiverRequests atomic.Int64
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		_, _ = io.Copy(io.Discard, request.Body)
		receiverRequests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(receiver.Close)

	seed := seedWebhookLoadData(t, db, loadConfig.Concurrency, receiver.URL)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	loginClient := newHTTPClient(1, 5*time.Second, false)
	cookie, err := login(loginClient, server.URL, "/v2/auth/login", seed.User.Email, "Password123")
	loginClient.CloseIdleConnections()
	if err != nil {
		t.Fatalf("login webhook load-test user: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.Pool.Exec(context.Background(), `ALTER TABLE profiles ENABLE TRIGGER profiles_webhook_event`)
	})
	worker := webhookservice.Worker{
		DB:         db,
		Config:     testutil.TestConfig(),
		HTTPClient: receiver.Client(),
		Now:        time.Now,
	}
	profileStates := make([]bool, loadConfig.Concurrency)
	warmupMode := webhookLoadModes[0]
	if err := configureWebhookLoadMode(t.Context(), db, warmupMode); err != nil {
		t.Fatalf("configure warmup: %v", err)
	}
	warmupClient := newHTTPClient(loadConfig.Concurrency, 5*time.Second, false)
	warmupSummary := runWebhookWriteStep(
		warmupClient,
		server.URL,
		cookie,
		seed.ProfileIDs,
		profileStates,
		loadConfig.Concurrency,
		min(loadConfig.Duration, 250*time.Millisecond),
	)
	warmupClient.CloseIdleConnections()
	assertWebhookWriteResult(t, warmupMode, warmupSummary, 0, 0, receiverRequests.Load())

	writeResults := make([]webhookWriteResult, 0, loadConfig.Repeats*len(webhookLoadModes))
	for repeat := 0; repeat < loadConfig.Repeats; repeat++ {
		for order := 0; order < len(webhookLoadModes); order++ {
			mode := webhookLoadModeFor(repeat, order)
			if err := configureWebhookLoadMode(t.Context(), db, mode); err != nil {
				t.Fatalf("configure mode %s: %v", mode.Name, err)
			}
			beforeReceiver := receiverRequests.Load()
			stopWorker, workerErrors := startWebhookLoadWorker(t.Context(), worker, mode.WorkerRunning)
			client := newHTTPClient(loadConfig.Concurrency, 5*time.Second, false)
			summary := runWebhookWriteStep(
				client,
				server.URL,
				cookie,
				seed.ProfileIDs,
				profileStates,
				loadConfig.Concurrency,
				loadConfig.Duration,
			)
			client.CloseIdleConnections()
			stopWorker()
			if err := <-workerErrors; err != nil {
				t.Fatalf("mode %s worker: %v", mode.Name, err)
			}
			eventRows, succeededDelivery, err := webhookLoadCounts(t.Context(), db)
			if err != nil {
				t.Fatalf("mode %s counts: %v", mode.Name, err)
			}
			assertWebhookWriteResult(t, mode, summary, eventRows, succeededDelivery, receiverRequests.Load()-beforeReceiver)
			result := webhookWriteResult{
				Mode:              mode,
				Repeat:            repeat + 1,
				ExecutionOrder:    order + 1,
				Summary:           summary,
				EventRows:         eventRows,
				SucceededDelivery: succeededDelivery,
			}
			writeResults = append(writeResults, result)
			t.Logf(
				"mode=%s repeat=%d order=%d requests=%d success_rps=%.1f p95=%s events=%d delivered=%d",
				mode.Name,
				result.Repeat,
				result.ExecutionOrder,
				summary.Total,
				summary.SuccessRPS,
				formatDuration(summary.P95),
				eventRows,
				succeededDelivery,
			)
		}
	}

	workerResult := benchmarkWebhookWorker(t, db, worker, seed, loadConfig.WorkerEvents, &receiverRequests)
	report := webhookLoadReport(loadConfig, writeResults, workerResult, time.Now())
	if err := os.MkdirAll(filepath.Dir(webhookLoadReportPath()), 0o755); err != nil {
		t.Fatalf("create webhook report directory: %v", err)
	}
	if err := os.WriteFile(webhookLoadReportPath(), []byte(report), 0o644); err != nil {
		t.Fatalf("write webhook load report: %v", err)
	}
}

func webhookLoadModeFor(repeat, order int) webhookLoadMode {
	return webhookLoadModes[(repeat+order)%len(webhookLoadModes)]
}

func webhookLoadConfigFromEnv() (webhookLoadConfig, error) {
	concurrency, err := webhookLoadPositiveInt("WEBHOOK_LOADTEST_CONCURRENCY", 50, 500)
	if err != nil {
		return webhookLoadConfig{}, err
	}
	repeats, err := webhookLoadPositiveInt("WEBHOOK_LOADTEST_REPEATS", 4, 20)
	if err != nil {
		return webhookLoadConfig{}, err
	}
	workerEvents, err := webhookLoadPositiveInt("WEBHOOK_LOADTEST_EVENTS", 1000, 100_000)
	if err != nil {
		return webhookLoadConfig{}, err
	}
	maxDBConns, err := webhookLoadPositiveInt("WEBHOOK_LOADTEST_DB_MAX_CONNECTIONS", 20, 500)
	if err != nil {
		return webhookLoadConfig{}, err
	}
	duration := time.Second
	if raw := strings.TrimSpace(os.Getenv("WEBHOOK_LOADTEST_DURATION")); raw != "" {
		duration, err = time.ParseDuration(raw)
		if err != nil || duration <= 0 || duration > time.Minute {
			return webhookLoadConfig{}, fmt.Errorf("invalid WEBHOOK_LOADTEST_DURATION %q", raw)
		}
	}
	return webhookLoadConfig{
		Concurrency:  concurrency,
		Duration:     duration,
		Repeats:      repeats,
		WorkerEvents: workerEvents,
		MaxDBConns:   maxDBConns,
	}, nil
}

func webhookLoadPositiveInt(name string, fallback, maximum int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 || value > maximum {
		return 0, fmt.Errorf("invalid %s %q", name, raw)
	}
	return value, nil
}

func seedWebhookLoadData(t *testing.T, db *database.DB, concurrency int, endpointURL string) webhookLoadSeed {
	t.Helper()
	ctx := t.Context()
	seed := webhookLoadSeed{
		User:       testutil.CreateUser(t, db, "webhook-load@example.com", "Password123", "WebhookLoad", false),
		ProfileIDs: make([]string, 0, concurrency),
	}
	for index := 0; index < concurrency; index++ {
		profile := testutil.CreateProfile(t, db, seed.User.ID, "", fmt.Sprintf("WL%04d", index))
		seed.ProfileIDs = append(seed.ProfileIDs, profile.ID)
	}

	loadConfig := testutil.TestConfig()
	box, err := util.NewSecretBox(loadConfig.IdentityEncryptionKey)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := box.Encrypt("webhook-load-signing-secret")
	if err != nil {
		t.Fatal(err)
	}
	now := database.NowMS()
	readPermission := permission.MustDefinitionByCode("profile.read.owned")
	client := model.OAuthClient{
		ID:          webhookLoadClientID,
		OwnerUserID: seed.User.ID,
		Name:        "Webhook Load Client",
		ClientType:  "confidential",
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	endpoint := model.WebhookEndpoint{
		ID:               webhookLoadEndpointID,
		ClientID:         client.ID,
		URL:              endpointURL,
		SecretCiphertext: ciphertext,
		Status:           "active",
		EventTypes:       []string{"profile.updated"},
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	permissionIDs := []int64{int64(readPermission.ID)}
	if err := db.OAuth.CreateClient(ctx, client, permissionIDs, []model.WebhookEndpoint{endpoint}); err != nil {
		t.Fatalf("seed webhook load client: %v", err)
	}
	grant := model.OAuthGrant{
		ID:        webhookLoadGrantID,
		UserID:    seed.User.ID,
		SubjectID: permissiondb.SubjectIDForUser(seed.User.ID),
		ClientID:  client.ID,
		Status:    "active",
		CreatedAt: now,
	}
	if err := db.OAuth.CreateGrant(ctx, grant, permissionIDs); err != nil {
		t.Fatalf("seed webhook load grant: %v", err)
	}
	return seed
}

func configureWebhookLoadMode(ctx context.Context, db *database.DB, mode webhookLoadMode) error {
	triggerState := "DISABLE"
	if mode.Trigger {
		triggerState = "ENABLE"
	}
	if _, err := db.Pool.Exec(ctx, "ALTER TABLE profiles "+triggerState+" TRIGGER profiles_webhook_event"); err != nil {
		return err
	}
	if _, err := db.Pool.Exec(ctx, `DELETE FROM webhook_events`); err != nil {
		return err
	}
	if _, err := db.Pool.Exec(ctx, `DELETE FROM webhook_endpoint_events WHERE endpoint_id=$1`, webhookLoadEndpointID); err != nil {
		return err
	}
	if mode.Subscribed {
		_, err := db.Pool.Exec(ctx, `
			INSERT INTO webhook_endpoint_events (endpoint_id,event_type,created_at)
			VALUES ($1,'profile.updated',$2)
		`, webhookLoadEndpointID, database.NowMS())
		return err
	}
	return nil
}

func runWebhookWriteStep(client *http.Client, baseURL, cookie string, profileIDs []string, profileStates []bool, concurrency int, duration time.Duration) stepSummary {
	results := make(chan requestResult, concurrency*32)
	var wait sync.WaitGroup
	start := time.Now()
	end := start.Add(duration)
	for workerIndex := 0; workerIndex < concurrency; workerIndex++ {
		workerIndex := workerIndex
		wait.Add(1)
		go func() {
			defer wait.Done()
			target := baseURL + "/v2/users/me/profiles/" + profileIDs[workerIndex]
			for time.Now().Before(end) {
				profileStates[workerIndex] = !profileStates[workerIndex]
				suffix := "A"
				if profileStates[workerIndex] {
					suffix = "B"
				}
				opts := options{
					method:      http.MethodPatch,
					body:        fmt.Sprintf(`{"name":"WL%04d%s"}`, workerIndex, suffix),
					contentType: "application/json",
				}
				results <- doRequest(client, target, opts, cookie)
			}
		}()
	}
	go func() {
		wait.Wait()
		close(results)
	}()
	collected := make([]requestResult, 0, concurrency*128)
	for result := range results {
		collected = append(collected, result)
	}
	return summarize(concurrency, collected, time.Since(start))
}

func startWebhookLoadWorker(ctx context.Context, worker webhookservice.Worker, enabled bool) (func(), <-chan error) {
	errors := make(chan error, 1)
	if !enabled {
		errors <- nil
		close(errors)
		return func() {}, errors
	}
	stop := make(chan struct{})
	var once sync.Once
	go func() {
		defer close(errors)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			if err := worker.RunOnce(ctx); err != nil {
				errors <- err
				return
			}
			select {
			case <-stop:
				errors <- nil
				return
			case <-ctx.Done():
				errors <- ctx.Err()
				return
			case <-ticker.C:
			}
		}
	}()
	return func() { once.Do(func() { close(stop) }) }, errors
}

func webhookLoadCounts(ctx context.Context, db *database.DB) (int, int, error) {
	var events int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM webhook_events`).Scan(&events); err != nil {
		return 0, 0, err
	}
	var succeeded int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM webhook_deliveries WHERE status='succeeded'`).Scan(&succeeded); err != nil {
		return 0, 0, err
	}
	return events, succeeded, nil
}

func assertWebhookWriteResult(t *testing.T, mode webhookLoadMode, summary stepSummary, eventRows, succeededDelivery int, receiverRequests int64) {
	t.Helper()
	if summary.Total == 0 || summary.Success != summary.Total || summary.Failed != 0 || summary.Statuses[http.StatusNoContent] != summary.Total {
		t.Fatalf("mode %s request summary mismatch: %#v", mode.Name, summary)
	}
	if mode.Subscribed {
		if eventRows != summary.Success {
			t.Fatalf("mode %s event rows=%d want successful requests=%d", mode.Name, eventRows, summary.Success)
		}
	} else if eventRows != 0 {
		t.Fatalf("mode %s event rows=%d want=0", mode.Name, eventRows)
	}
	if !mode.WorkerRunning && (succeededDelivery != 0 || receiverRequests != 0) {
		t.Fatalf("mode %s delivered=%d receiver_requests=%d want=0", mode.Name, succeededDelivery, receiverRequests)
	}
	if mode.WorkerRunning && (succeededDelivery != int(receiverRequests) || succeededDelivery > eventRows) {
		t.Fatalf("mode %s delivered=%d receiver_requests=%d events=%d", mode.Name, succeededDelivery, receiverRequests, eventRows)
	}
}

func benchmarkWebhookWorker(t *testing.T, db *database.DB, worker webhookservice.Worker, seed webhookLoadSeed, total int, receiverRequests *atomic.Int64) webhookWorkerResult {
	t.Helper()
	mode := webhookLoadMode{Trigger: true, Subscribed: true}
	if err := configureWebhookLoadMode(t.Context(), db, mode); err != nil {
		t.Fatal(err)
	}
	prefix := fmt.Sprintf("evt_load_%d_", time.Now().UnixNano())
	now := database.NowMS()
	if _, err := db.Pool.Exec(t.Context(), `
		INSERT INTO webhook_events (id,event_type,subject_user_id,data,created_at)
		SELECT $1::TEXT || item::TEXT,
		       'profile.updated',
		       $2::TEXT,
		       jsonb_build_object('user_id',$2::TEXT,'profile_id',$3::TEXT),
		       $4::BIGINT + item
		FROM generate_series(1,$5::INT) AS item
	`, prefix, seed.User.ID, seed.ProfileIDs[0], now, total); err != nil {
		t.Fatalf("seed worker events: %v", err)
	}

	dispatchStart := time.Now()
	dispatchBatches := drainWebhookDispatch(t, db, worker)
	dispatchDuration := time.Since(dispatchStart)
	var deliveries int
	if err := db.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM webhook_deliveries`).Scan(&deliveries); err != nil {
		t.Fatal(err)
	}
	if deliveries != total {
		t.Fatalf("worker deliveries=%d want=%d", deliveries, total)
	}

	beforeReceiver := receiverRequests.Load()
	deliveryStart := time.Now()
	deliveryBatches := drainWebhookDeliveries(t, db, worker, total)
	deliveryDuration := time.Since(deliveryStart)
	received := receiverRequests.Load() - beforeReceiver
	_, succeeded, err := webhookLoadCounts(t.Context(), db)
	if err != nil {
		t.Fatal(err)
	}
	if succeeded != total || received != int64(total) {
		t.Fatalf("worker succeeded=%d received=%d want=%d", succeeded, received, total)
	}
	return webhookWorkerResult{
		Events:            total,
		DispatchBatches:   dispatchBatches,
		DispatchDuration:  dispatchDuration,
		DeliveryBatches:   deliveryBatches,
		DeliveryDuration:  deliveryDuration,
		ReceiverRequests:  received,
		SucceededDelivery: succeeded,
	}
}

func drainWebhookDispatch(t *testing.T, db *database.DB, worker webhookservice.Worker) int {
	t.Helper()
	batches := 0
	for {
		var before int
		if err := db.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM webhook_events WHERE expanded_at IS NULL`).Scan(&before); err != nil {
			t.Fatal(err)
		}
		if before == 0 {
			return batches
		}
		if err := worker.DispatchBatch(t.Context()); err != nil {
			t.Fatal(err)
		}
		batches++
		var after int
		if err := db.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM webhook_events WHERE expanded_at IS NULL`).Scan(&after); err != nil {
			t.Fatal(err)
		}
		if after >= before {
			t.Fatalf("dispatch made no progress: before=%d after=%d", before, after)
		}
	}
}

func drainWebhookDeliveries(t *testing.T, db *database.DB, worker webhookservice.Worker, total int) int {
	t.Helper()
	batches := 0
	for {
		var succeeded int
		if err := db.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM webhook_deliveries WHERE status='succeeded'`).Scan(&succeeded); err != nil {
			t.Fatal(err)
		}
		if succeeded == total {
			return batches
		}
		if err := worker.DeliverBatch(t.Context()); err != nil {
			t.Fatal(err)
		}
		batches++
		var nextSucceeded int
		if err := db.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM webhook_deliveries WHERE status='succeeded'`).Scan(&nextSucceeded); err != nil {
			t.Fatal(err)
		}
		if nextSucceeded <= succeeded {
			t.Fatalf("delivery made no progress: before=%d after=%d", succeeded, nextSucceeded)
		}
	}
}

func aggregateWebhookModes(results []webhookWriteResult) []webhookModeAggregate {
	aggregates := make([]webhookModeAggregate, 0, len(webhookLoadModes))
	for _, mode := range webhookLoadModes {
		var rpsValues []float64
		var p95Values []time.Duration
		var failureValues []float64
		var eventRatios []float64
		var deliveries []float64
		for _, result := range results {
			if result.Mode.Name != mode.Name {
				continue
			}
			rpsValues = append(rpsValues, result.Summary.SuccessRPS)
			p95Values = append(p95Values, result.Summary.P95)
			failureValues = append(failureValues, result.Summary.FailurePct)
			ratio := 0.0
			if result.Summary.Success > 0 {
				ratio = float64(result.EventRows) / float64(result.Summary.Success)
			}
			eventRatios = append(eventRatios, ratio)
			deliveries = append(deliveries, float64(result.SucceededDelivery))
		}
		aggregates = append(aggregates, webhookModeAggregate{
			Mode:             mode,
			MedianRPS:        medianFloat(rpsValues),
			MedianP95:        medianDuration(p95Values),
			MedianFailurePct: medianFloat(failureValues),
			MedianEventRatio: medianFloat(eventRatios),
			MedianDeliveries: medianFloat(deliveries),
		})
	}
	return aggregates
}

func medianFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / 2
}

func medianDuration(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(left, right int) bool { return sorted[left] < sorted[right] })
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / 2
}

func webhookLoadReport(loadConfig webhookLoadConfig, writeResults []webhookWriteResult, worker webhookWorkerResult, generatedAt time.Time) string {
	aggregates := aggregateWebhookModes(writeResults)
	baseline := aggregates[0]
	var report strings.Builder
	fmt.Fprintf(&report, "# Webhook 性能影响压测报告\n\n")
	fmt.Fprintf(&report, "- 生成时间：`%s`\n", generatedAt.Format(time.RFC3339))
	fmt.Fprintf(&report, "- 命令：`WEBHOOK_LOADTEST_ENABLE=1 go test ./cmd/loadtest -run TestWebhookLoadImpact -count=1 -v`\n")
	fmt.Fprintf(&report, "- 写请求：`PATCH /v2/users/me/profiles/{profile_id}`，每个并发 worker 独占一个 profile 并交替修改名称\n")
	fmt.Fprintf(&report, "- 并发：`%d`；每阶段时长：`%s`；重复：`%d`；数据库连接池：`%d`\n", loadConfig.Concurrency, loadConfig.Duration, loadConfig.Repeats, loadConfig.MaxDBConns)
	fmt.Fprintf(&report, "- Worker 固定事件数：`%d`；接收端：进程内零延迟 `204` HTTP server\n", loadConfig.WorkerEvents)
	fmt.Fprintf(&report, "- 数据隔离：临时 PostgreSQL 数据库和独立 Redis prefix，测试结束自动清理\n")
	fmt.Fprintf(&report, "- 预热：正式轮次前以关闭触发器模式运行最多 `250ms`，不计入结果\n\n")

	fmt.Fprintf(&report, "## 主站写请求影响\n\n")
	fmt.Fprintf(&report, "| 模式 | 中位成功 req/s | 相对基线 | 中位 P95 | P95 增量 | 失败率 | event/成功请求 | worker 阶段内中位成功投递 |\n")
	fmt.Fprintf(&report, "| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, aggregate := range aggregates {
		fmt.Fprintf(&report, "| %s | %.1f | %+.1f%% | %s | %s | %.2f%% | %.3f | %.0f |\n",
			aggregate.Mode.Label,
			aggregate.MedianRPS,
			percentChange(aggregate.MedianRPS, baseline.MedianRPS),
			formatDuration(aggregate.MedianP95),
			formatSignedDuration(aggregate.MedianP95-baseline.MedianP95),
			aggregate.MedianFailurePct,
			aggregate.MedianEventRatio,
			aggregate.MedianDeliveries,
		)
	}

	fmt.Fprintf(&report, "\n## 原始写请求结果\n\n")
	fmt.Fprintf(&report, "每轮旋转模式执行顺序，降低固定顺序带来的缓存和温度偏差。\n\n")
	fmt.Fprintf(&report, "| 重复 | 执行顺序 | 模式 | 请求 | 成功 req/s | P50 | P95 | P99 | event | 成功投递 |\n")
	fmt.Fprintf(&report, "| ---: | ---: | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, result := range writeResults {
		fmt.Fprintf(&report, "| %d | %d | `%s` | %d | %.1f | %s | %s | %s | %d | %d |\n",
			result.Repeat,
			result.ExecutionOrder,
			result.Mode.Name,
			result.Summary.Total,
			result.Summary.SuccessRPS,
			formatDuration(result.Summary.P50),
			formatDuration(result.Summary.P95),
			formatDuration(result.Summary.P99),
			result.EventRows,
			result.SucceededDelivery,
		)
	}

	dispatchRate := ratePerSecond(worker.Events, worker.DispatchDuration)
	deliveryRate := ratePerSecond(worker.Events, worker.DeliveryDuration)
	endToEnd := worker.DispatchDuration + worker.DeliveryDuration
	fmt.Fprintf(&report, "\n## 独立 Worker 吞吐\n\n")
	fmt.Fprintf(&report, "| 阶段 | 事件 | 批次 | 耗时 | 吞吐 |\n")
	fmt.Fprintf(&report, "| --- | ---: | ---: | ---: | ---: |\n")
	fmt.Fprintf(&report, "| outbox 展开 | %d | %d | %s | %.1f events/s |\n", worker.Events, worker.DispatchBatches, formatDuration(worker.DispatchDuration), dispatchRate)
	fmt.Fprintf(&report, "| HTTP 投递并落成功状态 | %d | %d | %s | %.1f deliveries/s |\n", worker.Events, worker.DeliveryBatches, formatDuration(worker.DeliveryDuration), deliveryRate)
	fmt.Fprintf(&report, "| 展开加投递 | %d | %d | %s | %.1f events/s |\n", worker.Events, worker.DispatchBatches+worker.DeliveryBatches, formatDuration(endToEnd), ratePerSecond(worker.Events, endToEnd))
	fmt.Fprintf(&report, "\n接收端收到 `%d` 个请求，数据库记录 `%d` 个成功投递，两者与固定事件数完全一致。\n", worker.ReceiverRequests, worker.SucceededDelivery)

	fmt.Fprintf(&report, "\n## 如何解读\n\n")
	fmt.Fprintf(&report, "- “关闭触发器”近似表示没有 Webhook 功能时的写路径；“无订阅”测量每次业务写入执行索引化订阅存在性检查的固定成本。\n")
	fmt.Fprintf(&report, "- “仅写 outbox”包含存在订阅时在业务事务内新增一条不可变事件快照的成本，不包含外部 HTTP。\n")
	fmt.Fprintf(&report, "- “worker 同时运行”额外反映异步 worker 与主站共享 PostgreSQL 时的连接池和 I/O 竞争，但回调仍不会占用主站请求 goroutine。\n")
	fmt.Fprintf(&report, "- Worker 的 HTTP 数字使用本机零延迟接收端，表示站点自身上限；真实吞吐会受第三方网络延迟、TLS 和接收方响应时间影响。\n")
	fmt.Fprintf(&report, "- 单机短窗口结果用于比较相对变化，不应直接作为生产 SLA；容量规划应在接近生产的数据库、网络和 endpoint 延迟下复测。\n")
	return report.String()
}

func percentChange(value, baseline float64) float64 {
	if baseline == 0 {
		return 0
	}
	return (value/baseline - 1) * 100
}

func formatSignedDuration(value time.Duration) string {
	if value > -50*time.Microsecond && value < 50*time.Microsecond {
		return "0ms"
	}
	sign := "+"
	if value < 0 {
		sign = "-"
		value = -value
	}
	return sign + formatDuration(value)
}

func ratePerSecond(total int, duration time.Duration) float64 {
	if duration <= 0 {
		return 0
	}
	return float64(total) / duration.Seconds()
}

func webhookLoadReportPath() string {
	if path := strings.TrimSpace(os.Getenv("WEBHOOK_LOADTEST_REPORT")); path != "" {
		return path
	}
	return filepath.Clean(filepath.Join("..", "..", "..", "reports", "webhook-load-test.md"))
}
