# TX XHTTP / EdgeOne 诊断记录

> 本文保留早期轮次的历史观察。2026-10-08 已通过 HTTP/2 `END_STREAM` 单变量对照和恢复后的实时日志进一步定位；最新结论与证据见 [请求体结束标记与回源记录](tx-xhttp-end-stream-20261008.md)。

日期：2026-10-07（UTC）  
结论：**TX 的 `stream-one` XHTTP 公网链路未通过。已确认普通 HTTPS/TLS 和日志通道可用；目前只能把故障收敛到 XHTTP 隧道路径，不能据此断言是 EdgeOne 边缘拒绝还是回源失败。** 未修改服务器、EdgeOne 规则或订阅数据。

## 测试条件

- 客户端：现有 Android Studio `Pixel_10` 模拟器，已安装 RikoClash `2.11.35.Alpha.debug`；没有操作用户实体手机，也没有清除应用数据。
- 测试配置：临时单节点 Mihomo 配置，外层地址固定为 `43.174.246.63:443`；SNI、Host 为 `tx.vpn.riko.asia`，path 为 `/vless-xhttp`，ALPN 为 `h2`，XHTTP mode 固定 `stream-one`。每组只改变 `no-grpc-header`。
- 禁用了配置中的自动健康检查。Google 探针通过 ADB 端口转发至应用的 SOCKS 端口，并强制使用 `socks5h://127.0.0.1:17890`；不使用 `--noproxy '*'`。
- 本地 WSL 的无头 A/B 和 8 MiB PUT/GET 哈希测试已经记录在 [xhttp-no-grpc-header-wsl-20261007.md](xhttp-no-grpc-header-wsl-20261007.md)，本轮没有重复该测试。

## 结果

| `no-grpc-header` | Google `/generate_204` | 客户端观察 |
|---|---:|---|
| `false` | 0/3 | 三次均在 12 秒左右超时，HTTP 000、0 字节 |
| `true` | 0/3 | 一次 20 秒、一次 10 秒、一次 12 秒超时；HTTP 000、0 字节 |

正确的强制 SOCKS 测试均通过本地 SOCKS 握手，但未收到目标 TLS/HTTP 响应。Mihomo debug 日志中出现了 `www.google.com:443 match Match using PROXY[手机-tx-xhttp-h2]`，证明请求规则选择了 TX 测试节点；它不能单独证明 EdgeOne 或源站完成了建流。两个头部设置的结果相同，当前数据不支持仅靠切换 `no-grpc-header` 修复。

Android TUN 服务确实启动过：系统服务列表显示 RikoClash `TunService`，接口有 `tun0`，`dumpsys netstats` 将 `tun0` 记为默认 VPN 网络。Chrome 访问 Google 时，Mihomo 记录了 Google/tx 域名流量并选择 TX 节点，但等待期间页面没有显示已加载内容；因此 TUN 实际访问也**不计为通过**。

## 网络和日志通道对照

- VPN 关闭时，模拟器到两个 EdgeOne IP 的 TCP/443 探测均成功：`43.174.246.63` 约 0.43 秒，`43.174.247.63` 约 0.50 秒。
- 模拟器 Chrome 直连 `https://tx.vpn.riko.asia/` 完成 TLS，并显示 `404 Not Found`、`nginx/1.30.5`。404 是根路径无页面的既有表现，不代表 XHTTP 成功。
- WSL 将 TX 固定到真实 EdgeOne IP 的普通 HTTP/2 GET 返回预期 404，TLS 系统证书校验成功，ALPN 为 HTTP/2；EdgeOne 返回了 `eo-log-uuid`。同一唯一查询参数随后出现在源站 Nginx access log，证明普通 GET 的源站日志通道可用。
- 本轮 Nginx access/error 日志容器通道指向 stdout/stderr。普通 GET 可见；XHTTP 测试的对应时间窗及停止临时会话后的窗口均没有可关联的 Nginx 记录。Xray 容器当前配置只记录 `warning` 级别，测试窗口内 stdout 为空，不能用它证明请求未到达 Xray。
- EdgeOne 控制台显示实时日志任务数为 0；免费套餐的离线日志不可用。实时任务界面可选 CLS、S3 兼容存储或 HTTP POST，但没有已知的自有 HTTP 接收端；本轮未创建任务、未升级套餐、未创建付费 CLS，也没有改规则。因而没有 EdgeOne RequestID、边缘响应状态或回源状态可供区分。

## 结论边界与未完成项

已排除模拟器到 EdgeOne 的基础 TCP/HTTPS 完全不可达，也验证了普通 HTTPS 的源站日志通道。当前故障仍可能位于 Mihomo XHTTP 建流、EdgeOne 对该流的处理或 EdgeOne 到 Nginx/Xray 的回源段；现有日志不足以在这些位置中作唯一判定。Nginx location 和实时 Xray inbound 的配置本轮未能通过宝塔 MCP 稳定读取，不能将旧快照当作本轮确认值。

由于 Google 204 没有成功，本轮没有进行 8 MiB 上传、下载或 SHA-256 验收，也没有宣称上传/下载链路通过。没有执行服务器修复、容器重载、Xray debug 切换或 EdgeOne 参数变更。订阅管理器源码现有生成逻辑固定输出 `stream-one`（见 `subscription-manager-rs/src/subscriptions.rs`）；但模拟器原活动配置是本地 `File` 类型，恢复后仍为 `packet-up` 且没有 `no-grpc-header` 字段，无法通过刷新订阅更新它。本轮保留该原配置，避免用尚未通过公网验证的设置覆盖原数据。

本轮最初一批看似通过的六个 204 使用了 `curl --noproxy '*'`，该选项会绕过显式 SOCKS 代理并受电脑 Fake-IP/Clash 路径影响；这些结果已撤销，不计入 A/B。

## 清理与恢复

- 恢复 `XHTTP stream-one reimport` 为活动配置；VPN 为 Stopped。
- 删除本轮创建的两个 false 临时配置记录及一个 true 临时配置记录；原有其他配置未删除。
- 删除本轮两个设备 Download YAML 和本地临时 YAML；移除 `127.0.0.1:17890` ADB 转发。
- 停止临时 LogcatService 和 Chrome；未清除应用数据、未触碰实体手机。服务器容器、Xray/Nginx 配置和 EdgeOne 规则均未修改。
