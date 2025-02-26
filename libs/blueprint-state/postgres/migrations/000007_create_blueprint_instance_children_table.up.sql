CREATE TABLE IF NOT EXISTS blueprint_instance_children (
    parent_instance_id uuid,
    child_instance_name varchar(255) NOT NULL,
    child_instance_id uuid,
    FOREIGN KEY (parent_instance_id) REFERENCES blueprint_instances (id)
        ON DELETE CASCADE,
    FOREIGN KEY (child_instance_id) REFERENCES blueprint_instances (id)
        ON DELETE CASCADE,
    PRIMARY KEY (parent_instance_id, child_instance_name)
);
