---
Title: redis 缓存设计方案 
Date: 2025-10-02
---

## 老系统 redis 改造


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


还有一种方式来缓解热点高并发查询，在每个业务服务器上部署一个小容量的Redis来保存热点缓存数据，
通过脚本将热点数据同步到每个服务器的小Redis上，每次查询数据之前都会在本地小Redis查找一下，
如果找不到再去大缓存内查询，通过这个方式缓解缓存的读取性能。