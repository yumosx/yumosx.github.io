---
Title: 消息队列踩坑记录
Date: 2025-9-22
---

# Kafka

## Consumer 重平衡（Rebalance）

在 Java 应用中，使用 spring-kafka 只需配置 `group_id` 和 `topic` 就能消费消息，但这会埋下安全隐患：消费速度过慢时触发 rebalance，消息被分配给另一个消费者，新消费者同样消费过慢，再次 rebalance，形成恶性循环。

**典型症状**：日志出现 `CommitFailedException`，并附带以下信息：

> Commit cannot be completed since the group has already rebalanced and assigned the partitions to another member. This means that the time between subsequent calls to poll() was longer than the configured max.poll.interval.ms, which typically implies that the poll loop is spending too much time message processing. You can address this either by increasing the session timeout or by reducing the maximum size of batches returned in poll() with max.poll.records.

### 影响 Rebalance 的三个关键参数（Kafka 0.10.1.0+）

#### 1. `session.timeout.ms`

Broker 多久没收到 consumer 心跳就触发 rebalance，默认 **10s**。

- 0.10.1.0 之前：心跳在 `poll()` 内执行，消息处理过久会导致无法发心跳 → 误判死亡 → rebalance
- 0.10.1.0 之后：心跳由独立线程发送，不再受消息处理耗时影响

#### 2. `max.poll.interval.ms`

两次 `poll()` 之间的最大间隔，默认 **5 分钟**。超时即触发 rebalance。

这是导致消息重复消费的**最常见原因**。能否在 5 分钟内处理完，取决于每次拉取的消息数量。

#### 3. `max.poll.records`

`poll()` 最多返回的消息条数，默认 **500**。

**坑点**：如果消息处理较重（查库、调接口、复杂计算），500 条可能无法在 5 分钟内处理完。上游突发大量消息时，每个消费者分到 500 条处理不完 → rebalance → 新消费者同样处理不完 → 反复 rebalance，消息永远消费不完。

### 解决方案

提前评估单条消息最大处理耗时，**务必覆盖默认的 `max.poll.records`**。

在 spring-kafka 中对应配置项为 `max-poll-records`。对于重操作，建议设为 **50 以下**。

---

# RabbitMQ

## 一、队列属性

创建队列时有两个关键属性：

### 1. `durable`（持久化）

| 值 | 行为 |
|---|---|
| `True` | 队列元数据写入磁盘，RabbitMQ 重启后队列仍存在（未消费消息是否丢失取决于消息本身的持久化设置） |
| `False` | 队列仅存内存，重启后消失 |

### 2. `exclusive`（排他）

| 值 | 行为 |
|---|---|
| `True` | 仅创建它的连接可使用，连接断开时自动删除 |
| `False` | 可被多个连接共享，连接断开后仍存在 |

---

## 二、四种队列组合与 RabbitMQ 4.0 变更

| 类型 | durable | exclusive | 特点 |
|---|---|---|---|
| 持久化共享队列 | True | False | 重启后存在，可多连接共享 |
| 持久化排他队列 | True | True | 重启后存在，仅单连接可用 |
| 非持久化排他队列 | False | True | 重启后消失，连接断开即删除 |
| **非持久化共享队列** | **False** | **False** | 重启后消失，可多连接共享 |

**RabbitMQ 4.0 变更**：移除了第四种——**非持久化共享队列**（`transient non-exclusive queue`）。

**移除原因**：不持久化（重启丢）+ 不排他（不会自动删除）= 占用资源却无清理机制，易造成资源泄漏。

---

## 三、踩坑：代码为什么出错

`taskiq-aio-pika` 声明队列时未指定 `durable` 和 `exclusive`，AMQP 协议默认值为 `durable=False` + `exclusive=False`，恰好命中 RabbitMQ 4.0 禁用的组合。

报错信息：

```
INTERNAL_ERROR - Feature `transient_nonexcl_queues` is deprecated.
```

### 解决方法

二选一：

**方案一：`durable=True`**（推荐，消息不因重启丢失）

```python
channel.declare_queue(queue_name, durable=True)
```

**方案二：`exclusive=True`**

```python
channel.declare_queue(queue_name, exclusive=True)
```

---

## 四、相关官方文档

### 1. 废弃特性总览

- [Deprecated Features](https://www.rabbitmq.com/docs/deprecated-features)
- [Deprecated Features List](https://www.rabbitmq.com/release-information/deprecated-features-list)

> `transient_nonexcl_queues` 在 **4.3.0** 从 `permitted_by_default` 变为 `denied_by_default`。

### 2. transient 非独占队列

- [Queues - Durability](https://www.rabbitmq.com/docs/queues#durability)
- [4.3.0 Release Notes](https://github.com/rabbitmq/rabbitmq-server/releases/tag/v4.3.0)

临时允许旧行为需在 `rabbitmq.conf` 配置：

```ini
deprecated_features.permit.transient_nonexcl_queues = true
```

### 3. `x-consumer-timeout`（classic 队列不再支持）

- [consumer_timeouts.md](https://github.com/rabbitmq/rabbitmq-server/blob/main/consumer_timeouts.md)
- [RabbitMQ 4.3 Highlights - Consumer Timeouts](https://www.rabbitmq.com/blog/2026/04/23/rabbitmq-4.3-release)

**要点**：4.3 起 classic 队列不再处理 consumer timeout；`x-consumer-timeout` 作为队列声明参数会被拒绝（`invalid arg`）。

### 4. Quorum 队列的 consumer timeout

- [Quorum Queues - Consumer Timeout](https://www.rabbitmq.com/docs/quorum-queues#consumer-timeout)

---

## 五、为什么线上有的服务还能跑？

核心原因：**触发时机和 RabbitMQ 版本不同**。

### 1. RabbitMQ 版本差异（最常见）

| 版本 | transient 非独占队列 | classic 上 `x-consumer-timeout` |
|------|---------------------|--------------------------------|
| 3.12.x | 允许 | 声明时允许，会生效 |
| 4.0–4.2 | 逐步收紧 | 仍可能声明成功 |
| **4.3.x** | 默认**拒绝**声明 | classic 上**拒绝**该参数 |

### 2. 队列早已建好，服务只连不重建

RabbitMQ 规则：**已存在的队列不会因 broker 升级自动删除**。

- 历史队列用 `durable=False` / 带 `x-consumer-timeout` 创建 → 依然存在
- 服务启动时若不重新 `declare`，只 `basic.consume` → 照常消费
- **scheduler / 新 worker** 启动时会 `queue.declare` → 参数不兼容 → 崩溃

→ 所以会出现 **worker 能 `Listening started`，scheduler 起不来**。

### 3. 线上开了废弃特性白名单

```ini
deprecated_features.permit.transient_nonexcl_queues = true
```

各环境配置不一致时，会出现「有的集群能跑、有的不能」。

### 4. 服务角色不同

| 服务 | 典型行为 | 踩坑风险 |
|------|----------|----------|
| api | 发任务、可能 declare exchange/queue | 中 |
| worker | 启动时 declare + consume | 高 |
| scheduler | 启动时 declare（broker startup） | **最高** |
| 其他业务 | 只用现成队列，不 declare | 低 |

### 5. `x-consumer-timeout` 在旧队列上「挂着但不生效」

RabbitMQ 4.3 + classic 队列：

- **已存在队列**：参数留在元数据，但不再执行 consumer timeout
- **新声明**带此参数：直接 `invalid arg`

→ 老 worker 一直在跑，新 scheduler 一声明就挂。

### 6. 镜像/代码版本不一致

- worker 旧镜像（无 `durable=True`、仍有 `x-consumer-timeout`），队列是历史遗留
- 只更新了 scheduler → durable / timeout 冲突
- api/worker/scheduler 不是同一次 build → `inequivalent arg 'durable'`

---

## 六、排查命令

在线上 RabbitMQ 容器执行：

```bash
rabbitmqctl version
rabbitmqctl list_queues name durable arguments | grep taskiq
rabbitmqctl list_exchanges name durable | grep taskiq
cat /etc/rabbitmq/rabbitmq.conf 2>/dev/null | grep deprecated_features
```

确认三点：

1. 版本是否为 **4.3.x**
2. 队列是否带 `x-consumer-timeout`、`durable` 是否统一
3. 是否开了 `transient_nonexcl_queues` 白名单

---

## 总结

| 废弃/变更项 | 原代码 | 4.3 行为 | 修复 |
|-------------|--------|----------|------|
| transient 非独占 | `durable=False`（taskiq 默认） | 拒绝声明 | `durable=True` |
| `x-consumer-timeout` on classic | 720 分钟 | 拒绝声明 | 去掉该参数 |
| exchange `durable` 不一致 | 新旧混用 | `PRECONDITION_FAILED` | 统一 durable + 清旧资源 |

**一句话**：文档在 RabbitMQ 4.3 的 [deprecated features](https://www.rabbitmq.com/docs/deprecated-features) 和 [consumer timeout 说明](https://github.com/rabbitmq/rabbitmq-server/blob/main/consumer_timeouts.md)；线上有的服务还能跑，多半是 **旧队列还在 + 没重新 declare**，或 **RabbitMQ 版本/配置更宽松**；scheduler 每次启动都要 declare，所以最先暴露问题。
