-- migrate:up

CREATE TABLE tasks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid()

);

-- migrate:down

