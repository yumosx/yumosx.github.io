---
Title: 高性能任务调度系统设计实战 
Date: 2025-9-22
---

在我们的日常开发中经常会碰到一些长时间，异步任务的执行，比如一个执行很耗时间的任务，可以长达 20多分钟, 这些任务的执行显然不能设计成阻塞形式的，如果是阻塞形式的系统很显然是扛不住的，所以这种系统很显然需要设计成非阻塞的异步的。

设计这种系统可用大概拆分出这样几个架构:
1. 调度对应的任务，调度策略可以有定时调度，轮询调度，抢占式调度
2. 将调度的任务发送到消息队列，你可以设置多个队列/rabbit mq, 或者多个 topic
3. 安排多个消费者组，分别去消费对应的 broker 和消费者
4. 并行执行对应的任务
5. 实时返回对应的任务进度，推送
7. 当任务执行失败的时候如何设计调度任务去重新执行，同时不要影响主任务执行
8. 熔断设计，当出现连续多个不同类型的任务执行失败的时候，需要考虑熔断
9. 可观测性设计，可观测性设计非常重要在这种任务执行中

下面我们一条条拆开分析

## 调度策略

首先调度策略的选择上，我们是选择轮询调度还是，定时调度，这两个都有好处，

首先轮询调度的好处就是对 CPU 不是很友好，但是实时性比较好。因为一旦有新的任务进来，轮询调度可以及时发现，而定时调度则需要等待定时时间，才能发现新的任务。因此轮询调度比较适合任务执行时间短、且对实时性要求高的场景（短任务配合高频轮询才能摊薄 CPU 开销；如果任务本身就长，轮询会让 worker 大量空转，CPU 反而吃不消）。

定时调度的好处对 CPU 较好，但是实时性比较差，因为定时调度需要等待定时时间，才能发现新的任务。但是在某些场景下我觉得是很合适的，比如一个任务执行时间很长，且你限制了同一时刻任务执行的数量，出于资源考虑，可以考虑定时调度。

在调度中缓存策略的设计，需要考虑两个方面：什么东西应该放入缓存、放入缓存的过期时间是多少。
根据我的实践，我觉得下面几个东西应该放到缓存中:

（1）当前 worker 中任务执行的数量，这个数量的计算是通过 `select count(*) from tasks where status = 'running'`；如果不加索引的话性能也是非常差，加上索引之后性能会提高很多。由于任务执行时间较长，running 数量在短期窗口内变化很小（主要取决于任务的进入/退出速率），所以适合缓存，缓存的过期时间可以设置为 cron 周期的 2 倍。这样既能容忍一次漏读，也不会因为缓存值过于陈旧而误判 worker 容量（前提是 cron 周期 T 远小于任务平均执行时长 D，否则缓存会严重滞后于真实并发数，导致超出 worker 容量限流）。

（2）预取下一批任务：在上一次调度的时候，就把下一批要调度的任务一起查出来。也就是说原来你的 SQL 只 `limit 8`，现在改为 `limit 16`，把下一批任务一并查出放入缓存。这样原来需要两次 IO，现在只需要一次 IO，减少了主路径的数据库访问次数。

## 消息队列设计

因为我们是异步任务，所以消息队列的设计就比较重要了。Topic 划分的本质是按**消费语义**切分：不同 topic 的消费速度、积压容忍度、处理逻辑都不一样，混在同一个 topic 里会出现 head-of-line blocking（一条慢消息阻塞后续所有消息）。

如果你使用的是 Kafka，**核心 topic 至少要设计 3 个**：

1. **任务调度 topic**：向 worker 分发待执行任务
2. **高优先级任务 topic**：用于抢占式插队，避免被低优先级任务阻塞
3. **任务失败 topic**：消费失败任务的重试入口

> 为什么"高优先级"要单独开 topic？独立 topic 能物理隔离，避免被低优先级任务阻塞；代价是 topic 数量膨胀、运维成本上升。另一种方案是在同 topic 内加 `priority` 字段，由消费者侧实现优先级队列——更省 topic，但消费者实现复杂，且仍可能被 head-of-line 阻塞。两种方案各有 trade-off，按业务规模选择。

结合文章开头的架构清单（第 5 条"实时返回任务进度"、第 7 条"失败重试"），还建议额外增加两个 topic：

- **任务进度 topic**：worker 实时回传任务进度（架构第 5 条）
- **死信 topic（DLQ）**：多次重试仍失败、或消息格式无法解析时进入此 topic，等待人工介入

### Consumer Group 与 Partition

通常为每个 topic 设计一个 consumer group 来消费对应的消息，目的是**并行度隔离**（不同 topic 互不影响消费速度）。例外场景：监控/审计旁路消费、跨 topic 聚合（如 Flink 同时订阅多个 topic），可以一个 topic 对应多个 group。

**关键设计**（也是这里最容易漏掉的）：每个 topic 的 **partition 数 ≥ worker 实例数**，并预留扩容空间。原因是一个 consumer group 内，**partition 数 = 最大并行度**；partition 数定下来后只能加不能减（减会丢消息），所以宁可一次多配。

| partition 数 | consumer 数 | 实际并行度 | 结果 |
| --- | --- | --- | --- |
| 2 | 4 | 2 | ⚠️ 2 个 worker 闲置 |
| 4 | 4 | 4 | ✅ 刚好 |
| 8 | 4 | 4 | ✅ 4 个 worker 都工作，4 个 partition 排队等 |
| 4 | 8 | 4 | ⚠️ 4 个 worker 闲置 |

在一个 consumer group 内， 一个 partition 同时只能被一个 consumer 消费 （不能像线程池那样一个 partition 分给多个 consumer 抢）。
但是，一个 consumer 可以消费多个 partition。所以 partition 数 >= 最大并行度。这样就可以把 worker 给打满。

```txt
broker 上的 topic: cache-consumer  (假设 6 个 partition)
┌─────────┬─────────┬─────────┬─────────┬─────────┬─────────┐
│ part 0  │ part 1  │ part 2  │ part 3  │ part 4  │ part 5  │
└────┬────┴────┬────┴────┬────┴────┬────┴────┬────┴────┬────┘
     │         │         │         │         │         │
     │  rebalance 把 6 个 partition 摊到 N 个 consumer
     ▼         ▼         ▼         ▼         ▼         ▼

process A (group=cache-consumer)            process B (group=cache-consumer)
└── client = ConsumerGroup                   └── client = ConsumerGroup
    └── *Consumer (1 个)                        └── *Consumer (1 个)
        ├── ConsumeClaim goroutine p0             ├── ConsumeClaim goroutine p2
        ├── ConsumeClaim goroutine p1             ├── ConsumeClaim goroutine p4
        └── ConsumeClaim goroutine p5             └── ConsumeClaim goroutine p3
```

一个 partition = 一个只能追加的日志（append-only log）

```txt
一个 partition = 一个只能追加的日志（append-only log）

partition 0:
┌──────┬──────┬──────┬──────┬──────┬──────┬──────┬──────┐
│ msg0 │ msg1 │ msg2 │ msg3 │ msg4 │ msg5 │ msg6 │ ...  │
└──────┴──────┴──────┴──────┴──────┴──────┴──────┴──────┘
  offset=0  1      2      3      4      5      6
```
worker 设计:

```txt
Kafka topic（4 个 partition）
┌─────────┬─────────┬─────────┬─────────┐
│ part 0  │ part 1  │ part 2  │ part 3  │
└────┬────┴────┬────┴────┬────┴────┬────┘
     │         │         │         │
     ↓         ↓         ↓         ↓
┌─────────┬─────────┬─────────┬─────────┐
│Consumer │Consumer │Consumer │Consumer │
│   +     │   +     │   +     │   +     │
│ Worker  │ Worker  │ Worker  │ Worker  │  ← 每个进程：1 个 consumer + 1 个 worker
└─────────┴─────────┴─────────┴─────────┘
   任务1     任务2     任务3     任务4
   ↑ 4 个任务真正并发
```

## 幂等设计

每次 schedule 时为 task 生成新的 effective_id（UUID）并写入 DB，同时把 effective_id 一起发到消息里。worker 侧用 UPDATE ... WHERE id=? AND status='running' 原子抢占执行权，抢不到的 worker（affected_rows=0）说明任务已被其他 worker 抢占或已完成，直接 ack 消息不执行。effective_id 主要用于 日志/排查 （区分是哪次调度触发的执行），不是幂等的核心机制。
```
A:10:00 -> update task set status='running', effective_id=? where id=? and status='pending' -> affected_rows=1 -> 执行任务
B:10:00 -> update task set status='running', effective_id=? where id=? and status='pending' -> affected_rows=0 -> 拒绝执行
```


## 参数自动升级降级策略

自适应调级：策略等级与对应参数定义。

L0 NORMAL       - 正常运行
L1 WARNING      - 预警：降低单次投递量，告警
L2 DEGRADED     - 降级：限制并发，缩小允许的 task kind
L3 CIRCUIT_OPEN - 熔断：暂停普通任务，仅保留重试
L4 EMERGENCY    - 紧急：完全停止调度，等待人工恢复


## 任务动态执行上下文

在数据库表中，每个任务的每个阶段都会存在一个上下文，来存储任务的执行状态。例如: 任务1 执行到阶段1，上下文为: {"stage": 1, "status": "pending"}。这个上下文会在任务执行过程中，动态更新。这样在重跑的时候，就可以直接跳过已经执行过的阶段。并拿到他的上下文。

```python
class TaskContext:
    def __init__(self, task_id: str, stage: int, status: str):
        self.task_id = task_id
        self.stage = stage
        self.status = status
        self.context = {}
    def set(self, key: str, value: str):
        self.context[key] = value
    def get(self, key: str) -> str:
        return self.context.get(key, None)
```

## 失败任务重新调度

失败任务走的是一个单独的 topic， 用于重试。这样是避免失败任务阻塞正常任务的执行。因为很多失败任务因为网络波动，第三方服务 down 导致的。

## serverless 服务设计


## 可观测性设计

这是一个多服务系统，一旦一个服务出现问题，排查起来比较麻烦。所以需要设计一个可观测性机制，来监控这个链路的性能和稳定性。
为此我设计了一个 trace id，来跟踪每个请求的链路。同时设计了一个多级id， 设计了一个 trace log 系统

worker, scheduler, 都需要设计对应的指标:

- worker 并发数
- worker 执行时间
- worker 执行失败数
- worker 执行成功数
- worker 执行任务数

- scheduler 执行任务数
- scheduler 排队任务数量
- scheduler 执行失败数
- scheduler 执行成功数