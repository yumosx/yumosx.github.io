---
Title: RabbitMQ 与 Aio-Pika
Date: 2025-9-22
---

整个 rabbitmq 分成了 四个部分

1. producer: 用来生成对应的消息, 将消息发送出去
2. exchange: 用来按照对应的 router 把消息转发到对应的 queue 上面去
3. queue： 负责持久化对应的消息，兔子mq 的消息持久化机制是通过 Erlang 的 分布式数据库做持久化的
4. consumer: 负责去消费对应的消息

整个消息发送层和接收层都是遵守一个叫做 AMQP 的协议，这个协议是建立在 TCP 协议之上的,
客户端和 broker 之间的连接叫做 connection, 这个 connection 中包含了 channel，

Connection 是指 TCP 连接，Channel 是 Connection 中的虚拟连接。两者的关系是：一个客户端和一个 Broker 之间只会建立一条 TCP 连接，就是指 Connection。Channel（虚拟连接）的概念在这个连接中定义，一个 Connection 中可以创建多个 Channel。客户端和服务端的实际通信都是在 Channel 维度通信的。这个机制可以减少实际的 TCP 连接数量，从而降低网络模块的损耗。从设计角度看，也是基于 IO 复用、异步 I/O 的思路来设计的。


## 生产消息

下面与 [aio-pika Quick start](https://docs.aio-pika.com/quick-start.html) 一致：`connect_robust` 在断线后会重连；`Message` 用 `body=` 显式传入负载；先 `declare_queue` 再发布，保证队列存在。

```python
import asyncio

import aio_pika


async def main() -> None:
    connection = await aio_pika.connect_robust("amqp://guest:guest@localhost/")

    async with connection:
        channel = await connection.channel()
        queue = await channel.declare_queue("hello")

        await channel.default_exchange.publish(
            aio_pika.Message(body=b"Hello World!"),
            routing_key=queue.name,
        )


if __name__ == "__main__":
    asyncio.run(main())
```


## 消费消息

用 `queue.iterator()` 持续拉取；`async with message.process()` 在块正常结束时自动 ack（与官方 Simple consumer 一致）。`set_qos(prefetch_count=...)` 限制未确认消息数量，避免消费者被压垮。

```python
import asyncio

import aio_pika


async def main() -> None:
    connection = await aio_pika.connect_robust("amqp://guest:guest@localhost/")

    async with connection:
        channel = await connection.channel()
        await channel.set_qos(prefetch_count=10)

        queue = await channel.declare_queue("hello")

        async with queue.iterator() as queue_iter:
            async for message in queue_iter:
                async with message.process():
                    print(message.body.decode())


if __name__ == "__main__":
    asyncio.run(main())
```