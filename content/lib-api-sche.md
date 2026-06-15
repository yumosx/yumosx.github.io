---
Title: APSchduler 库学习 
Date: 2025-10-22
---

## APSchduler 介绍

**在你当前的代码中使用 `AsyncIOScheduler` 时，默认情况下会等到上一个 `self.run()` 执行完毕之后，才会开启下一个 job**。

这正是 APScheduler 的 **interval trigger**（间隔触发器）的默认行为，具体由参数 `coalesce` 和 `max_instances` 控制。

### 默认行为详解（APScheduler 4.x / 5.x）

当你这样添加任务：

```python
self.scheduler.add_job(self.run, 'interval', seconds=2)
```

默认参数是：

- `coalesce=True`：如果因为上一个任务还没执行完，导致错过了多个触发时间点，则合并（coalesce）成只执行一次。
- `max_instances=1`：同一 job 同时最多只能有 1 个实例在运行。

**结果就是**：

- 每隔 2 秒，scheduler 会尝试触发 `self.run()`。
- 如果上一个 `self.run()` 还没执行完（比如数据库查询或 Kafka 发送耗时 > 2 秒），新触发会被**拒绝**，不会并发执行。
- 等上一个执行完毕后，下一个触发时间到来时，才会执行下一个。
- 如果期间错过了多个周期，只会在上一个结束后立即补执行一次（因为 coalesce=True），而不是补跑所有错过的。

这正好避免了任务堆积和重复处理同一批任务，非常适合你现在的“轮询数据库任务并发送 Kafka”的场景。

### 常见情况举例

| 场景 | self.run() 执行耗时 | 实际执行频率 | 说明 |
|------|----------------------|--------------|------|
| 正常 | 0.5 秒 | 每 2 秒一次 | 严格按间隔执行 |
| 偶尔慢 | 3 秒 | 大约每 3-4 秒一次 | 等上一个结束，下一个触发时执行 |
| 持续慢 | 10 秒 | 每 10 秒一次 | 完全串行，不会并发 |
| 错过多个 | 上一个跑了 10 秒，错过 4 个周期 | 上一个结束后立即再跑一次（补一次） | 因 coalesce=True |

### 如果你想改变这个行为

| 需求 | 修改方式 | 代码示例 |
|------|----------|----------|
| 允许并发执行（不管上一个是否完成，都在新触发时启动新实例） | 设置 `max_instances` 大于 1 | `max_instances=10` |
| 错过的不补执行（严格每隔 fixed 时间执行一次，不堆积） | `coalesce=False` + `misfire_grace_time=None` | 一般不推荐 |
| 错过的不执行（只在准点触发，错过就跳过） | `coalesce=False` | 较少用 |

**推荐配置（在你的场景下）**：

```python
self.scheduler.add_job(
    self.run,
    'interval',
    seconds=2,
    id='task_poller',
    coalesce=True,          # 默认就是 True，建议保留
    max_instances=1,        # 默认就是 1，建议保留（防止重复处理同一批任务）
    misfire_grace_time=30   # 可选：允许迟到 30 秒内仍执行，防止轻微延迟被丢弃
)
```

### 总结回答你的问题

**是的**，在你当前的设置下（也是默认设置），APScheduler 会等到 `self.run()` 完全执行完毕之后，才会开始下一个 job 的执行。

这保证了：

- 任务串行执行，不会并发处理同一批任务（避免重复发送 Kafka）。
- 不会因为任务变慢导致大量任务堆积。
- 行为稳定、可预测。

完全符合你原来的 `while True: await self.run(); await asyncio.sleep(2)` 的语义，甚至比手动 sleep 更可靠（因为 interval 是基于“上一个结束时间 + 间隔”计算的，不会漂移）。

所以你无需担心并发问题，当前配置已经是最安全、最合适的。