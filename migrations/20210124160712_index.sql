-- migrate:up

CREATE INDEX tasks_ready ON tasks (id)
    INCLUDE (data)
    WHERE finished = FALSE AND deps = '{}';

CREATE INDEX task_deps ON tasks
    USING gin(deps);

-- migrate:down

DROP INDEX tasks_ready;
DROP INDEX task_deps;

