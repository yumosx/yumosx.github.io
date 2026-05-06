---
Title: Python ORM 
Date: 2025-9-22
---

## session

首先一句话解释一下 session 是一个数据库会话，它是一个用对象抽象过程的简单接口，用于与数据库进行交互。

## add 方法

add 方法用于将对象添加到会话中，等待后续 commit 方法提交到数据库。当 commit 之后对象就会过期
但是当你访问对象属性时，会话会自动重新加载对象，所以你可以继续访问对象属性。

在创建会话时，我们需要指定一个参数叫做expire_on_commit=False，这样会话提交后对象就不会过期。


这个参数是 session 里面的

```python
with Session(engine) as session:
        session.add(hero_1)
        session.add(hero_2)
        session.add(hero_3)
        session.commit()
        #- 提交后对象就会过期
        print(hero_1)
        print(hero_2)
        print(hero_3)

        #- 访问对象属性时，会话会自动重新加载对象
        print(hero_1.id)
        print(hero_2.id)
        print(hero_3.id)
```

## refresh 方法

```python
AsyncSessionFactory = async_sessionmaker(
    bind=engine,
    class_=AsyncSession,
    expire_on_commit=False, # 提交后不过期
)
```


refresh 方法用于刷新对象的属性，从数据库中重新加载对象。
```python
with Session(engine) as session:
        session.add(hero_1)
        session.add(hero_2)
        session.add(hero_3)
        session.commit()

        #- 刷新对象属性
        session.refresh(hero_1)
        session.refresh(hero_2)
        session.refresh(hero_3)

        print(hero_1)
        print(hero_2)
        print(hero_3)
```
