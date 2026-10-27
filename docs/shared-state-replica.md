# 主节点与查询副本配置

同一 API 镜像可运行两个角色。主节点正常处理请求；
副本读取本地内容，登录与写入在服务端透明代理到主节点。浏览器URL不会变，
这不是 HTTP 301/302 跳转，也不需要在 OpenResty 增加逐接口重定向。

## 主节点

PRIMARY_API_URL、MTV2_STATE_MONGODB_URI 留空。
MongoDB、Redis、ES继续使用本机现有配置。
主节点 /api/auth/verify 使用现有JWT、版本和Redis黑名单验证，再返回最小身份信息。

## 副本

示例配置（地址仅为本部署的私网SSH转发端点）：

    PRIMARY_API_URL=http://172.30.90.1:18081
    MTV2_STATE_MONGODB_URI=mongodb://172.30.90.1:27018/?directConnection=true
    MTV2_STATE_MONGODB_DATABASE=mtv2
    MTV2_REDIS_HOST=172.30.90.1
    MTV2_REDIS_PORT=16379
    MTV2_REDIS_PASSWORD=
    MTV2_REDIS_DB=0
    MTV2_TRUSTED_PROXIES=127.0.0.1,::1,172.30.90.22
    MTV2_ALLOWED_ORIGINS=https://mt.nya.pink,http://mt.nya.pink,http://112.126.85.221:18000

本部署源 Redis 原本未配置密码；其他部署应填写实际服务密码。
本地内容仍由 MTV2_MONGODB_URI 和 MTV2_ES_HOSTS 指向副本的 mongo/es 服务。
容器监听用 MTV2_SERVER_HOST=0.0.0.0，是否对公网可达由 Docker ports 决定。

shared_state 支持 YAML mongodb_uri/mongodb_database；
server.trusted_proxies 也可通过 YAML 配置。

## 集合与请求

Accounts、Invitations、SearchQuotas、WeeklyScores、UserRankings 绑定 StateDB；
其余题目、评论、笔记等绑定本地 DB。未提供共享URI时 StateDB 等于本地DB。

副本的非GET/HEAD/OPTIONS API写请求回主节点。
GET默认本地，以下实时查询回主：

- /api/admin/、/api/points/、/api/weekly_scores/ 前缀
- /api/user_detail/account_points、/api/problem/quota
- 本部署额外配置 /api/user_detail/rankings

primary_read_prefixes 和 primary_read_paths 可配置；指定列表会替换对应默认列表。
/api/user_detail/ai_summary 保持本地，避免绕过节点2的AI停用选择。

副本通过私网主节点的 /api/auth/verify 验证 Bearer，不复制JWT签名密钥；
无效响应、超时、隧道不可用均拒绝访问，不回退到旧副本身份或无限额度。
保留原来的 token_version 单会话规则。

## 容器与更新

节点2采用独立 bridge 网络，API/web 使用 Docker DNS动态地址；
网关固定可信IP，使用resolver刷新上游。只发布网关 HTTP18000，
数据库与应用端口不单独发布。

backend/Dockerfile 从源码构建 API 与 mtv2ctl。
初始化命令委托固定 legacy 镜像中的初始化工具，以保留原ES映射；
该二进制依赖是显式构建依赖，尚未改写成当前仓库源码。
check-config 和 admin 使用新共享库逻辑。

两台服务器固定同一发布版本与摘要，角色差异只在配置。
此功能位于 feat/shared-state-replica，合入main后才进入后续主线发布；
不要把旧main镜像当作已经包含此功能。

Go测试和真实双节点登录、读写、额度、Redis、断链恢复已通过。
独立同步程序不包含在本仓库，见 E:\codes\mt_sync。
