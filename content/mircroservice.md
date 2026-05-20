---
Title: 高性能系统设计之注册中心设计
Date: 2025-9-23
---

# 注册中心设计

## 注册表
```go
type cluster struct {
    endpoints  []string                    // etcd 集群地址
    key        string                      // 集群标识
    watchers   map[watchKey]*watchValue    // 监听器映射
    watchGroup *threading.RoutineGroup     // 协程组
    done       chan lang.PlaceholderType
    lock       sync.RWMutex
}
```

框架中的注册表
```go
type Registry struct {
    clusters map[string]*cluster  // etcd 集群映射
    lock     sync.RWMutex
}
```

## 服务发布

```go
Publisher.KeepAlive()
       │
       ├─▶ doRegister()
       │       │
       │       ├─▶ Grant(TTL)                   // 申请租约
       │       └─▶ Put(key, value, WithLease)  // 写入键值
       │
       └─▶ keepAliveAsync()
               │
               ├─▶ KeepAlive(lease)   // 保持心跳
               └─▶ Watch(key)         // 监听删除事件，自动重注册
```

1. 服务注册

```go
func (p *Publisher) doRegister() (internal.EtcdClient, error) {
	// 1. 获取对应的服务, 底层会调用 etOrCreateCluster 这个函数
    // 如果不存在则创建一个新的集群
    cli, err := internal.GetRegistry().GetConn(p.endpoints)
	if err != nil {
		return nil, err
	}
    // 2. 注册服务
	p.lease, err = p.register(cli)
	return cli, err
}
```

注册到 etcd 流程:

```go
func (p *Publisher) register(client internal.EtcdClient) (clientv3.LeaseID, error) {
	resp, err := client.Grant(client.Ctx(), TimeToLive)
	if err != nil {
		return clientv3.NoLease, err
	}

	lease := resp.ID
	if p.id > 0 {
		p.fullKey = makeEtcdKey(p.key, p.id)
	} else {
		p.fullKey = makeEtcdKey(p.key, int64(lease))
	}
    // 写入对应的键值对
	_, err = client.Put(client.Ctx(), p.fullKey, p.value, clientv3.WithLease(lease))

	return lease, err
}
```

2. 保活机制

```go
// 通过客户端保持返回一个 chan
ch, err := cli.KeepAlive(cli.Ctx(), p.lease)

//通过 cli.Watch 监听 p.fullKey，且使用 WithFilterPut() 过滤掉 Put 事件，只关心 Delete 事件
wch := cli.Watch(cli.Ctx(), p.fullKey, clientv3.WithFilterPut())
for _, evt := range c.Events {
    if evt.Type == clientv3.EventTypeDelete {
        _, err := cli.Put(cli.Ctx(), p.fullKey, p.value, clientv3.WithLease(p.lease))
    }
}

/*
当 ch 通道关闭（ok == false），说明租约已失效（如租约过期、连接断开等），
会先调用 p.revoke(cli) 撤销租约，然后尝试 p.doKeepAlive() 重新建立保活（可能重新申请租约并放置键值）。
*/
if !ok {
	p.revoke(cli)
	if err := p.doKeepAlive(); err != nil {
		logc.Errorf(cli.Ctx(), "etcd publisher KeepAlive: %v", err)
	}
	return
}

/*
收到 p.pauseChan 信号后，撤销当前租约，并阻塞等待 p.resumeChan 或退出信号。
恢复时调用 p.doKeepAlive() 重新开始保活，并退出当前 goroutine（调用者可能需要重新启动一个新的保活循环）。
这种设计可以在某些场景（如配置变更、服务降级）下临时停止保活，之后再恢复。
*/
case <-p.pauseChan:
    logc.Infof(cli.Ctx(), "paused etcd renew, key: %s, value: %s", p.key, p.value)
    p.revoke(cli)
    select {
    case <-p.resumeChan:
        if err := p.doKeepAlive(); err != nil {
            logc.Errorf(cli.Ctx(), "etcd publisher KeepAlive: %v", err)
        }
        return
    case <-p.quit.Done():
        return
    }
```

服务注册流程:

```Graph
服务启动
    │
    ▼
RpcServerConf.HasEtcd() ──── Yes ──▶ NewRpcPubServer()
    │                                      │
    No                                     ▼
    │                              registerEtcd()
    ▼                                      │
NewRpcServer()                             ▼
    │                              figureOutListenOn()
    │                             (处理 0.0.0.0 → 实际 IP)
    ▼                                      │
server.Start()                             ▼
                                    NewPublisher()
                                            │
                                            ▼
                                    Publisher.KeepAlive()
                                            │
                                    ┌───────┴───────┐
                                    │               │
                                    ▼               ▼
                                Grant(TTL)    Put(key, addr)
                                    │               │
                                    └───────┬───────┘
                                            ▼
                                    etcd: key → addr (with lease)
```

客户端初始化

```
 客户端初始化
       │
       ▼
   RpcClientConf.BuildTarget()
       │
       ├─▶ 有 Endpoints ──▶ BuildDirectTarget() ──▶ direct://addr1,addr2
       │
       └─▶ 有 Etcd ──▶ BuildDiscovTarget() ──▶ discov://hosts/key
                              │
                              ▼
                      gRPC Resolver (discovBuilder.Build)
                              │
                              ▼
                      NewSubscriber(hosts, key)
                              │
                              ▼
                      Registry.Monitor()
                              │
                              ▼
                      cluster.monitor()
                              │
                              ├─▶ load()        // 首次加载
                              │       │
                              │       └─▶ etcd.Get(key, WithPrefix)
                              │
                              └─▶ watch()       // 持续监听
                                      │
                                      └─▶ etcd.Watch(key)
                                              │
                                              ▼
                                      handleWatchEvents()
                                              │
                                      ┌───────┴───────┐
                                      │               │
                                      ▼               ▼
                                 OnAdd(kv)     OnDelete(kv)
                                      │               │
                                      └───────┬───────┘
                                              ▼
                                      Container 更新
                                              │
                                              ▼
                                      listeners 通知
                                              │
                                              ▼
                                      gRPC ClientConn.UpdateState()
```

```go
func NewSubscriber(endpoints []string, key string, opts ...SubOption) (*Subscriber, error) {
	sub := &Subscriber{
		endpoints: endpoints,
		key:       key,
	}
	for _, opt := range opts {
		opt(sub)
	}
	if sub.items == nil {
		sub.items = newContainer(sub.exclusive)
	}

	if err := internal.GetRegistry().Monitor(endpoints, key, sub.exactMatch, sub.items); err != nil {
		return nil, err
	}

	return sub, nil
}
```