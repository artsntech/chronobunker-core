BEGIN;
CREATE OR REPLACE FUNCTION notify_audit_entry() RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('audit_entries_channel', NEW.tenant_id::text || ':' || NEW.id::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
COMMIT;