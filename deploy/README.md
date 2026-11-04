# MTV2 Compose 第一版

部署范围：Next.js、Go API、Nginx，以及按需创建的 MongoDB、Redis、Elasticsearch。
不包含爬虫、持续抓取或 ES 数据同步任务。

## 新服务器首次部署

要求 Linux Docker Engine、Docker Compose v2.24+、Python 3.10+。运行账号需要 Docker 权限。
应用使用 Linux host networking，内部监听 `127.0.0.1`，只有网关入口可对外开放；
Docker Desktop、Windows 和 macOS 不作为生产部署目标。

完整数据库部署建议至少 4 GB 内存、2 CPU，并按实际数据量预留磁盘。
本机 ES 启动前检查 `sysctl vm.max_map_count` 至少为 `262144`，不足时由管理员设置。

将发布包解压到 `/root/mtv2/deploy`，或者在源码目录执行：

```bash
cd /root/mtv2/deploy
chmod +x setup.sh deploy.sh settings.sh
# 私有镜像仓库需要先 docker login ghcr.io，使用只有 read:packages 的凭据。
./setup.sh
```

Setup 会**分别**询问 MongoDB、Redis、Elasticsearch：

1. 新建 Docker 服务：生成独立密码，创建本项目的数据卷，仅向宿主机回环地址发布数据库端口。
2. 连接已有宿主机或远程服务：询问地址、认证信息，只检查连接，不自动初始化、迁移或修改原数据。

支持任意混合，例如 Docker MongoDB + 宿主机 Redis + 远程 ES。
已有服务地址应为宿主机可访问的地址。远程 ES 建议 HTTPS，私有 CA 放在 `certs/` 下，
通过 `MTV2_ES_CA_FILE=/etc/mtv2/certs/文件名` 指定；默认验证证书，不自动关闭校验。
Redis 第一版配置支持普通 TCP；跨服务器连接应通过 SSH 隧道、VPN 或可信私网，不直接公开端口。

首次部署需要能拉取 `API_IMAGE` / `WEB_IMAGE`。默认使用本仓库发布的 GHCR latest；
也可以在初始化时指定已发布版本：

```bash
./setup.sh --api-image ghcr.io/neiro-o/mtv2-api:v1.0.1 \
           --web-image ghcr.io/neiro-o/mtv2-web:v1.0.1
```

尚未发布到 GHCR 时，可使用本次验证的离线镜像包（Linux AMD64）：

```bash
docker load -i mtv2-compose-v1-images.tar.gz
./setup.sh --local --api-image mtv2-api:compose-v1 --web-image mtv2-web:compose-v1
```

此方式仍会按所选数据库拉取 MongoDB / Redis / ES 官方镜像。
如果 setup 已保存配置、但后续拉取镜像失败，修复网络或仓库登录后直接执行
`./deploy.sh`（离线程序镜像使用 `./deploy.sh --local`），不要重新初始化。

部署文件夹会设为 `0700`；`.env`、配置历史为 `0600`。YAML 为 `0644`，
使 API 容器的非 root 用户可读取，宿主机其他用户受文件夹权限限制。
不要把运行时 `.env`、`config.yml`、配置历史或备份提交到 Git，也不要运行 `docker compose config` 后公开其输出。

## 复用已有配置、保留数据库

```bash
# 交互式选择每一种数据库，已有服务的连接信息从旧 YAML 继承。
./setup.sh --existing-config /root/mtv2/config.yml

# 主服务器试运行：全部使用原来的服务，只准备配置，不启动容器。
./setup.sh --existing-config /root/mtv2/config.yml --reuse-all --prepare-only \
           --api-image mtv2-api:compose-v1 --web-image mtv2-web:compose-v1
# ES 使用私有 CA 时，先复制 CA 到 certs/，在 .env 中指定 MTV2_ES_CA_FILE。
./deploy.sh --local
```

`--reuse-all` 默认入口为 `127.0.0.1:18000`，API 为 `127.0.0.1:18080`，前端为
`127.0.0.1:13000`，可与原有 8080 / 3000 并行验证。
Compose 不会停止或接管原来的 MongoDB/ES 容器或 Redis 系统服务。
已有 ES 如果未挂数据卷，务必保留原容器；本工具不把其容器层迁移到新卷。

可先从同一台服务器访问入口验证，再由现有 HTTPS Nginx 将网站流量转发到新入口。
更改反向代理前备份配置，执行 `nginx -t`，成功后 reload。不要改动其他站点或证书。
网关仅信任回环地址上的上游代理，以保留真实客户端 IP 和登录令牌的 IP 绑定。

## 后续一键修改设置

```bash
./settings.sh
./settings.sh GATEWAY_PORT 18000
./settings.sh MTV2_ALLOWED_ORIGINS https://example.com
./settings.sh MTV2_DEEPSEEK_KEY   # 隐藏输入，不把密钥放在命令行参数里
./settings.sh MTV2_ES_CA_FILE /etc/mtv2/certs/es-ca.crt
```

流程是保存私有配置副本 → 校验 → 更新应用服务 → 等待健康检查。
失败时恢复配置并尝试恢复原应用服务。此操作不会重建数据库容器。
修改 JWT 会使旧令牌失效；修改数据库连接会切换应用读取的数据源，但不搬迁数据。
数据库版本、密码、端口、卷名称和 Docker/已有服务模式不能通过 settings 随意切换，需单独规划迁移。
高级集合名等设置可编辑 `config.yml`，再运行 `./deploy.sh --local`；修改前保留自己的文件副本。

## 发布、升级、回退

```bash
./deploy.sh v1.0.1           # 拉取指定版本，再更新
./deploy.sh --local         # 使用本机已有镜像
./deploy.sh --build         # 源码 checkout 下从本机源码构建；发布包不包含源码
python3 manage.py status
python3 manage.py rollback  # 上一个成功版本，仅更新应用，不回退或重建数据库
python3 manage.py admin     # 显式创建管理员，不覆盖已有账号
```

只有选为 Docker 的 MongoDB / ES 才会在首次部署执行一次显式初始化。
初始化只建索引/必要结构，已有 ES 索引保留，不 drop、不清空、不重建数据。
成功标记为 `.initialized-mongo` / `.initialized-es`，普通更新跳过。
不要删除这些标记来强行升级结构；数据库迁移应该单独审核、备份和执行。
Compose 不承诺零停机更新，镜像回退也不能撤销数据库结构或业务数据变更。
不要对生产项目执行 `down --volumes`、`volume rm`、删除 ES 索引或清库脚本。

数据卷位于 Docker 的卷目录，独立于镜像和部署文件；单机卷不是备份。
MongoDB 使用有认证的 `mongodump --archive --gzip` 备份；ES 优先使用官方 snapshot/repository。
主服务器本次采用只读业务索引导出，保留 mapping/settings、文档 ID 和来源文档；
在线导出不保证 MongoDB 与 ES 的跨库同一时刻一致性，也不包含 ES 内部安全索引。
恢复必须先在隔离环境验证，不能直接对现有数据库执行覆盖恢复。

## CI/CD

- `ci.yml`：Go 测试和 vet、真实 Redis 集成测试、前端构建和资源测试、部署安全测试、8 种数据库组合校验、独立全栈 smoke。
- `release.yml`：版本标签 `v*` 或手动触发，先通过 CI，再构建 AMD64 / ARM64 的 api/web 镜像并推送 GHCR；打包不含密钥的部署文件。
- `deploy.yml`：手动选择已发布版本，通过 SSH 调用服务器部署脚本。生产环境可在 GitHub Environments 中配置审批规则。

在 GitHub 配置 `production` 环境，以及 `DEPLOY_HOST`、`DEPLOY_USER`、
`DEPLOY_SSH_KEY`、`DEPLOY_KNOWN_HOSTS` secrets。部署工作流目前使用固定路径
`/root/mtv2/deploy/manage.py`。应使用专用部署 SSH key；不要提交个人密钥或 passphrase。
`DEPLOY_KNOWN_HOSTS` 需要事先核实服务器指纹，工作流不会跳过主机身份检查。
服务器需事先登录私有镜像仓库。部署包或脚本有更新时先安装对应版本的部署文件，
保留运行时 `.env`、`config.yml`、`certs/` 和版本历史。

尚未推送工作流/发布镜像前，CI/CD 不会自动开始运行；可先通过本地构建镜像进行验证。

## 验证

```bash
python3 -m unittest discover -s deploy/tests -v
python3 deploy/tests/validate_compose.py
# 仅在测试 Linux 主机运行；创建随机项目、端口、独立数据卷，退出时只删除测试项目。
python3 deploy/tests/smoke.py --build
```

## 后续主从数据同步边界

这一版不实现数据同步、不在新服务器上运行爬虫。后续建议从主服务器向只读副本同步，
使用 SSH 隧道、WireGuard 或双向 TLS 保护传输，校验主机身份和数据完整性；
账号、题目、搜索索引分别定义增量游标、删除标记与恢复策略。
账号、积分、配额等可变数据需要先确定唯一写入端与冲突规则，不能直接双向覆盖。
数据同步属于独立能力，不等同于备份或高可用。
