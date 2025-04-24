CREATE TABLE IF NOT EXISTS blueprint_instances (
    id uuid PRIMARY KEY,
    "name" varchar(255) NOT NULL UNIQUE,
    "status" smallint NOT NULL,
    last_status_update_timestamp timestamptz,
    last_deployed_timestamp timestamptz,
    last_deploy_attempt_timestamp timestamptz,
    metadata jsonb NOT NULL,
    exports jsonb NOT NULL,
    child_dependencies jsonb,
    durations jsonb 
);
