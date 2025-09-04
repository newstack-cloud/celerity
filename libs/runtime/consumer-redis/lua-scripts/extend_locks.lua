-- A script to extend message locks if the current consumer owns the lock.

local results = {}
local consumer_id = ARGV[1]
local lock_duration = tonumber(ARGV[2])

for i = 1, #KEYS do
    local lock_key = KEYS[i]

    local current_owner = redis.call('GET', lock_key)
    if current_owner == consumer_id then
        redis.call('PEXPIRE', lock_key, lock_duration)
        results[i] = 1
    else
        results[i] = 0
    end
end

return results
