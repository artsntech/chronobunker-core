-- ChronoBunker Core - Phase H7: cryptographic device binding.
-- Stores the Ed25519 public key registered by each device at login.
-- The private key never leaves the device.

BEGIN;

CREATE TABLE device_keys (
    device_id       TEXT PRIMARY KEY,
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    public_key      TEXT NOT NULL,
    algorithm       TEXT NOT NULL DEFAULT 'ed25519'
                    CHECK (algorithm IN ('ed25519')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_device_keys_tenant ON device_keys (tenant_id);

ALTER TABLE device_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE device_keys FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_device_keys ON device_keys
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::UUID)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::UUID);

GRANT SELECT, INSERT, UPDATE ON device_keys TO chrono_app;

COMMENT ON TABLE device_keys IS
    'Ed25519 public keys registered per device. The private key stays on the device.';

COMMIT;