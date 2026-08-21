local index_key = KEYS[1]
local meta = ARGV[1]
local node = ARGV[2]
local capacity = tonumber(ARGV[3])
local ttl_ms = tonumber(ARGV[4])
local new_group_id = ARGV[5]

local node_key = meta .. ':node:' .. node
local emptiest_id = nil
local emptiest_count = nil

for _, group_id in ipairs(redis.call('SMEMBERS', index_key)) do
    local members_key = meta .. ':node-group-members:' .. group_id
    local live = 0
    local holds_this_node = false

    for _, member in ipairs(redis.call('SMEMBERS', members_key)) do
        if member == node then
            holds_this_node = true
            live = live + 1
        elseif redis.call('EXISTS', meta .. ':node:' .. member) == 1 then
            live = live + 1
        else
            redis.call('SREM', members_key, member)
        end
    end

    if holds_this_node then
        redis.call('SET', node_key, group_id, 'PX', ttl_ms)
        return { group_id, 'held' }
    end

    if live < capacity and (emptiest_count == nil or live < emptiest_count) then
        emptiest_id = group_id
        emptiest_count = live
    end
end

local group_id = emptiest_id
if group_id == nil then
    group_id = new_group_id
    redis.call('SADD', index_key, group_id)
end

redis.call('SADD', meta .. ':node-group-members:' .. group_id, node)
redis.call('SET', node_key, group_id, 'PX', ttl_ms)
return { group_id, 'joined' }
