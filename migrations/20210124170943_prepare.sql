-- migrate:up

PREPARE propagate_deps(uuid) AS UPDATE tasks
     SET deps = array_remove(deps, $1)
     WHERE Array[$1] <@ deps;

PREPARE set_finished(uuid) AS UPDATE tasks
    SET finished = TRUE
    WHERE id = $1;

-- migrate:down

DEALLOCATE propagate_deps;
DEALLOCATE set_finished;