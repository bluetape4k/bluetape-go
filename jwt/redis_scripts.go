package jwt

const redisCurrentScript = `
local kid = redis.call("GET", KEYS[1])
if not kid or kid == "" then
	return {0, "", ""}
end

local payload = redis.call("HGET", KEYS[2], kid)
if not payload then
	return {0, "", kid}
end

return {1, payload, kid}
`

const redisRotateCASScript = `
local observed_kid = ARGV[1]
local candidate_kid = ARGV[2]
local candidate_payload = ARGV[3]
local candidate_score = tonumber(ARGV[4])
local capacity = tonumber(ARGV[5])
local ttl_ms = tonumber(ARGV[6])
local version = ARGV[7]
local algorithm = ARGV[8]

local current_kid = redis.call("GET", KEYS[1])
if current_kid and current_kid ~= "" and current_kid ~= observed_kid then
	local current_payload = redis.call("HGET", KEYS[2], current_kid)
	if current_payload then
		return {0, current_payload, current_kid}
	end
end

redis.call("HSET", KEYS[2], candidate_kid, candidate_payload)
redis.call("ZADD", KEYS[3], candidate_score, candidate_kid)
redis.call("SET", KEYS[1], candidate_kid)
redis.call("HSET", KEYS[4], "version", version, "algorithm", algorithm)

local count = redis.call("ZCARD", KEYS[3])
if count > capacity then
	local stale = redis.call("ZRANGE", KEYS[3], 0, -1)
	for _, stale_kid in ipairs(stale) do
		if redis.call("ZCARD", KEYS[3]) <= capacity then
			break
		elseif stale_kid ~= candidate_kid then
			redis.call("HDEL", KEYS[2], stale_kid)
			redis.call("ZREM", KEYS[3], stale_kid)
		end
	end
end

if ttl_ms > 0 then
	redis.call("PEXPIRE", KEYS[1], ttl_ms)
	redis.call("PEXPIRE", KEYS[2], ttl_ms)
	redis.call("PEXPIRE", KEYS[3], ttl_ms)
	redis.call("PEXPIRE", KEYS[4], ttl_ms)
else
	redis.call("PERSIST", KEYS[1])
	redis.call("PERSIST", KEYS[2])
	redis.call("PERSIST", KEYS[3])
	redis.call("PERSIST", KEYS[4])
end

return {1, candidate_payload, candidate_kid}
`

const redisStoreScript = `
local candidate_kid = ARGV[1]
local candidate_payload = ARGV[2]
local candidate_score = tonumber(ARGV[3])
local capacity = tonumber(ARGV[4])
local ttl_ms = tonumber(ARGV[5])
local version = ARGV[6]
local algorithm = ARGV[7]

redis.call("HSET", KEYS[2], candidate_kid, candidate_payload)
redis.call("ZADD", KEYS[3], candidate_score, candidate_kid)
redis.call("SET", KEYS[1], candidate_kid)
redis.call("HSET", KEYS[4], "version", version, "algorithm", algorithm)

local count = redis.call("ZCARD", KEYS[3])
if count > capacity then
	local stale = redis.call("ZRANGE", KEYS[3], 0, -1)
	for _, stale_kid in ipairs(stale) do
		if redis.call("ZCARD", KEYS[3]) <= capacity then
			break
		elseif stale_kid ~= candidate_kid then
			redis.call("HDEL", KEYS[2], stale_kid)
			redis.call("ZREM", KEYS[3], stale_kid)
		end
	end
end

if ttl_ms > 0 then
	redis.call("PEXPIRE", KEYS[1], ttl_ms)
	redis.call("PEXPIRE", KEYS[2], ttl_ms)
	redis.call("PEXPIRE", KEYS[3], ttl_ms)
	redis.call("PEXPIRE", KEYS[4], ttl_ms)
else
	redis.call("PERSIST", KEYS[1])
	redis.call("PERSIST", KEYS[2])
	redis.call("PERSIST", KEYS[3])
	redis.call("PERSIST", KEYS[4])
end

return {1, candidate_payload, candidate_kid}
`
