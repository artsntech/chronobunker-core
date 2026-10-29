// Package breaker implements a three-state circuit breaker with Redis-backed
// state and Postgres-backed transition logging.
package breaker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/telemetry"
)

// State is the circuit breaker state.
type State string

const (
	StateClosed   State = "closed"
	StateOpen     State = "open"
	StateHalfOpen State = "half_open"
)

// TargetType is the type of the entity protected by the breaker.
type TargetType string

const (
	TargetAgent TargetType = "agent"
	TargetTool  TargetType = "tool"
	TargetModel TargetType = "model"
)

// Config defines the breaker's thresholds.
type Config struct {
	FailureThreshold int
	SuccessThreshold int
	OpenTimeout      time.Duration
}

// DefaultConfig returns production-reasonable defaults.
func DefaultConfig() Config {
	return Config{
		FailureThreshold: 5,
		SuccessThreshold: 3,
		OpenTimeout:      30 * time.Second,
	}
}

// Breaker is a circuit breaker for a single target.
type Breaker struct {
	rdb    *redis.Client
	db     *sql.DB
	config Config
}

// New constructs a Breaker.
func New(rdb *redis.Client, db *sql.DB, config Config) *Breaker {
	return &Breaker{rdb: rdb, db: db, config: config}
}

// Allow returns true if the request should be attempted.
func (b *Breaker) Allow(ctx context.Context, targetType TargetType, targetID string) (bool, error) {
	state, err := b.loadState(ctx, targetType, targetID)
	if err != nil {
		return false, err
	}

	switch state {
	case StateClosed:
		return true, nil

	case StateOpen:
		openedAt, err := b.rdb.Get(ctx, stateKey(targetType, targetID, "opened_at")).Int64()
		if errors.Is(err, redis.Nil) {
			log.Printf("breaker: open state but opened_at missing for %s/%s; rejecting request", targetType, targetID)
			return false, nil
		}
		if err != nil {
			return false, err
		}
		elapsed := time.Since(time.Unix(openedAt, 0))
		if elapsed >= b.config.OpenTimeout {
			if err := b.transition(ctx, targetType, targetID, StateOpen, StateHalfOpen, "timeout elapsed"); err != nil {
				return false, err
			}
			return true, nil
		}
		return false, nil

	case StateHalfOpen:
		return true, nil

	default:
		return false, fmt.Errorf("breaker: unknown state %q", state)
	}
}

// RecordResult records the outcome of a request.
func (b *Breaker) RecordResult(ctx context.Context, targetType TargetType, targetID string, success bool) error {
	state, err := b.loadState(ctx, targetType, targetID)
	if err != nil {
		return err
	}

	switch state {
	case StateClosed:
		if success {
			b.rdb.Del(ctx, stateKey(targetType, targetID, "failures"))
			return nil
		}
		failures, err := b.rdb.Incr(ctx, stateKey(targetType, targetID, "failures")).Result()
		if err != nil {
			return err
		}
		if int(failures) >= b.config.FailureThreshold {
			return b.transition(ctx, targetType, targetID, StateClosed, StateOpen, fmt.Sprintf("%d consecutive failures", failures))
		}
		return nil

	case StateOpen:
		return nil

	case StateHalfOpen:
		if success {
			successes, err := b.rdb.Incr(ctx, stateKey(targetType, targetID, "successes")).Result()
			if err != nil {
				return err
			}
			if int(successes) >= b.config.SuccessThreshold {
				return b.transition(ctx, targetType, targetID, StateHalfOpen, StateClosed, "success threshold reached")
			}
			return nil
		}
		return b.transition(ctx, targetType, targetID, StateHalfOpen, StateOpen, "probe failed")

	default:
		return fmt.Errorf("breaker: unknown state %q", state)
	}
}

// State returns the current state of the breaker.
func (b *Breaker) State(ctx context.Context, targetType TargetType, targetID string) (State, error) {
	return b.loadState(ctx, targetType, targetID)
}

func (b *Breaker) loadState(ctx context.Context, targetType TargetType, targetID string) (State, error) {
	raw, err := b.rdb.Get(ctx, stateKey(targetType, targetID, "state")).Result()
	if errors.Is(err, redis.Nil) {
		return StateClosed, nil
	}
	if err != nil {
		return "", err
	}
	return State(raw), nil
}

func (b *Breaker) transition(ctx context.Context, targetType TargetType, targetID string, from, to State, reason string) error {
	log.Printf("breaker: transitioning %s/%s: %s -> %s (%s)", targetType, targetID, from, to, reason)

	stateKeyName := stateKey(targetType, targetID, "state")
	openedAtKeyName := stateKey(targetType, targetID, "opened_at")
	failuresKeyName := stateKey(targetType, targetID, "failures")
	successesKeyName := stateKey(targetType, targetID, "successes")

	pipe := b.rdb.TxPipeline()
	pipe.Set(ctx, stateKeyName, string(to), 0)
	if to == StateOpen {
		pipe.Set(ctx, openedAtKeyName, time.Now().Unix(), 0)
	} else {
		pipe.Del(ctx, openedAtKeyName)
	}
	pipe.Del(ctx, failuresKeyName)
	if to == StateClosed {
		pipe.Del(ctx, successesKeyName)
	}

	cmds, err := pipe.Exec(ctx)
	if err != nil {
		log.Printf("breaker: pipeline exec failed: %v", err)
		return err
	}
	for i, cmd := range cmds {
		if cmdErr := cmd.Err(); cmdErr != nil {
			log.Printf("breaker: pipeline command %d failed: %v", i, cmdErr)
		}
	}

	// Verification.
	got, err := b.rdb.Get(ctx, stateKeyName).Result()
	if err != nil {
		log.Printf("breaker: verification failed to read state: %v", err)
		return err
	}
	if got != string(to) {
		return fmt.Errorf("breaker: state verification failed: expected %q, got %q", to, got)
	}
	if to == StateOpen {
		openedAt, err := b.rdb.Get(ctx, openedAtKeyName).Int64()
		if err != nil {
			return fmt.Errorf("breaker: opened_at missing after transition to open: %w", err)
		}
		if time.Since(time.Unix(openedAt, 0)) > time.Second {
			return fmt.Errorf("breaker: opened_at is stale: %v", time.Unix(openedAt, 0))
		}
	}

	// Emit the Prometheus gauge. 0=closed, 1=half_open, 2=open.
	var stateValue float64
	switch to {
	case StateClosed:
		stateValue = 0
	case StateHalfOpen:
		stateValue = 1
	case StateOpen:
		stateValue = 2
	}
	telemetry.BreakerState.WithLabelValues(string(targetType), targetID).Set(stateValue)

	// Publish for observers.
	payload := fmt.Sprintf("%s:%s:%s->%s:%s", targetType, targetID, from, to, reason)
	b.rdb.Publish(ctx, "breaker:transitions", payload)

	// Persist for audit.
	if _, err := b.db.ExecContext(ctx, `
		INSERT INTO breaker_transitions (target_type, target_id, from_state, to_state, reason)
		VALUES ($1, $2, $3, $4, $5)`,
		string(targetType), targetID, string(from), string(to), reason); err != nil {
		log.Printf("breaker: failed to record transition: %v", err)
	}

	return nil
}

func stateKey(targetType TargetType, targetID, suffix string) string {
	return fmt.Sprintf("breaker:%s:%s:%s", targetType, targetID, suffix)
}