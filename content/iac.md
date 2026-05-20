---
Title: 基础设施学习
Date: 2023-11-15
---

### 一个简单的例子：

假设你需要一台有特定配置的云服务器。

*   **传统方式**：登录云服务商的网站，点点点，选择CPU、内存、硬盘大小……然后创建。
*   **IaC 方式**：你写一个这样的配置文件（这里是 Terraform 的简化例子）：

    ```
    resource "aws_instance" "my_web_server" {
      ami           = "ami-12345678"
      instance_type = "t2.micro"
      tags = {
        Name = "HelloWorldServer"
      }
    }
    ```

    然后运行一条命令 `terraform apply`，工具就会自动帮你在云上创建出这台服务器。

## 网站状态建设