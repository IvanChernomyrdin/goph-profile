CREATE OR REPLACE FUNCTION public.set_updated_at()
RETURNS TRIGGER
AS $$
BEGIN
    NEW.updated_at := NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_avatars_set_updated_at ON public.avatars;

CREATE TRIGGER trg_avatars_set_updated_at
BEFORE UPDATE ON public.avatars
FOR EACH ROW
EXECUTE FUNCTION public.set_updated_at();