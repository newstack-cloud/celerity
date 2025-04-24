CREATE TABLE IF NOT EXISTS resources (
    id uuid PRIMARY KEY,
    "type" varchar(255) NOT NULL,
    template_name varchar(255),
    "status" smallint NOT NULL,
    precise_status smallint NOT NULL,
    last_status_update_timestamp timestamptz,
    last_deployed_timestamp timestamptz,
    last_deploy_attempt_timestamp timestamptz,
    spec_data jsonb NOT NULL,
    "description" text,
    metadata jsonb,
    depends_on_resources jsonb,
    depends_on_children jsonb,
    failure_reasons jsonb NOT NULL,
    drifted boolean,
    last_drift_detected_timestamp timestamptz,
    durations jsonb
);
