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