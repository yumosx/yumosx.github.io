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

下面我用一个真实场景来推算参数。

**场景目标**：48 小时内跑完 720 个任务。
```
目标吞吐量 = 720 个 / 48 h = 15 个/h
```
即每个小时要处理完 15 个任务。

**已知现状**：并发度（worker 数）= 4 时，48h 跑完 392 个任务。

```
当前吞吐量 = 392 / 48 = 8.17 个/h  ≈ 8.2 个/h
```

套 Little's Law：

```
并发度 = 吞吐量 × 平均执行时间
4      = 8.2    × t
t      = 4 / 8.2 = 0.488 h ≈ 29 分钟
```

**平均每个任务执行时间 ≈ 29 分钟**。

> 注意：这里的"吞吐量"是指**系统实际处理速率**，不是单 worker 的速率。
> `并发度 = 吞吐量 × 平均执行时间` 的物理含义是：
> 任意时刻，同时在执行的任务数 = 每小时处理几个 × 每个任务占几小时。
> 单位必须对齐：吞吐量用 个/h，时间就必须用 h。


仍用 Little's Law，已知目标吞吐量 15 个/h，平均执行时间 0.488 h：

```
所需并发度 = 15 × 0.488 = 7.32
```

向上取整 → **8**（即同时有 8 个任务在跑即可达成目标）。

Kafka 的关键约束（见上文 Consumer Group 与 Partition 章节）：

| 约束 | 含义 |
| --- | --- |
| `partition 数 = 一个 group 的最大并行度` | 8 个 worker 要打满，partition 必须 ≥ 8 |
| `partition 数 ≥ worker 数` | 否则会有 worker 闲置 |
| `partition 只能加不能减` | 必须一次给够，预留扩容空间 |

所以这个场景下：

| 参数 | 取值 | 理由 |
| --- | --- | --- |
| topic 数 | **1**（任务调度 topic） | 这里的 worker 只消费"任务调度 topic"；高优先级、失败重试等是另外的 topic 和 worker，不算进这次推算 |
| partition 数 | **10** | 略大于 worker 数 8，预留 2 个 partition 扩容 |
| consumer 数 | **8** | = worker 数，一个进程 = 一个 consumer + 一个 worker |
| worker 数 | **8** | 上面算出的目标并发度 |

---

Consumer / Worker 配比的三种方案

上面算的是"1 consumer = 1 worker"的简化模型。实际上 consumer 和 worker 的关系可以解耦，三种主流方案各有适用场景。

如果 partition 内的任务**有顺序依赖**（必须按 offset 顺序执行），那同 partition 内开多个 worker 没意义——还是串行。
如果任务之间**无依赖**，那 worker 池才有意义。

方案 A：1 Consumer × 1 Worker（1:1 配比）

```
进程 1 (consumer + worker) ← partition 0
进程 2 (consumer + worker) ← partition 1
...
进程 N (consumer + worker) ← partition N-1
```

**总并发度 = partition 数 = 进程数**

| 项 | 说明 |
| --- | --- |
| 优点 | 简单清晰、故障定位容易、任务隔离好 |
| 缺点 | 进程数多、运维成本高、不适合 I/O 密集型任务 |
| 适用 | CPU 密集型任务（转码、压缩、计算） |
| 你的场景 | `partition=8, worker=8`，最朴素方案 |

---

方案 B：1 Consumer + 进程内 Worker 池

```
进程 1 (1 consumer + M workers) ← partition 0
```

**总并发度 = 1 × M = M（受 partition 数 = 1 限制）**

| 项 | 说明 |
| --- | --- |
| 优点 | 进程数少、复用连接池、部署简单 |
| 缺点 | 并行度上限被 partition=1 锁死，无法水平扩展 |
| 适用 | 单分区就够用 + 想省部署成本 |
| 你的场景 | ❌ 达不到目标并发度 8 |

**注意**：这种方案下 worker 池其实没意义——partition 限制了同时只能拉 1 条消息处理。所以实际还是串行。

---

方案 C：N Consumer + 进程内 Worker 池（推荐）

```
进程 1 (1 consumer + M workers) ← partition 0
进程 2 (1 consumer + M workers) ← partition 1
...
进程 N (1 consumer + M workers) ← partition N-1
```

**总并发度 = 进程数 × 每进程 worker 数 = N × M**

| 项 | 说明 |
| --- | --- |
| 优点 | 进程级 + 任务级两层并行，复用连接池，故障影响面可控 |
| 缺点 | 单进程内 worker 互相竞争（DB 连接、内存） |
| 适用 | I/O 密集型任务（HTTP 调用、DB 查询、文件读写） |
| 你的场景 | ✅ `partition=4, 进程=4, 每进程 worker=2`，总并发度 8 |

你的场景应用方案 C 推算

**已知条件：**
- 目标并发度：8
- 任务平均时长：29 分钟（0.488h）
- 任务特征：调接口、查库 → I/O 密集

**worker 数公式：**

```
每进程 worker 数 = 1 + (I/O 等待时间 / CPU 计算时间)
```

假设单任务 29 分钟中，4 分钟 CPU 计算、25 分钟等 I/O：

```
worker 数 = 1 + (25 / 4) = 1 + 6.25 ≈ 7
```

但实际**不超过 4~8**（worker 多了爆 DB 连接池和内存）。

**折中取 2 worker/进程**（既能填满 I/O 等待，又不会过载）。

**最终配置：**

```yaml
topic: task-schedule
partition: 4
consumer 进程数: 4
每进程 worker 数: 2
总并发度: 4 × 2 = 8 ✅
```

**相比方案 A 的优势：**
- 进程数 4 vs 8：部署成本减半
- 每进程 2 worker：任务有 I/O 等待时互相补位
- partition 4 vs 8：Kafka 资源减半
- 单进程故障影响 25% 流量（vs 方案 A 的 12.5%）

**注意事项：**
- 每 worker 占一份 DB 连接/HTTP 客户端 → 2 worker = 2 倍资源占用，需评估下游容量
- worker 池需要任务队列（channel）+ 优雅退出机制
- 同 partition 内任务如果有时序要求，worker 池无效

---

| 维度 | 方案 A | 方案 B | 方案 C |
| --- | --- | --- | --- |
| 架构 | N 进程 × (1c+1w) | 1 进程 × (1c+Mw) | N 进程 × (1c+Mw) |
| 总并发度 | N | 1（受 partition 限）| N × M |
| 进程数 | 多 | 少 | 中 |
| 适合任务类型 | CPU 密集 | — | I/O 密集 |
| 你的场景 | partition=8, worker=8 | ❌ | partition=4, worker=2×4 |
| 复杂度 | 低 | 低 | 中 |

---

## 幂等设计

每次 schedule 时为 task 生成新的 effective_id（UUID）并写入 DB，同时把 effective_id 一起发到消息里。worker 侧用 UPDATE ... WHERE id=? AND status='pending' 原子抢占执行权，抢不到的 worker（affected_rows=0）说明任务已被其他 worker 抢占或已完成，直接 ack 消息不执行。effective_id 主要用于 日志/排查 （区分是哪次调度触发的执行），不是幂等的核心机制。


## 参数自动升级降级策略

自适应调级：策略等级与对应参数定义。

L0 NORMAL       - 正常运行
L1 WARNING      - 预警：降低单次投递量，告警
L2 DEGRADED     - 降级：限制并发，缩小允许的 task kind
L3 CIRCUIT_OPEN - 熔断：暂停普通任务，仅保留重试
L4 EMERGENCY    - 紧急：完全停止调度，等待人工恢复



## 失败任务重新调度

失败任务走的是一个单独的 topic， 用于重试。这样是避免失败任务阻塞正常任务的执行。

## 可观测性设计

这是一个多服务系统，一旦一个服务出现问题，排查起来比较麻烦。所以需要设计一个可观测性机制，来监控这个链路的性能和稳定性。
为此我设计了一个 trace id，来跟踪每个请求的链路。同时设计了一个多级id， 设计了一个 trace log 系统