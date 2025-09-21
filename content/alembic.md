---
Title: Alembic 数据库迁移工具
Date: 2023-11-15
---

Alembic 是一个 Python 环境中的数据库迁移工具，类似于 Go 语言中 GORM 的数据库迁移功能。它是 SQLAlchemy 的迁移工具扩展，用于管理数据库架构变更的版本控制。

在 Alembic 中最简单的命令就是初始化项目：
```bash
alembic init migrations
```
执行完成这个命令之后会在当前目录下生成：

- 一个 `alembic.ini` 配置文件，包含 Alembic 命令的全局配置
- 一个 `migrations` 文件夹，包含：
  - `versions` 子文件夹，用于存储生成的迁移脚本
  - `env.py` 文件，包含迁移运行时的环境配置

## 环境配置文件 (env.py)

`env.py` 文件是 Alembic 迁移运行的核心配置文件，主要包含以下关键部分：

```python
# 导入必要的模块
from logging.config import fileConfig
from sqlalchemy import engine_from_config, pool
from alembic import context

# 获取配置对象
config = context.config

# 加载日志配置
if config.config_file_name is not None:
    fileConfig(config.config_file_name)
```

## 设置数据库连接

在 `alembic.ini` 文件中配置数据库连接字符串：

```ini
sqlalchemy.url = mysql+pymysql://username:password@localhost:3306/database_name?charset=utf8mb4
```

## 基本迁移操作

### 创建迁移脚本

使用 `revision` 命令创建新的迁移脚本：

```bash
alembic revision -m "create tasks table"
```

执行后会在 `versions` 目录下生成一个新的迁移文件，文件内容包含两个主要函数：

```python
"""create tasks table"""

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision = '5776bfcc91f0'
down_revision = None
branch_labels = None
depends_on = None

def upgrade() -> None:
    """Upgrade schema."""
    # 在这里添加数据库升级操作
    pass

def downgrade() -> None:
    """Downgrade schema."""
    # 在这里添加数据库降级操作
    pass
```

### 添加表结构定义

在生成的迁移文件中，我们可以使用 `op` 和 `sa` 对象添加数据库操作，例如创建表：

```python
def upgrade() -> None:
    """Upgrade schema."""
    op.create_table(
        'tasks',
        sa.Column('id', sa.Integer, primary_key=True, autoincrement=True),
        sa.Column('title', sa.String(100), nullable=False),
        sa.Column('description', sa.Text, nullable=True),
        sa.Column('created_at', sa.DateTime, default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime, default=sa.func.now(), onupdate=sa.func.now())
    )

def downgrade() -> None:
    """Downgrade schema."""
    op.drop_table('tasks')
```

### 应用迁移

使用 `upgrade` 命令应用迁移：

```bash
# 应用所有未应用的迁移
alembic upgrade head

# 应用指定版本的迁移
alembic upgrade 5776bfcc91f0

# 回滚一个版本
alembic downgrade -1

# 回滚到初始状态
alembic downgrade base
```

## 修改现有表结构

当需要添加新字段时，创建新的迁移脚本：

```bash
alembic revision -m "add status column to tasks"
```

然后在生成的迁移文件中添加字段操作：

```python
def upgrade() -> None:
    """Upgrade schema."""
    op.add_column(
        'tasks',
        sa.Column('status', sa.String(20), default='pending', nullable=False)
    )

def downgrade() -> None:
    """Downgrade schema."""
    op.drop_column('tasks', 'status')
```

## 查看迁移历史

使用 `history` 命令查看迁移历史记录：

```bash
alembic history
```

输出示例：
```
5776bfcc91f0 -> be3e66122ef1 (head), add status column to tasks
<base> -> 5776bfcc91f0, create tasks table
```

## 自动迁移

Alembic 支持根据模型定义自动生成迁移脚本，需要以下配置：

在 `env.py` 文件中，导入你的 SQLAlchemy 模型并配置 `target_metadata`：

```python
# 在 env.py 中添加
from myapp.models import Base  # 导入你的模型基类

# 配置目标元数据
target_metadata = Base.metadata

# 修改 run_migrations_online 函数
def run_migrations_online():
    # ... 现有代码 ...
    context.configure(
        connection=connection,
        target_metadata=target_metadata,
        # 其他配置...
    )
    # ... 现有代码 ...
```

使用 `revision --autogenerate` 命令自动生成迁移脚本：

```bash
alembic revision --autogenerate -m "add user table"
```

Alembic 会自动检测模型变更并生成相应的迁移代码。这对于大型项目特别有用，可以减少手动编写迁移脚本的工作量。
