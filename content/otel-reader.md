---
Title: OTEL Manual Reader 组件
Date: 2025-9-20
---

## otel 的 manual reader 组件

首先我们需要看下这个接口是如何使用的

1. 创建一个 manual reader 组件, 并传递给 meter provider
```
reader := otel.NewManualReader()
mp := metrics.NewMeterProvider(metrics.WithReader(reader))
```
2. 使用 collect 接口去收集对应的 metric
```
var got metricdata.ResourceMetrics
reader.Collect(ctx, &got)
```
3. 我们查看一下 got 中是否有我们期望的 metric
```
assert.Equal(t, 1, len(got.ResourceMetrics))
```