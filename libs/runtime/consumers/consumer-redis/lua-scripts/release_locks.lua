-- A script to release locks for a set of messages if the current consumer
-- owns the current lock.

local results = {}
local consumer_id = ARGV[1]

for i = 1, #KEYS do
    local lock_key = KEYS[i]

    local current_owner = redis.call('GET', lock_key)
    if current_owner == consumer_id then
        redis.call('DEL', lock_key)
        results[i] = 1
    else
        results[i] = 0
    end
end

return results
