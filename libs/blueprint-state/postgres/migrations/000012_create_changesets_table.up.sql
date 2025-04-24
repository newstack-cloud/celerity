CREATE TABLE IF NOT EXISTS changesets (
    id uuid PRIMARY KEY,
    instance_id uuid,
    destroy boolean NOT NULL,
    "status" varchar(128) NOT NULL,
    blueprint_location text NOT NULL,
    "changes" jsonb,
    "created" timestamptz,
    FOREIGN KEY (instance_id) REFERENCES blueprint_instances (id)
        ON DELETE CASCADE
);
