---
Title: OTLP 导出器的部分失败问题
Date: 2025-9-20
---

OTLP 是 opentelemetry 中的协议传输标准，在收集到对应的metrics， trace，log 这些数据之后，我们往往会把对应的数据发送到远端，比如对应的 collector
对于整个数据传输的结果其实是存在三种状态的:

1. 失败状态: 比如因为网络问题出现的失败
2. 部分失败: 有些数据发送成功但是有些数据发送失败, 出现这种情况的原因可能是服务端拒绝了某些连接
3. 发送成功

当出现发送失败之后最先做的就是进行重试, OTLP 官方采用的重试策略是使用指数退避算法，

```
type RequestFunc func(context.Context, func(context.Context) error) error
```

如果是部分失败, 是不会去重试的, 因为如果重试的话很可能会导致出现数据覆盖的情况
```
The client MUST NOT retry the request when it receives a partial success response where the partial_success is populated.
```

下面这段代码处理的方式其实也说明了这一点, 当出现部分失败的时候是不会 return 一个 err 的
```
return errors.Join(uploadErr, c.requestFunc(ctx, func(ctx context.Context) error {
    success = count
    resp, err := c.lsc.Export(ctx, &collogpb.ExportLogsServiceRequest{
        ResourceLogs: rl,
    })
    if resp != nil && resp.PartialSuccess != nil {
        msg := resp.PartialSuccess.GetErrorMessage()
        n := resp.PartialSuccess.GetRejectedLogRecords()
        success -= n
        if n != 0 || msg != "" {
            err := errPartial{msg: msg, n: n}
            uploadErr = errors.Join(uploadErr, err)
        }
    }
    // nil is converted to OK.
    if status.Code(err) == codes.OK {
        // Success.
        return nil
    }
    success = 0
    return err
}))
```