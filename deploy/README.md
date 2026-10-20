# 部署与更新

CXPalm 网关运行在你自己的电脑上，任务始终由这台电脑的官方 Codex 桌面执行。
跨网络访问时，使用自己管理的 HTTPS 入口，将手机和网页的请求转发到本机网关。
本文中的域名、路径和配置均为示例；部署脚本不会替你安装系统服务或修改防火墙。

## 1. 准备电脑端

- 保持官方 Codex 桌面运行，安装或从源码构建 CXPalm 桌面端。
- 在 CXPalm 的连接设置中填写自己的 HTTPS 地址；在登录设置中创建账号密码。
- 网关只监听 `127.0.0.1`。账号、设备授权与运行数据保存在电脑端，不放在公开下载目录。

需要手动配置时，参考 [cxpalm.example.json](cxpalm.example.json)，将域名和绝对路径
替换为自己的值，保存为不提交到 Git 的私有配置。示例为 Windows 路径；其他系统
需使用对应的路径和官方桌面 IPC。构建方式见 [开发文档](../docs/DEVELOPMENT.md)。

## 2. 配置远程入口

推荐的 SSH 反向隧道拓扑：

```text
手机 / 网页
    │ HTTPS / WSS
自有反向代理
    │ 服务器回环端口 127.0.0.1:18791
SSH 反向隧道
    │ 电脑回环端口 127.0.0.1:18789
CXPalm 网关 → 官方 Codex 桌面
```

SSH 服务器地址、密钥与已验证主机记录由自己配置；不要关闭主机校验。
服务器隧道端口和电脑网关端口均只监听回环地址，不直接暴露公网。
不使用 SSH 时，可通过自己信任的私有网络/代理连接；网关仍须保持回环监听，
不要为了连接方便将其改为 `0.0.0.0`。

以 [Nginx 模板](nginx-cxpalm-win.conf) 为起点，在自己的 HTTPS 虚拟主机中配置路由：

| 路径 | 用途 |
| --- | --- |
| `/cxpalm/` | Web 入口，对应 `web/index.html` |
| `/cxpalm/assets/` | Web 模块、样式、图标与内置 Markdown 依赖 |
| `/api/v1/` | 转发到网关的鉴权 API / WebSocket |
| `/cxpalm/download.html` | 可选的下载页 |

按实际部署替换模板的目录、回环端口和 CSP 中的 `cxpalm.example.com`。
模板引用的 WebSocket 连接变量需在 Nginx `http` 上下文定义：

```nginx
map $http_upgrade $cmb_win_connection {
    default upgrade;
    '' '';
}
```

保留 Host / Origin、WebSocket 升级、鉴权及安全响应头。配置修改后先执行
`nginx -t`，通过后再重载。不要把数据库、上传目录、配置或日志配置为静态资源。

## 3. 按需开启 IP 白名单

白名单由 CXPalm 桌面端管理，支持 IP 和网段，仅限制 CXPalm 登录及会话 API，
不修改系统防火墙。移动网络出口 IP 可能变化，开启前确认允许的来源。

经过反向隧道时，网关无法直接看到手机出口 IP。可信代理必须覆盖客户端传来的
同名头，而不是追加或原样透传：

```nginx
proxy_set_header X-CXPalm-Client-IP $remote_addr;
```

确认实际链路中的代理能安全提供真实来源后，才在私有配置中设置
`trustedProxyIP: true`，并重新加载 CXPalm 网关。若前面还有 CDN 或其他代理，
须先正确限定可信来源，不能直接信任任意 `X-Forwarded-For`。

## 附件大小

Nginx 模板允许 **20 MiB 的完整请求体**。单个附件的应用上限同为 20 MiB，
但 multipart 表单头也占请求体大小，接近上限的文件需要留余量。
图片读取使用鉴权接口，不提供任意文件读取或公开上传文件下载地址。

## 更新与发布

- **网页**：发布 `web/` 中所需的静态资源，保持 HTML、脚本和样式版本配套。
- **Android / 桌面端**：客户端检查 HTTPS 更新清单，下载并校验后，由用户确认安装。
- **自建分发**：登录服务器与原生更新源是两套独立配置。当前原生代码绑定 `702165405/CXPalm`
  的清单与 Releases 白名单；仅更改登录地址不会更改更新源。独立分发前，统一调整更新器、
  发布脚本和对应测试，具体位置见 [更新源说明](../docs/PUBLICATION.md#deliberately-preserved-public-compatibility-identifiers)。

更新包应使用带版本/构建号的不可变文件名。先上传包并核对实际 HTTPS 下载的
大小、SHA-256 和包身份，再更新清单；不要覆盖历史同名包或关闭降级保护。
Android 覆盖升级需要相同包名和签名、递增的 versionCode。桌面清单使用
`releaseLine: initial-001`，自建版本需保持自己的发布策略一致。

`node scripts/prepare-desktop-release.cjs` 只在本地生成桌面发布包及校验清单，
不上传文件。`index.html` 是下载页模板，不代表安装包已经发布。
当前 Windows 打包配置未签名，Android 为开发签名；正式分发需自行管理签名与证书，
不要公开签名密钥。更新包不应携带个人配置、设备授权或聊天数据。

维护时保留自己的配置与授权存储，更新前做好私有备份；不要以清除应用数据作为
常规更新步骤。故障反馈与漏洞报告方式见 [贡献指南](../CONTRIBUTING.md) 和
[安全说明](../SECURITY.md)。

## 仓库更新源

- Android 清单：`https://raw.githubusercontent.com/702165405/CXPalm/main/updates/download.json`
- Windows 清单：`https://raw.githubusercontent.com/702165405/CXPalm/main/updates/desktop-download.json`
- 安装包：本仓库 Releases 中带版本和构建号的固定路径，例如 `releases/download/v0.0.9-build45/`。

GitHub 下载会跳转到自己的资源 CDN，客户端只接受限定仓库 ID 的 CDN 路径，最多
三次跳转；清单本身不跟随跳转，也不会向更新服务发送 CXPalm 登录凭据。

旧客户端不会凭空改变内置更新源。迁移版本通过旧源提供最后一次兼容更新，安装后
后续检查转到本仓库；也可从 Releases 手动安装。不要把旧清单直接改成 GitHub URL，
旧客户端会按原有白名单拒绝它。仓库更新是否可达取决于设备实际网络。
