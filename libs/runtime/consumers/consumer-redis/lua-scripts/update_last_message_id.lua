-- A script to update the last message ID that was read and successfully processed
-- from a Redis stream.
-- This script is used as an efficient way to atomically update the last message ID
-- making use of the sequential nature of Redis script execution to avoid race
-- conditions between multiple consumers/workers simultaneously reading from the same stream.

local last_message_id_key = KEYS[1]
local new_last_message_id = ARGV[1]

local current_id = redis.call("GET", last_message_id_key)

if not current_id then
    redis.call("SET", last_message_id_key, new_last_message_id)
    return 1
end

-- Helper function to parse a Redis stream ID and convert to comparable values
local function parse_stream_id(stream_id)
    local timestamp, sequence = string.match(stream_id, "^(%d+)-(%d+)$")
    if timestamp and sequence then
        return tonumber(timestamp), tonumber(sequence)
    end

    -- Fallback for edge cases like "0-0" or malformed IDs
    return 0, 0
end

local new_timestamp, new_sequence = parse_stream_id(new_last_message_id)
local current_timestamp, current_sequence = parse_stream_id(current_id)

-- Compare timestamps first, then sequences if timestamps are equal
if new_timestamp > current_timestamp
    or (new_timestamp == current_timestamp and new_sequence > current_sequence)
then
    redis.call("SET", last_message_id_key, new_last_message_id)
    return 1
end

return 0
