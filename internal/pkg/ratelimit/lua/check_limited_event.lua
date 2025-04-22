-- 检查指定时间点后是否触发过限流


-- 限流对象的限流事件记录键
local limitedEventsKey = KEYS[1]
-- 起始时间戳（毫秒）
local sinceMilli = tonumber(ARGV[1])
-- 当前时间戳（毫秒）
local now = tonumber(ARGV[2])

-- 检查该时间范围内是否有限流事件记录
local events = redis.call('ZCOUNT', limitedEventsKey, sinceMilli, now)

if events > 0 then
    return "true"
else
    return "false"
end