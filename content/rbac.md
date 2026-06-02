---
Title: RBAC 权限控制系统设计 
Date: 2025-10-02
---

role -> permission -> resource

经过这个链路去控制用户对应资源的访问权限，实现 RBAC 权限控制。

所以至少需要 5 张表:
- role 表
- role 对应着 permission 之间的关系
- permission 表
- resource 和 permission 之间的关系表
- resource 表