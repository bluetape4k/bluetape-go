package redisbloom

import "github.com/redis/go-redis/v9"

const initConfigLua = `
local existing = redis.call("HGETALL", KEYS[2])
if #existing == 0 then
	if redis.call("STRLEN", KEYS[1]) > 0 then
		return redis.error_reply("config_corrupt")
	end
	redis.call("HSET", KEYS[2],
		"version", ARGV[1],
		"family", ARGV[2],
		"expected_insertions", ARGV[3],
		"false_positive_probability", ARGV[4],
		"bit_size", ARGV[5],
		"hash_function_count", ARGV[6],
		"hasher_key", ARGV[7],
		"fingerprint", ARGV[8])
	return "created"
end
local stored = redis.call("HGET", KEYS[2], "fingerprint")
if stored == false then
	return redis.error_reply("config_corrupt")
end
if stored ~= ARGV[8] then
	return redis.error_reply("config_mismatch")
end
if redis.call("HGET", KEYS[2], "version") ~= ARGV[1] then return redis.error_reply("config_corrupt") end
if redis.call("HGET", KEYS[2], "family") ~= ARGV[2] then return redis.error_reply("config_corrupt") end
if redis.call("HGET", KEYS[2], "expected_insertions") ~= ARGV[3] then return redis.error_reply("config_corrupt") end
if redis.call("HGET", KEYS[2], "false_positive_probability") ~= ARGV[4] then return redis.error_reply("config_corrupt") end
if redis.call("HGET", KEYS[2], "bit_size") ~= ARGV[5] then return redis.error_reply("config_corrupt") end
if redis.call("HGET", KEYS[2], "hash_function_count") ~= ARGV[6] then return redis.error_reply("config_corrupt") end
if redis.call("HGET", KEYS[2], "hasher_key") ~= ARGV[7] then return redis.error_reply("config_corrupt") end
return "matched"
`

var initConfigScript = redis.NewScript(initConfigLua)
