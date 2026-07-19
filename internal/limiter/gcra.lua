-- AegisRL GCRA (Generic Cell Rate Algorithm) — Atomic Redis Lua Script
--
-- Stores a single value per key (the Theoretical Arrival Time, TAT) instead of
-- two fields (tokens + timestamp) — a smaller Redis memory footprint per key
-- than the token bucket, with identical burst semantics.
--
-- KEYS[1] = rate limit key
-- ARGV[1] = capacity (burst size)
-- ARGV[2] = rate (requests per second)
-- ARGV[3] = requested (usually 1)
--
-- Returns the same shape as token_bucket.lua: [allowed, remaining, retry_after_ms]

local key       = KEYS[1]
local capacity  = tonumber(ARGV[1])
local rate      = tonumber(ARGV[2])
local requested = tonumber(ARGV[3])

local t      = redis.call("TIME")
local now_ms = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)

-- emission interval: ideal spacing between requests at the sustained rate
local emission_ms = 1000 / rate
-- burst tolerance: how far ahead of schedule a client may run
local burst_ms    = emission_ms * capacity

-- TAT = theoretical arrival time of the next conforming request
local tat = tonumber(redis.call("GET", key))
if tat == nil or tat < now_ms then
    tat = now_ms
end

local new_tat  = tat + requested * emission_ms
local allow_at = new_tat - burst_ms

local allowed        = 0
local remaining      = 0
local retry_after_ms = 0

if now_ms >= allow_at then
    allowed = 1
    redis.call("SET", key, new_tat, "PX", math.max(1, math.ceil(burst_ms * 2)))
    -- how many more requests could pass right now
    remaining = math.floor((now_ms - (new_tat - burst_ms)) / emission_ms)
else
    retry_after_ms = math.ceil(allow_at - now_ms)
end

return {allowed, remaining, retry_after_ms}
