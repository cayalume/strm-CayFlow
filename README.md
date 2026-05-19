# STRM 配置管理面板 (StrmFlow)

🚀 Emby/Jellyfin 媒体库自动化管理工具。本项目提供了一个优雅的 Web 界面，用于管理 STRM 文件生成、海报墙制作、刮削视频以及媒体库同步任务。

免费项目，源码未上传至github，请自行用docker命令向docker仓库拉取镜像。

---
本项目依赖以下项目： 
- [p115client](https://github.com/ChenyangGao/p115client/)
- [embyExternalUrl](https://github.com/bpking1/embyExternalUrl)
## 💬 交流群组

欢迎加入我们的 Telegram 群组进行交流讨论：

[![Telegram Group](https://img.shields.io/badge/Telegram-OpenStrm%20Group-blue?style=for-the-badge&logo=telegram)](https://t.me/strm_cayflow)
## 更新日志
**2026-05-06**  
* v1.0.1  
新增刮削功能：  
自动识别电影或是剧集  
支持本地刮削和网盘刮削  
支持自定义tmdb api和图像地址  
(可选)重命名文件夹为：片名 (年份) {tmdbid}  
自动整理分类，将在整理目录创建 电影/剧集 文件夹
## 🌟 核心功能
1.  **strm生成**
   *  基于p115项目，使用115目录树快速生成strm与复制元数据到本地。
   *  元数据可选复制/删除/上传
   *  支持定时执行
   *  支持url和load两种strm格式，不影响302重定向

2.  **媒体库海报生成**
   * 🖼️ **海报生成**：一键为 Emby/Jellyfin 媒体库生成精美的海报墙。
   *   🕒 **定时任务**：支持 Cron 表达式，配置媒体库的独立定时生成任务。
   *   📝 **日志监控**：实时查看海报生成日志，排查问题一目了然。

3.  **Emby & 302 代理**
 *   🔔 **通知集成**：支持 Telegram (TG) 机器人通知，支持自定义telegram api，即时推送媒体添加、播放状态。
 *   ⛓️ **302 重定向**：基于emby2alist，修改302配置页面内几个参数即可302播放媒体视频。

4.  **刮削视频**
 *  支持本地strm刮削和网盘视频刮削
 *  自动识别视频为电影或剧集并分类
 *  自动重命名文件夹为 片名 (年份) {tmdbid} 如 测试 (2001) {tmdbid=123456}
 *  支持设置定时执行

---

## 📸 界面预览

 ![界面预览](https://s1.imagehub.cc/images/2026/05/06/6595a058e365887f3b31a384282203e9.jpg) 
 ![同步目录](https://s1.imagehub.cc/images/2026/05/01/e9f3651c61c432d19edc939ac8a5abbf.jpg) 
 ![刮削目录](https://s1.imagehub.cc/images/2026/05/06/7a76ad699f381b85f52833b2080ab4a5.jpg)
 ![海报目录](https://s1.imagehub.cc/images/2026/05/01/2d47a486e972b2eb1a4d7e47d0a9cb87.jpg) 

*   **直观的卡片式设计**：每个媒体库和账号都以卡片形式展示，状态清晰可见。
*   **响应式布局**：无论是电脑大屏还是手机端，都能完美适配操作。

---

## 🚀 快速开始

### 前置准备
-   确保您已安装并运行了alist跟cd2，复制元数据与获取视频直链时需要。
-   准备好 115 网盘账号及 Emby/Jellyfin 服务器信息。

### 使用 Docker Run
```bash
docker run -d \
  --name strm-cayflow \
  --user root \
  --network host \
  -p 8092:8092 \
  -p 8091:8091 \
  -v /volume1:/volume1 \
  -v /opt/115strm/data:/app/data \
  -v /opt/115strm/config.json:/app/config.json \
  -e TZ=Asia/Shanghai \
  --restart=always \
  cayalume/strm-cayflow:latest
```

### 使用 Docker Compose

```
version: '3.8'  # 推荐使用较新的版本规范

services:
  strm-cayflow:
    image: cayalume/strm-cayflow:latest
    container_name: strm-cayflow
    user: root
    network_mode: "host"  # 关键：使用主机网络模式
    ports:
      - "8091:8091"
      - "8092:8092"
    
    volumes:
      - /volume1:/volume1
      - /opt/115strm/data:/app/data
      - /opt/115strm/config.json:/app/config.json
    environment:
            - TZ=Asia/Shanghai
    restart: always
```
## 📝 路径映射说明

| 本地路径 | 容器路径 | 说明 |
| :--- | :--- | :--- |
| /volume1 | /volume1 | 网盘挂载路径 |
| /opt/115strm/data | /app/data| 容器数据存放路径 |
| /opt/115strm/config.json | /app/config.json | 容器各项配置储存路径 |

*  运行容器前确保映射路径有config.json这个文件，如果没有务必创建一个同名空文件，会自动写入
---
###  首次配置
1.  **登录**：使用默认或配置的账号密码登录。
2.  **配置 Emby**：进入 `Emby管理` -> `Emby配置`，填入您的 Emby 服务器地址和 API Key。
3.  **添加账号**：进入 `STRM管理` -> `账号列表`，添加 115 Cookie。

---


## 💡 常见问题

**Q: 如何获取 115 Cookie？**
> A: 在浏览器登录 115 网盘，按 F12 打开控制台，在 Application (应用) 或 Storage (存储) 标签页中找到 Cookie，复制 `UID`, `CID`, `SEID` 等字段。

---

## 📄 许可证

本项目基于 [MIT License](./LICENSE) 许可证。
