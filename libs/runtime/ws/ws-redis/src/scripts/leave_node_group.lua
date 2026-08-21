local index_key = KEYS[1]
local meta = ARGV[1]
local node = ARGV[2]
local group_id = ARGV[3]

local members_key = meta .. ':node-group-members:' .. group_id

redis.call('SREM', members_key, node)
redis.call('DEL', meta .. ':node:' .. node)

-- Counted in the same step that takes the group out of the index, so a node
-- joining cannot land in a group between the two and be left in one nothing
-- else can find.
if redis.call('SCARD', members_key) == 0 then
    redis.call('SREM', index_key, group_id)
    return 1
end

return 0
