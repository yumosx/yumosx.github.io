---
Title: 规则引擎设计
Date: 2025-9-26
---

目前我设计一个规则引擎分成两个部分
- 规则: when
- 执行器或者叫做 action

使用 代码表示其实就类似下面这样写法:
```java
if car.owner.hasCellPhone then premium += 100;
if car.model.theftRating > 4 then premium += 200;
if car.owner.livesInDodgyArea && car.model.theftRating > 2 
    then premium += 300;
```

