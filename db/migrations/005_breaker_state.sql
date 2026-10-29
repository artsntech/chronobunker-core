-- ChronoBunker Core - Phase 3c: circuit breaker transition log.
-- The live state lives in Redis. This table records every transition for
-- post-incident analysis and for the Grafana dashboard.

BEGIN;

CREATE TABLE breaker_transitions (
    id            BIGSERIAL PRIMARY KEY,
    target_type   TEXT NOT NULL CHECK (target_type IN ('agent', 'tool', 'model')),
    target_id     TEXT NOT NULL,
    from_state    TEXT NOT NULL CHECK (from_state IN ('closed', 'open', 'half_open')),
    to_state      TEXT NOT NULL CHECK (to_state IN ('closed', 'open', 'half_open')),
    reason        TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_breaker_target_created ON breaker_transitions (target_type, target_id, created_at DESC);

-- Breaker transitions are global (not tenant-scoped) because a model outage
-- affects every tenant. No RLS on this table.

GRANT SELECT, INSERT ON breaker_transitions TO chrono_app;
GRANT USAGE, SELECT ON SEQUENCE breaker_transitions_id_seq TO chrono_app;

COMMIT;