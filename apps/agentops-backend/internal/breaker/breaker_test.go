package breaker_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/breaker"
)

func setup(t *testing.T, openTimeout time.Duration) (*breaker.Breaker, context.Context, string) {
	t.Helper()

	rdbAddr := os.Getenv("CHRONO_REDIS_ADDR")
	if rdbAddr == "" {
		rdbAddr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: rdbAddr})
	t.Cleanup(func() { _ = rdb.Close() })

	dbURL := os.Getenv("CHRONO_TEST_DB_URL")
	if dbURL == "" {
		t.Skip("CHRONO_TEST_DB_URL not set")
	}
	database, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	config := breaker.Config{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		OpenTimeout:      openTimeout,
	}

	// Unique target per test run to avoid state leakage between runs.
	target := "test/model-" + t.Name() + "-" + time.Now().Format("150405.000000000")

	ctx := context.Background()
	// Clean any residual state (should not exist, but defensive).
	rdb.Del(ctx, "breaker:model:"+target+":state")
	rdb.Del(ctx, "breaker:model:"+target+":opened_at")
	rdb.Del(ctx, "breaker:model:"+target+":failures")
	rdb.Del(ctx, "breaker:model:"+target+":successes")

	return breaker.New(rdb, database, config), ctx, target
}

func TestBreakerTripsAfterConsecutiveFailures(t *testing.T) {
	b, ctx, target := setup(t, 10*time.Second)

	// Initially CLOSED.
	state, err := b.State(ctx, breaker.TargetModel, target)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if state != breaker.StateClosed {
		t.Fatalf("expected closed, got %s", state)
	}

	// Three failures trip to OPEN.
	for i := 0; i < 3; i++ {
		allowed, err := b.Allow(ctx, breaker.TargetModel, target)
		if err != nil {
			t.Fatalf("allow %d: %v", i, err)
		}
		if !allowed {
			t.Fatalf("request %d should be allowed in closed state", i)
		}
		if err := b.RecordResult(ctx, breaker.TargetModel, target, false); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
	}

	state, err = b.State(ctx, breaker.TargetModel, target)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if state != breaker.StateOpen {
		t.Fatalf("expected open after 3 failures, got %s", state)
	}

	// Requests must be rejected while OPEN and within OpenTimeout.
	allowed, err := b.Allow(ctx, breaker.TargetModel, target)
	if err != nil {
		t.Fatalf("allow after open: %v", err)
	}
	if allowed {
		t.Fatal("requests should be rejected in open state")
	}
}

func TestBreakerTransitionsToHalfOpenAfterTimeout(t *testing.T) {
	b, ctx, target := setup(t, 50*time.Millisecond)

	for i := 0; i < 3; i++ {
		_, _ = b.Allow(ctx, breaker.TargetModel, target)
		_ = b.RecordResult(ctx, breaker.TargetModel, target, false)
	}

	state, _ := b.State(ctx, breaker.TargetModel, target)
	if state != breaker.StateOpen {
		t.Fatalf("expected open, got %s", state)
	}

	// Wait for OpenTimeout to elapse with margin.
	time.Sleep(150 * time.Millisecond)

	allowed, err := b.Allow(ctx, breaker.TargetModel, target)
	if err != nil {
		t.Fatalf("allow after timeout: %v", err)
	}
	if !allowed {
		t.Fatal("requests should be allowed after open timeout elapses")
	}

	state, _ = b.State(ctx, breaker.TargetModel, target)
	if state != breaker.StateHalfOpen {
		t.Fatalf("expected half_open, got %s", state)
	}
}

func TestBreakerClosesAfterSuccessThreshold(t *testing.T) {
	b, ctx, target := setup(t, 50*time.Millisecond)

	// Trip to OPEN.
	for i := 0; i < 3; i++ {
		_, _ = b.Allow(ctx, breaker.TargetModel, target)
		_ = b.RecordResult(ctx, breaker.TargetModel, target, false)
	}
	time.Sleep(150 * time.Millisecond)

	// Transition to HALF_OPEN.
	allowed, _ := b.Allow(ctx, breaker.TargetModel, target)
	if !allowed {
		t.Fatal("expected half_open Allow to succeed")
	}

	// Two successes close the breaker.
	for i := 0; i < 2; i++ {
		_ = b.RecordResult(ctx, breaker.TargetModel, target, true)
	}

	state, _ := b.State(ctx, breaker.TargetModel, target)
	if state != breaker.StateClosed {
		t.Fatalf("expected closed after success threshold, got %s", state)
	}
}
