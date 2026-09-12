镜像构建说明：
登录阿里云仓库
make login

多架构构建并推送
make push BUILDER=ruyi-builder VERSION=v0.0.1-202605271729

验证远端是否是多架构镜像
make imagetools-inspect VERSION=v0.1.0

本地调试说明：
把远程 Redis 映射到本机
在 VS Code：Terminal → Run Task... → 选择：port-forward redis (cotriage/redis svc 6379)

在 VS Code：Run and Debug 面板里选这个配置：log-collector (local debug, connect remote redis via port-forward)