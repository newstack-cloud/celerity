-- A script to release the lock for the stream trimming worker,
-- where a lock provide a best effort mechanism to avoid multiple consumers
-- attempting to trim the stream at the same time.
-- This does not guarantee mutual exclusivity, so multiple consumers
-- may still attempt to trim the stream at the same time.
-- A more involved mechanism is deemed overkill for this use case as the
-- stream will either gets larger than expected for a short period of time in the case of failure
-- or occassional duplicate trims are made by multiple consumers.

if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
