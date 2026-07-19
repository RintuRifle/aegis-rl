-- AegisRL Token Bucket Rate Limiter — Atomic Redis Lua Script
-- Executes as a single atomic operation via EVALSHA (no interleaving possible).
--
-- KEYS[1] = rate limit key, e.g. "rl:key:{apikey}" or "rl:ip:{ip}"
-- ARGV[1] = capacity (max tokens / burst size)
-- ARGV[2] = refill_rate (tokens per second)
-- ARGV[3] = requested tokens (usually 1)
--
-- Timestamps come from Redis TIME, not the client. With multiple Go replicas,
-- client clocks can drift; using the Redis server clock keeps the bucket math
-- consistent no matter which replica sent the request. (Safe on Redis >= 5:
-- scripts are replicated by effects, so non-deterministic commands are fine.)

local key       = KEYS[1]
local capacity  = tonumber(ARGV[1])
local rate      = tonumber(ARGV[2])
local requested = tonumber(ARGV[3])

local t   = redis.call("TIME")
local now = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)

-- Fetch current bucket state (returns {nil,nil} if key doesn't exist)
local bucket  = redis.call("HMGET", key, "tokens", "ts")
local tokens  = tonumber(bucket[1])
local last_ts = tonumber(bucket[2])

-- First request for this key — initialize with full bucket
if tokens == nil then
    tokens  = capacity
    last_ts = now
end

-- Refill: add tokens proportional to elapsed time since last request
local elapsed_sec = math.max(0, (now - last_ts) / 1000)
tokens = math.min(capacity, tokens + elapsed_sec * rate)

-- Check if enough tokens are available
local allowed = 0
if tokens >= requested then
    tokens  = tokens - requested
    allowed = 1
end

-- Persist updated state (HSET — HMSET is deprecated since Redis 4)
redis.call("HSET", key, "tokens", tokens, "ts", now)

-- TTL = 2x full refill time (min 1s so EXPIRE never gets 0 and deletes the key)
redis.call("EXPIRE", key, math.max(1, math.ceil((capacity / rate) * 2)))

-- Calculate retry-after for denied requests
local retry_after_ms = 0
if allowed == 0 then
    retry_after_ms = math.ceil(((requested - tokens) / rate) * 1000)
end

-- Return: [allowed(0|1), tokens_remaining(floor), retry_after_ms]
return {allowed, math.floor(tokens), retry_after_ms}
