CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER
AS $$
BEGIN
    NEW.updated_at := NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_avatars_set_updated_at ON avatars;

CREATE TRIGGER trg_avatars_set_updated_at
BEFORE UPDATE ON avatars
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();