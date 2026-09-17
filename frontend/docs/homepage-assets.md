# 首页资源与预加载

六张透明 PNG 位于 `public/landing/`：classic、reader、computer、yellow-dress、black-dress、summer。通过内置 imagegen 编辑用户提供的对应图片，未修改原始图片。

统一处理提示词：
> Use case: background-extraction. Edit the attached image into a clean transparent PNG cutout for a website floating decoration. Remove only the white background and floor/shadow (for computer scene remove the tabletop, keep computer keyboard mouse chair and character). Preserve the exact character, pose, clothing, props, colors and existing lettering. Whole subject within canvas, small transparent margin, actual alpha transparency, no checkerboard baked into pixels.

`npm run build` 在 Next 构建后自动生成 `public/asset-manifest.json` 和 `public/asset-worker.js`，部署需一起携带这两个文件。清单包括 public 资源及所有页面的构建代码、样式，排除旧首页截图和 sourcemap。`npm run dev` 只生成 public 清单，开发代码由 Next 按需编译。可通过 NEXT_DIST_DIR 设置独立构建目录。

首页以四路并发预加载，进度按已完成资源数计算；全部必需资源完成后进入首页。图片失败自动尝试共三次，仍失败显示重试；字体失败视为可跳过，进入首页五秒后再尝试一次。系统宋体/苹方/微软雅黑作为本地中文字体回退，不依赖 Google Fonts。

缓存按资源内容版本隔离。Service Worker 仅处理清单中的同源静态文件；不缓存 API、登录态、页面 HTML 或 RSC。带版本查询参数的品牌图片命中同一版本缓存。浏览器不支持或禁止 Cache Storage 时降级为普通加载。持久缓存依赖 HTTPS 或 localhost，存储可能被浏览器回收。

验证命令：`npm run build`，`node scripts/test-assets.mjs`。
