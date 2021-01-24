-- migrate:up

create or replace function tg_notify_ready_task ()
    returns trigger
    language plpgsql
as $$
begin
    if NEW.deps = '{}' then
        PERFORM pg_notify('ready_task', NEW.id::text);
    end if;
    RETURN NEW;
end;
$$;

CREATE TRIGGER notify_counters
    AFTER INSERT OR UPDATE
    ON tasks
    FOR EACH ROW
EXECUTE PROCEDURE tg_notify_ready_task();

-- migrate:down

