CREATE TABLE IF NOT EXISTS events (
    id uuid PRIMARY KEY,
    "type" varchar(255) NOT NULL,
    channel_type varchar(255) NOT NULL,
    channel_id uuid NOT NULL,
    data jsonb NOT NULL,
    "timestamp" timestamptz,
    "end" boolean NOT NULL
);
