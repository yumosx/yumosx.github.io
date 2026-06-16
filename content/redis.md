---
Title: redis 缓存设计方案 
Date: 2025-10-02
---

## 预热缓存

在程序启动之前的时候把系统中高频访问的数据刷新到 redis 中去，比如商品列表信息，课程信息，当刷新完毕之后，将对外的api 端口打开
加载缓存的时候，我们可以在 go 中去嵌入 lua 脚本 对一些数据做清洗

同样在预热缓存的时候，我们可以先把这些预热缓存的东西刷新到 Rocksdb 中去，避免直接读csv，大json 文件
```go
config := configs.Load()
redis.Set(key, config)
```

过期时间设置多少
- 不是看表大小，而是看数据的访问频率和变更频率, 用户昵称这种变更少的热点数据，可以设 30 分钟甚至更长, 库存数量这种频繁变化的数据，设置 5-10 秒就够了
- 对于一些热点数据，可以稍微把缓存时间调长一点，这样可以提高缓存命中率


缓存预热的高阶姿势：灰度流量 + 压测热数据

你启动时“先预热，再开外网端口”，这个流程可以更精细化：

预热回放：不是简单地全量加载，而是从 流量录制回放系统（如 GoReplay、TCPCopy）或 前一日访问日志 中提取高 QPS 的 key，优先加载到缓存。
灰度切流：预热完成后，先放 1% 的生产流量进来，观察缓存命中率和 DB 负载，逐步放大到 100%，同时持续监控。
缓存预热任务与健康检查挂钩：在 K8s 的 readiness probe 里加入“缓存预热完成”检测，确保 Pod 不接收流量直到缓存就绪。


## 缓存更新要同步

如果在库中修改了某条数据，应该立刻去同步对应的缓存，比如某一个用户 9812 的信息被更新，应该立刻去把 key=9812 这个 redis 的 key, value 给删除，下次访问的时候继续刷新到缓存

```go
//读接口 get id 1
if v = cache.Get(ctx, id); v != nil {
    return v
}
// 先从db 中读取
value, err := db.GetByID(ctx, id)
if err != nil {
    return err
}
// 然后写回缓存
cache.Set(ctx, id, value)

//写接口 eidt id 2
db.Update(id)
cache.Delete(id)
```

### 订阅更新&关系型数据缓存同步

关系型数据缓存同步其实是有点难的，这是因为 数据库更新一次，需要同步更新多个 key, 

一种比较精巧的实现是:
1. 使用 canal 这种中间件订阅数据库 id 的变化
2. 当出现变化的时候，将对应的 id 推送到消息队列中去，
3. 执行预设脚本刷新对应的缓存

另外一种是异步脚本订阅刷新


## 冷数据和热数据分开缓存

临时缓存+热点缓存思路

热点检测用 SISMEMBER 维护了一个 hot_key 集合，当然热 key 判定可以结合 滑动时间窗口的 QPS 统计（比如每 100ms 统计一次请求频率），自动晋升/降级，避免人工维护集合。

```go
// 尝试从缓存中直接获取用户信息
userinfo, err: = Redis.Get("user_info_9527")
if err != nil {
    return nil, err
}

//缓存命中找到，直接返回用户信息
if userinfo != nil {
    return userinfo, nil
}

//set 检测当前是否是热数据
//之所以没有使用Bloom Filter是因为有概率碰撞不准
//如果key数量超过千个，建议还是用Bloom Filter
//这个判断也可以放在业务逻辑代码中，用配置同步做
isHotKey, err: = Redis.SISMEMBER("hot_key", "user_info_9527")
if err != nil {
    return nil, err
}

//如果是热key
if isHotKey {
    //没有找到就认为数据不存在
    //可能是被删除了
    return "", nil
}

//没有命中缓存，并且没被标注是热点，被认为是临时缓存，那么从数据库中获取
//设置更新锁set user_info_9527_lock nx ex 5
//防止多个线程同时并发查询数据库导致数据库压力过大
lock, err: = Redis.Set("user_info_9527_lock", "1", "nx", 5)
if !lock {
    //没抢到锁的直接等待1秒 然后再拿一次结果, 这是因为可能其它抢到锁的把数据给拿走了，类似singleflight实现
    //行业常见缓存服务，读并发能力很强，但写并发能力并不好
    //过高的并行刷新会刷沉缓存
    time.sleep(time.second)
        //等1秒后拿数据，这个数据是抢到锁的请求填入的
        //通过这个方式降低数据库压力
    userinfo, err: = Redis.Get("user_info_9527")
    if err != nil {
        return nil, err
    }
    return userinfo, nil
}

//拿到锁的查数据库，然后填入缓存
userinfo, err: = userInfoModel.GetUserInfoById(9527)
if err != nil {
    return nil, err
}

//查找到用户信息
if userinfo != nil {
    //将用户信息缓存，并设置TTL超时时间让其60秒后失效
    Redis.Set("user_info_9527", userinfo, 60)
    return userinfo, nil
}

// 没有找到，放一个空数据进去，短期内不再问数据库
Redis.Set("user_info_9527", "", 30)
return nil, nil
```

```go
请求 → 布隆过滤器判断是否存在 → 不存在直接返回空
                ↓ 可能存在
        Redis 查缓存 → 命中空值标记 → 返回空
                ↓ 未命中
        单飞锁(singleflight) → DB 查询
                ↓
        结果回填 Redis，不存在则写入空值并更新布隆过滤器
```


还有一种方式来缓解热点高并发查询，在每个业务服务器上部署一个小容量的Redis来保存热点缓存数据，
通过脚本将热点数据同步到每个服务器的小Redis上，每次查询数据之前都会在本地小Redis查找一下，
如果找不到再去大缓存内查询，通过这个方式缓解缓存的读取性能。

## 预读缓存，然后刷新到redis

在一些场景下面，我们会限制任务的执行，比如限制当前只能过几个任务，比如当前限制执行 8 个任务，但是我们取出来的任务数量是 16 个，那我们是不是就需要考虑一下将剩下的8个任务给
缓存到 redis 中去，这样我们下次在读取的时候就不用去扫描数据库。

```python
# 1. 先查一下 redis，
tasks = redis.ListGet()

if len(tasks) <= 16:
    return tasks

tasks := select(Tasks).where(tasks.status == "queued").limit(16)
redis.listSet(tasks.ids)
```

另外一种骚操作:

上面这个限制执行 8 个任务、预取 16 个并缓存剩余部分的设计，本质上是一种 预取（prefetch）+ 暂存。在任务队列里会进一步优化：

本地队列：从 Redis 预取一批任务到本地内存队列，然后逐个执行，减少 Redis 交互。
工作窃取（work-stealing）：如果某个节点本地队列空了，可以从其他节点的本地队列偷一半任务，平衡负载。这种模式在 Go 的协程调度里就有，你可以用类似思路在分布式任务处理中实现。
优先级细分：预取时，把不同优先级的任务分别暂存到不同的 Redis List 或 ZSet，Worker 按优先级轮询。



## 缓存降级和熔断，监控

redis 并不是完全高可用的, 需要做好熔断措施，比如 redis 请求失败达到对应的阈值之后，将会自动触发熔断操作，直接查询 DB 或者返回对应的降级数据

命中率分级：L1 本地缓存命中率、L2 Redis 命中率、最终 DB 命中率
热 key 排行榜：实时 TopN 热 key 及 QPS
缓存重建耗时（包含查 DB + 回填的时间）
缓存穿透/击穿次数
Redis 慢查询、大 key 监控
用 OpenTelemetry + Prometheus + Grafana 形成统一看板，出问题时第一时间定位。