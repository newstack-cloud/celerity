CREATE TABLE IF NOT EXISTS blueprint_instances (
    id uuid PRIMARY KEY,
    "status" smallint NOT NULL,
    last_status_update_timestamp timestamp,
    last_deployed_timestamp timestamp,
    last_deploy_attempt_timestamp timestamp,
    metadata jsonb NOT NULL,
    exports jsonb NOT NULL,
    child_dependencies jsonb,
    durations jsonb 
);
