BEGIN;

-- Ensure agent names are unique per tenant. This prevents the seed from
-- creating duplicates when re-run, and is a semantic requirement: two agents
-- with the same name in the same tenant are indistinguishable to the operator.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'uk_agents_tenant_name'
    ) THEN
        ALTER TABLE agents ADD CONSTRAINT uk_agents_tenant_name UNIQUE (tenant_id, name);
    END IF;
END
$$;

COMMIT;