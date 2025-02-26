CREATE TABLE IF NOT EXISTS resource_drift (
    id SERIAL PRIMARY KEY,
    resource_id uuid,
    instance_id uuid,
    drifted_spec_data jsonb NOT NULL,
    difference jsonb NOT NULL,
    "timestamp" timestamp,
    FOREIGN KEY (resource_id) REFERENCES resources (id)
        ON DELETE CASCADE,
    FOREIGN KEY (instance_id) REFERENCES blueprint_instances (id),
    UNIQUE (resource_id)
);
