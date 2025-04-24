CREATE TABLE IF NOT EXISTS blueprint_validations (
    id uuid PRIMARY KEY,
    "status" varchar(128) NOT NULL,
    blueprint_location text NOT NULL,
    "created" timestamptz
);
