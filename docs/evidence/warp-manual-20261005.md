# WARP 手动链路验证：2026-10-05

使用本工程已安装的 `asia.riko.clash.alpha.debug`，Android 16/API 36 arm64 手机。没有安装官方 WARP 客户端，没有启动原版 Clash，没有修改云端服务。

链路：手机应用流量 → RikoClash 的单个 VpnService/TUN → 内置 Mihomo MASQUE/H3 WARP → 自建服务器的 `dir.vpn.riko.asia` TLS / VLESS gRPC 入口 → 目标网站。服务器节点显式设置 `dialer-proxy: warp-egress`。WARP 本身不作为目标站出口。

先使用旧原型的已有私有身份验证 WARP 传输；随后读取用户在本应用导入的正式订阅“新配置”，保留原订阅不变，取其中 dir 域名的 gRPC 节点参数与已有 WARP 身份生成独立测试配置。身份、UUID 和完整订阅链接未写入源码、本文或 APK。

| 验证 | 实际观察 |
| --- | --- |
| WARP 节点选择 | 代理页选中 `Riko WARP grpc`，手动选择组内无自动回退 |
| Google 探测 | `https://www.google.com/generate_204` HTTP 204 |
| 正式订阅参数验证 | Google 首页 HTTP 200，84,446 字节，6.430513 秒 |
| 出口 | Cloudflare trace 请求成功，私下比对出口与自建服务器一致 |
| 故障 | 独立测试配置将 WARP 端口改为不可达端口 9；选中 WARP 的请求失败、HTTP 000，没有静默切主线路 |
| 手动恢复 | 同一故障配置手动选主线路后 Google HTTP 204，1.578335 秒 |
| 最终本地配置 | 成功配置通过 CMFA 的“复制”另存为 File 类型 `RikoWARP`；重新加载后 Google HTTP 204 |

测试使用服务器实际 IP 拨号，TLS 主机名仍为 `dir.vpn.riko.asia`；CF 主线路节点使用本次入口解析 IP 和 `vpn.riko.asia` TLS 主机名。这是开发验证快照，不是正式域名解析策略。TLS 校验未关闭。Google 是目标站；trace 只用于出口确认，不等同于独立抓包证明每一跳。

用户在本应用导入的“新配置”来源主机为 `vpn.riko.asia`，当前包含 15 个服务器节点。这份原订阅仍保留；它本身没有 WARP 节点。`RikoWARP` 是手动构造并保存的本地配置，订阅更新不会自动更新它。

已经实现并验证的是内置核心的 WARP 手动拨号能力，使用已有身份。每设备自动注册、身份重置/失效管理、订阅更新后自动派生备用节点、自动故障切换与产品界面尚未实现，不能据此将 RC-05/06 全部标为完成。未进行新的速度测试、UDP/IPv6 或长期稳定性验收。

删除本轮临时主线/WARP URL 配置、故障配置及未保存的 WARP 导入尝试，停止本机临时配置 HTTP 服务并移除本次 adb reverse。保留正式订阅和成功的本地 File 配置；最终 RikoClash 运行在 `RikoWARP` 的 WARP 节点上。
