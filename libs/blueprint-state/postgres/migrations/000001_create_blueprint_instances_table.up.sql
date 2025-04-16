CREATE TABLE IF NOT EXISTS blueprint_instances (
    id uuid PRIMARY KEY,
    "name" varchar(255) NOT NULL UNIQUE,
    "status" smallint NOT NULL,
    last_status_update_timestamp timestamp,
    last_deployed_timestamp timestamp,
    last_deploy_attempt_timestamp timestamp,
    metadata jsonb NOT NULL,
    exports jsonb NOT NULL,
    child_dependencies jsonb,
    durations jsonb 
);
