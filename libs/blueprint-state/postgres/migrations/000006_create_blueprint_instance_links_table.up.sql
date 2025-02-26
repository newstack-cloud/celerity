CREATE TABLE IF NOT EXISTS blueprint_instance_links (
    instance_id uuid,
    link_name varchar(255) NOT NULL,
    link_id uuid,
    FOREIGN KEY (link_id) REFERENCES links (id)
        ON DELETE CASCADE,
    FOREIGN KEY (instance_id) REFERENCES blueprint_instances (id)
        ON DELETE CASCADE,
    PRIMARY KEY (instance_id, link_name)
);
