-- migrate:up

CREATE TABLE tasks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    data JSONB NOT NULL,
    deps uuid[],
    finished bool NOT NULL default false
);

-- migrate:down

DROP TABLE tasks;