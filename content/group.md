---
Title: 消息队列踩坑记录
Date: 2025-9-22
---

# kafka

## consumer 重平衡

在Java应用中，我们往往会使用spring-kafka组件简单的设置一下group_id, topic就开始消费消息了，其实这样会埋下巨大的安全隐患，即当消费速度过慢时有可能会触发rebalance, 这批消息被分配到另一个消费者，然后新的消费者还会消费过慢，再次rebalance, 这样一直恶性循环下去。发生这种情况最明显的标志就是日志里能看到CommitFailedException异常，然后还会带上下面一段话：

Commit cannot be completed since the group has already rebalanced and assigned the partitions to another member. This means that the time between subsequent calls to poll() was longer than the configured max.poll.interval.ms, which typically implies that the poll loop is spending too much time message processing. You can address this either by increasing the session timeout or by reducing the maximum size of batches returned in poll() with max.poll.records.
其实这段话已经很走心了，kafka的开发者已经预料到了这可能是个很容易出现的问题，所以连解决方案都给你列出来了。这里我们需要明确一下，在Kafka 0.10.1.0以后的版本中，影响rebalance触发的参数有三个，说明如下：

session.timeout.ms

这个参数定义了当broker多久没有收到consumer的心跳请求后就触发rebalance，默认值是10s。在0.10.1.0之前的版本中，由于心跳请求是在poll()拉取消息的方法中执行的，因此如果当前批次处理消息耗时太长，就会导致consumer没有机会按时发送心跳，broker认为消费者已死，触发rebalance。在0.10.1.0或更新的版本中解决了这个问题，心跳请求会在单独的线程中发送，因此就不会出现因为消息处理过长而发不出心跳的问题了。

max.poll.interval.ms

这个参数定义了两次poll()之间的最大间隔，默认值为5分钟。如果超过这个间隔同样会触发rebalance。在多数情况下这个参数是导致rebalance消息重复的关键，即业务处理消息耗时太长。有人可能会疑惑，如果5分钟都没处理完消息那肯定时出了问题，其实不然。能否在5min内处理完还取决于你每次拉取了多少条消息，如果一次拿到了成千上万条的话，5min就够呛了。

max.poll.records
这个参数定义了poll()方法最多可以返回多少条消息，默认值为500。注意这里的用词是"最多"，也就是说如果在拉取消息的时候新消息不足500条，那有多少返回多少；如果超过500条，就只返回500。这个默认值是比较坑人的，如果你的消息处理逻辑比较重，比如需要查数据库，调用接口，甚至是复杂计算，那么你很难保证能够在5min内处理完500条消息，也就是说，如果上游真的突然大爆发生产了成千上万条消息，而平摊到每个消费者身上的消息达到了500的又无法按时消费完成的话就会触发rebalance, 然后这批消息会被分配到另一个消费者中，还是会处理不完，又会触发rebalance, 这样这批消息就永远也处理不完，而且一直在重复处理。

要避免出现上述问题也很简单，那就是提前评估好处理一条消息最长需要多少时间，然后务必覆盖默认的max.poll.records参数。在spring-kafka中这个原生参数对应的参数项是max-poll-records。对于消息处理比较重的操作，建议把这个值改到50以下会保险一些。


# Rabbitmq

## 队列属性设置


创建队列时，有两个关键属性可以设置：

### 1. durable（持久化）

-   **durable=True**：队列的元数据会被写入磁盘。即使 RabbitMQ 服务重启，这个队列依然存在，只是队列里未消费的消息会丢失（除非消息本身也被声明为持久化）。
-   **durable=False**：队列仅存在于内存中。RabbitMQ 重启后，队列就消失了。

### 2. exclusive（排他）

-   **exclusive=True**：队列只能被创建它的那个连接使用。当连接断开时，队列自动删除。
-   **exclusive=False**：队列可以被多个连接共享。连接断开后，队列依然存在。

---

## 三、 四种队列组合及 RabbitMQ 4.0 的变更

两个属性可以组合出四种队列类型：

| 类型 | durable | exclusive | 特点 |
|---|---|---|---|
| 持久化共享队列 | True | False | 重启后队列存在，可多连接共享 |
| 持久化排他队列 | True | True | 重启后队列存在，但仅单连接可用 |
| 非持久化排他队列 | False | True | 重启后消失，仅单连接可用，连接断开即删除 |
| **非持久化共享队列** | **False** | **False** | 重启后消失，可多连接共享 |

**RabbitMQ 4.0 的变更：**
RabbitMQ 4.0 移除了对第四种类型——**非持久化共享队列**（durable=False, exclusive=False）的支持。

这个队列类型在官方术语中叫做 **"transient non-exclusive queue"**。

**移除原因：**
这种队列因为不持久化，重启会丢；又因为不排他，用完不会自动删除。它存活期间占用资源，但没有清晰的清理机制，容易造成资源泄漏，使用场景也非常有限。RabbitMQ 4.0 为了简化架构，直接去掉了这个选项。

---

## 四、 你的代码为什么出错

你使用的 `taskiq-aio-pika` 库，在声明队列时，默认没有指定这两个参数。
在 AMQP 协议中，如果不指定，默认值就是 `durable=False` 和 `exclusive=False`。这恰好是 RabbitMQ 4.0 禁用的那个组合。
你的代码相当于向 RabbitMQ 4.0 请求创建一个它已经不支持的东西，所以服务器直接返回了错误：

```
INTERNAL_ERROR - Feature `transient_nonexcl_queues` is deprecated.
```

这个报错就是明确告诉你：你请求创建的这个队列类型，已经废弃不可用了。

---

## 五、 解决方法

把其中一个属性改成相反的值即可，两个方案都可以：

-   **方案一：设置 `durable=True`**
    这样队列变成持久化共享队列，是允许的。

    ```python
    channel.declare_queue(queue_name, durable=True)
    ```

-   **方案二：设置 `exclusive=True`**
    这样队列变成非持久化排他队列，也是允许的。

    ```python
    channel.declare_queue(queue_name, exclusive=True)
    ```

具体选哪个，取决于你的业务需求。一般情况下，如果希望消息不因服务重启而丢失，选 **`durable=True`** 会更常见。

## 官方文档在哪里

和这次踩坑相关的，主要看 RabbitMQ 官方这几处：

### 1. 废弃特性总览（deprecation 阶段）

- [Deprecated Features](https://www.rabbitmq.com/docs/deprecated-features)
- [Deprecated Features List](https://www.rabbitmq.com/release-information/deprecated-features-list)

`transient_nonexcl_queues` 在 **4.3.0** 从 `permitted_by_default` 变成 **`denied_by_default`**。

### 2. transient 非独占队列（`durable=false` + 非 exclusive）

- [Queues - Durability](https://www.rabbitmq.com/docs/queues#durability)
- [4.3.0 Release Notes](https://github.com/rabbitmq/rabbitmq-server/releases/tag/v4.3.0)（Deprecated Features 章节）

允许旧行为需在 `rabbitmq.conf` 里显式打开：

```ini
deprecated_features.permit.transient_nonexcl_queues = true
```

### 3. `x-consumer-timeout`（classic 队列不再支持）

- [consumer_timeouts.md（rabbitmq-server 源码文档）](https://github.com/rabbitmq/rabbitmq-server/blob/main/consumer_timeouts.md)
- [RabbitMQ 4.3 Highlights - Consumer Timeouts](https://www.rabbitmq.com/blog/2026/04/23/rabbitmq-4.3-release)
- [4.3 Release Notes - Consumer Timeouts](https://techdocs.broadcom.com/us/en/vmware-tanzu/data-solutions/open-source-rabbitmq/4-3/opn-src-rabbitmq/site-release-notes.html)

要点：**4.3 起 classic 队列不再处理 consumer timeout**；`x-consumer-timeout` 作为**队列声明参数**在 classic 上会被拒绝（你看到的 `invalid arg`）。

### 4. Quorum 队列上的 consumer timeout

- [Quorum Queues - Consumer Timeout](https://www.rabbitmq.com/docs/quorum-queues#consumer-timeout)

---

## 为什么线上有的服务还能跑？

不是「参数没问题」，而是 **触发的时机和 RabbitMQ 版本不同**。

### 1. RabbitMQ 版本不同（最常见）

| 版本 | transient 非独占队列 | classic 上 `x-consumer-timeout` |
|------|---------------------|--------------------------------|
| 3.12.x | 允许 | 声明时允许，会生效 |
| 4.0–4.2 | 逐步收紧 | 仍可能声明成功 |
| **4.3.x**（你本地 `4.3.1`） | 默认**拒绝**声明 | classic 上**拒绝**该参数 |

线上若还有 **3.x / 4.2**，旧代码可能一直能跑；升到 **4.3** 后，**新声明**会失败。

### 2. 队列/Exchange 早就建好了，服务只连不重建

RabbitMQ 规则：**已存在的队列不会因为 broker 升级自动删掉**。

典型情况：

- 很久以前用 `durable=false` / 带 `x-consumer-timeout` 建好了 `vector.taskiq`
- 服务启动时若**不再 declare**（或 declare 与现有参数一致），只是 `basic.consume` → **照常消费**
- **scheduler / 新 worker 镜像**启动时会 `queue.declare` → 参数和现网不一致或带了 4.3 不支持的参数 → **崩**

所以会出现：**worker 能 `Listening started`，scheduler 起不来**。

### 3. 线上开了废弃特性白名单

若 `rabbitmq.conf` 里有：

```ini
deprecated_features.permit.transient_nonexcl_queues = true
```

则 transient 非独占队列仍可声明，和 4.3 默认行为不同。各环境配置不一致时，就会出现「有的集群能跑、有的不能」。

### 4. 服务角色不同

| 服务 | 典型行为 | 是否容易踩坑 |
|------|----------|--------------|
| **api** | 发任务、可能 declare exchange/queue | 中等 |
| **worker** | 启动时 declare + consume | 高 |
| **scheduler** | 启动时 declare（taskiq broker startup） | **最高** |
| 其他业务 | 只用现成队列、不 declare | 可能完全不受影响 |

paas-vector 里 **taskiq 的 declare 逻辑集中在 broker startup**，scheduler 几乎每次都会撞声明逻辑。

### 5. `x-consumer-timeout` 在旧队列上「挂着但不生效」

你本地队列里仍有：

```
x-consumer-timeout = 43200000  (720 分钟)
```

在 **RabbitMQ 4.3 + classic** 上：

- **已存在的队列**：参数留在元数据里，但 **4.3 不再对 classic 执行 consumer timeout**
- **新声明**带这个参数：直接 `invalid arg`

所以老 worker 可能一直在跑，新 scheduler 一声明就挂。

### 6. 镜像/代码版本不一致

线上常见：

- worker 是旧镜像（无 `durable=True`、仍有 `x-consumer-timeout`），队列是历史遗留
- 只更新了 scheduler → durable / timeout 冲突
- 或 api/worker/scheduler **不是同一次 build** → `inequivalent arg 'durable'`

---

## 和你们项目的对应关系

| 废弃/变更项 | 你们代码里原来 | 4.3 行为 | 修复 |
|-------------|---------------|----------|------|
| transient 非独占 | `durable=False`（taskiq 默认） | 拒绝声明 | `durable=True` |
| `x-consumer-timeout` on classic | 720 分钟 | 拒绝声明 | 已去掉 |
| exchange `durable` 不一致 | 新旧混用 | `PRECONDITION_FAILED` | 统一 durable + 清旧资源 |

---

## 怎么确认线上是哪种情况

在**线上** RabbitMQ 容器执行：

```bash
rabbitmqctl version
rabbitmqctl list_queues name durable arguments | grep taskiq
rabbitmqctl list_exchanges name durable | grep taskiq
cat /etc/rabbitmq/rabbitmq.conf 2>/dev/null | grep deprecated_features
```

看三点：

1. 版本是不是 **4.3.x**
2. 队列是否还带 `x-consumer-timeout`、`durable` 是否统一
3. 是否开了 `transient_nonexcl_queues` 白名单

---

**一句话**：文档在 RabbitMQ 4.3 的 [deprecated features](https://www.rabbitmq.com/docs/deprecated-features) 和 [consumer timeout 说明](https://github.com/rabbitmq/rabbitmq-server/blob/main/consumer_timeouts.md)；线上有的服务还能跑，多半是 **旧队列还在 + 没重新 declare**，或 **RabbitMQ 版本/配置更宽松**；scheduler 每次启动都要 declare，所以最先暴露问题。