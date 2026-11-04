# MTV2

Next.js 前端 + Go API，使用 MongoDB、Redis、Elasticsearch。

## Docker Compose 部署

完整操作说明见 [deploy/README.md](deploy/README.md)。

```bash
cd deploy
./setup.sh
```

Setup 分别询问三个数据库使用新建 Docker 服务，还是连接已有的宿主机/远程服务；
支持混合配置。已有服务不会自动初始化或清空。

```bash
./settings.sh             # 交互修改配置，检查并更新应用
./deploy.sh v1.0.1        # 拉取已发布镜像并升级
python3 manage.py status
python3 manage.py rollback
```

部署目标为 Linux Docker Engine。首次使用需先通过 GitHub Actions 发布镜像，
或者使用离线镜像包，具体见部署说明。

## 目录

- `frontend/`：网站前端与生产镜像。
- `backend/`：API、初始化 CLI 与生产镜像。
- `deploy/`：Compose、首次安装、配置修改、升级回退和部署测试。
- `.github/workflows/`：CI、镜像发布和手动远端部署。
- `setup/`、`ESsync/`：原有初始化、运维和数据同步工具，默认不作为常驻服务部署。
- 爬虫不包含在网站部署镜像中。
