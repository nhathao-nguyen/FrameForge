-- Gate C+D repair: RenderProfile lifecycle transitions fail closed at the
-- durable boundary as well as in the application service.
CREATE OR REPLACE FUNCTION nh_media_reject_active_profile_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status = 'active' AND (NEW.document IS DISTINCT FROM OLD.document OR NEW.content_hash IS DISTINCT FROM OLD.content_hash OR NEW.version IS DISTINCT FROM OLD.version) THEN
        RAISE EXCEPTION 'active render profile is immutable';
    END IF;
    IF NOT ((OLD.status = 'draft' AND NEW.status = 'active')
        OR (OLD.status = 'active' AND NEW.status = 'deprecated')
        OR (OLD.status = 'deprecated' AND NEW.status = 'disabled')
        OR (OLD.status = NEW.status)) THEN
        RAISE EXCEPTION 'invalid render profile lifecycle transition';
    END IF;
    RETURN NEW;
END;
$$;
