seata 这个框架为我们提供了三个核心的组件分别是:

1. Transaction Coordinator (TC) - 事务协调器
    这个事务协调器负责维护全局事务的运行状态, 比如prepare, commit和 rollback
2. Transaction Manager (TM) - 事务管理器
    它是发起方, 定义了全局事务的边界, 负责开启，提交或者回滚一个全局事务
3. Resource Manager (RM) - 资源管理器
   向 TC 注册分支事务（本地事务）的状态，报告本地事务的执行状态，并接收来自 TC 的指令来驱动本地事务的提交或回滚。


tcc 模式需要开发者手动实现对应 Try Confirm 和 Cancel 三个接口