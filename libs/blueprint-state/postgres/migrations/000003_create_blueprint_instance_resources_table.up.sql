CREATE TABLE IF NOT EXISTS blueprint_instance_resources (
    instance_id uuid,
    resource_name varchar(255) NOT NULL,
    resource_id uuid,
    PRIMARY KEY (instance_id, resource_name),
    FOREIGN KEY (resource_id) REFERENCES resources (id)
        ON DELETE CASCADE,
    FOREIGN KEY (instance_id) REFERENCES blueprint_instances (id)
        ON DELETE CASCADE
);
