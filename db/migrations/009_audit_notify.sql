BEGIN;

CREATE OR REPLACE FUNCTION notify_audit_entry() RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('audit_entries_channel', NEW.id::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_entry_notify
    AFTER INSERT ON audit_entries
    FOR EACH ROW
    EXECUTE FUNCTION notify_audit_entry();

COMMIT;