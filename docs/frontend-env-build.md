# 前端环境配置构建

定位页使用 NEXT_PUBLIC_LOCATOR_KEY_1/2/3。它们按应用设计进入浏览器代码；
只在容器启动时设置 env，不能补回旧镜像中的客户端值。

本次以198服务器的frontend/.env为来源，未把值提交到Git。
构建使用BuildKit secret，避免把原始文件复制进镜像。
Next standalone会自动复制.env，Dockerfile在同一构建层显式删除该副本。

BuildKit不因secret内容变化自动使缓存失效，因此必须传入env摘要作为修订号：

    docker build \
      --build-arg FRONTEND_ENV_REVISION="$(sha256sum /secure/frontend.env | cut -d ' ' -f1)" \
      --secret id=frontend_env,src=/secure/frontend.env \
      -t your-registry/foodjudgeneko-web:version \
      frontend

Compose web应单独加载该配置：

    env_file:
      - ../frontend/.env

不要把数据库、JWT或AI服务凭据放入NEXT_PUBLIC变量。
node_modules、.next、.env及.env.*均排除出普通Docker构建上下文。
前端构建使用standalone输出，运行镜像采用非root的node用户。
源文件修改后应重新构建并验证路由、public变量及资源清单，而非修改运行容器的JS文件。

已部署修复版：frontend-env-20261028-r1。
定位配置解密离线验证通过，原12条路由保留，客户端三值存在。
运行镜像没有原始.env；节点2通过受保护的frontend/.env加载运行环境。
本次仅重建节点2web，API/网关/数据库未重启。
