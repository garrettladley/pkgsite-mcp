-- Fixed-window counter for shared IP rate limiting.
-- KEYS[1]: counter key
-- ARGV[1]: ttl_ms

local key = KEYS[1]
local ttl_ms = tonumber(ARGV[1])

local count = redis.call('INCR', key)
if count == 1 or redis.call('PTTL', key) < 0 then
    redis.call('PEXPIRE', key, ttl_ms)
end

return count
