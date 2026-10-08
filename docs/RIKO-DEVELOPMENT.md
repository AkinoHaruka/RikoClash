# RikoClash 开发起点

日期：2026-10-05。当前状态：完整源码和核心子模块已准备；尚未构建、安装或完成 Riko 品牌与线路改造。

逐项任务、依赖、源码范围、回退和验收条件见 [RikoClash 开发任务](../plans/RIKO-CLASH-PLAN.md)。以本工程 AGENTS.md 和该计划为当前路线依据，历史原型文档中的“WARP 必经”不应用于本工程。

## 来源

| 项目 | 来源及版本 |
| --- | --- |
| 应用上游 | https://github.com/MetaCubeX/ClashMetaForAndroid |
| 本地来源 | `C:\TRAE\VPN\ClashMetaForAndroid`，分支 `codex/vps-direct-route` |
| 应用起点 | `c320719b1cfb5195afa819244bd359c202e36b35` |
| Mihomo 上游 | https://github.com/MetaCubeX/mihomo |
| 核心起点 | `88dcbf7f1614a67c3b36b848ee3592dfa92ada36` |
| 核心目录 | `core/src/foss/golang/clash` |
| 当前开发目录 | `C:\TRAE\VPN\RikoClash` |
| 当前开发分支 | `codex/riko-clash` |

应用从本地已更新的 CMFA 分支克隆，保留该分支已有提交，不宣称是未修改的官方 main。主仓库远程名为 `upstream`；尚未创建 Riko 远程仓库或推送。核心具有独立 Git 对象副本，已转换为标准子模块并固定在上述提交；无需依赖原目录中的构建产物。

上游 README 仍保留 CMFA 的说明与自动化命令。未修改品牌和包名前，不能照抄其安装或服务启动命令来操作用户设备。根工程及核心的 LICENSE 原样保留。

## 产品和线路

基于 CMFA 完整客户端改造，复用订阅、节点、规则、DNS、统计、服务管理和网络观察机制。旧独立 Compose 原型仅提供已验证的 WARP 控制逻辑和诊断经验。

主线路为设备直接连接现有自建代理服务器，再访问目标站；备用线路在该服务器前加入内置 WARP。保留 WS / gRPC；依用户 2026-10-06 指示，Riko Mihomo 出站及自有订阅生成器把所有 XHTTP 节点固定为 `stream-one`。WARP 备用线路应显示启用原因及实际连接状态。

WSL Compose 已用 Xray 26.3.27、Mihomo 和 Nginx 验证 `stream-one`：客户端直连 Xray 可访问 Google；通过 Nginx 时 `proxy_pass` 复现 HTTP 400/关闭请求体，切换 `grpc_pass` 后 Google 204 通过，8 MiB PUT/GET 的 SHA-256 一致。此结果仅证明本地 WSL 链路和代理方式；EdgeOne/Cloudflare 公网入口、真实手机与云端配置仍须分别验收。详见 [XHTTP stream-one WSL 记录](evidence/xhttp-stream-one-wsl-20261006.md)。

服务器原生出口测试已成功完成三轮下载和上传，记录位于 `../Riko-VPN-APP/docs/evidence/server-native-speed-20261005.json`。这只证明服务器到该测速端点的出口表现；不能替代客户端到服务器、gRPC 数据面或手机测速。

## 实施顺序

1. **构建基线**：核对现有 Gradle / Android / Go / CMake / NDK 依赖，固定构建所用规则资产，完成上游衍生 APK 构建。首个 Debug 构建已成功；重复构建字节一致性尚未验证。
2. **应用隔离与品牌**：设定 Riko 名称、独立 applicationId、对应 Provider / 权限和签名；安装前确认不会覆盖用户现有 Clash 或其他 Riko 应用。
3. **主线路验收**：在同一手机、同一网络和同一目标下验证现有服务器节点，比较 WS / gRPC / XHTTP 上传下载、TCP/UDP、DNS、重连与网络变化。
4. **WARP 备用接入**：评估并迁入旧原型私有身份管理和 MASQUE 链式拨号，在本工程重新验证，沿用 CMFA 生命周期及配置入口，明确主备线路策略和状态。分别测试手动切换、失效、恢复与 UDP 路径，之后再决定自动切换条件。
5. **产品验收**：检查订阅更新、节点选择、规则、统计、前后台运行和断线恢复，记录每一阶段的代码版本、设备版本、命令、数据面结果及限制。

## 本次完成范围

独立 RikoClash Debug 包已构建并安装，沿用 CMFA 页面、订阅和节点管理。已修复 Socket protect 失败传播、默认固定服务器路由排除、备份策略和独立应用入口，并锁定本轮构建工具与规则资产。经用户授权启动本应用 VPN，gRPC 主线路出口已确认，用户确认可访问 Google。本轮没有变更云端服务，也没有启动用户原版 Clash。

当前结果与边界见 [主线路记录](evidence/mainline-20261005.md)和 [WARP 手动备用](evidence/warp-manual-20261005.md)，构建方法见 [BUILD.md](BUILD.md)。用户已在新应用导入正式订阅；使用该订阅节点与已有私有身份完成 WARP 手动链路。每设备 WARP 身份管理、订阅更新联动、自动备用切换和产品化界面尚未完成。
