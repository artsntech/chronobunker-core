// internal/router/cache.go
package router

import (
	"context"
	"database/sql"
	"sync"

	"golang.org/x/sync/singleflight"
)

// modelCache provides a concurrency-safe cache for model specs.
// It uses singleflight to collapse concurrent loads of the same key
// into a single database query, preventing cache stampede under load.
type modelCache struct {
	db    *sql.DB
	group singleflight.Group
	mu    sync.RWMutex
	specs map[string]ModelSpec
}

func newModelCache(db *sql.DB) *modelCache {
	return &modelCache{
		db:    db,
		specs: make(map[string]ModelSpec),
	}
}

// Get returns the spec for modelID. If the spec is not cached, it loads
// it from the database. Concurrent callers for the same modelID share
// a single database query.
func (c *modelCache) Get(ctx context.Context, modelID string) (ModelSpec, error) {
	c.mu.RLock()
	if spec, ok := c.specs[modelID]; ok {
		c.mu.RUnlock()
		return spec, nil
	}
	c.mu.RUnlock()

	v, err, _ := c.group.Do(modelID, func() (any, error) {
		var spec ModelSpec
		err := c.db.QueryRowContext(ctx, `
			SELECT id, provider, input_cost_per_1m, output_cost_per_1m, context_window
			FROM models WHERE id = $1 AND status = 'ga'`, modelID).
			Scan(&spec.ID, &spec.Provider, &spec.InputCostPer1M, &spec.OutputCostPer1M, &spec.ContextWindow)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		c.specs[modelID] = spec
		c.mu.Unlock()
		return spec, nil
	})
	if err != nil {
		return ModelSpec{}, err
	}
	return v.(ModelSpec), nil
}