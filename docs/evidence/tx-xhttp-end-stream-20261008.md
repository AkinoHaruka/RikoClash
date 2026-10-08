# TX XHTTP：请求体结束标记与回源的关联证据

日期：2026-10-08，Asia/Singapore。下面的原始时间戳使用 UTC（2026-10-07）。

## 当前结论

**当前 `tx.vpn.riko.asia` 七层入口在请求体保持打开时不进行可观测的回源；发送 HTTP/2 `END_STREAM` 后，同样的请求内容立即回源并返回目标响应。这个行为与 XHTTP `stream-one` 的持续双向流冲突。TX VPN 链路仍未通过验收。**

无 gRPC 头的三个串行对照均为：开放请求体超时；结束请求体得到外层 HTTP 200 和内层 Google HTTP 204。带 `application/grpc` 头的另外三个对照结果相同。无头分支第二、第三组的实时边缘/源站记录闭环：只有结束体请求具有源站状态 200、Nginx POST 200 和 Xray 接收记录。

这个结论限定于本轮冻结的 EdgeOne 入口、规则和请求形态。不把它扩大为所有 EdgeOne 套餐都不支持，也没有证据证明升级套餐能解决。没有修改已选定的 `stream-one`、VPN 转发规则或订阅输出。

## 为什么不能用“提前关闭上传”修复

`stream-one` 把 TCP 连接的后续上行数据继续写入同一个 HTTP 请求体，并从该请求的响应读取下行数据。它需要在上传请求体尚未结束时就获得响应。

诊断中发送 `END_STREAM` 只是让一次有限请求完成。把这个操作放进正常 VPN 会使后续 TLS 握手消息、HTTP 请求、上传等无法继续发送；因此有限请求通过不计为正常 `stream-one` 或 Android TUN 验收通过。

## 冻结条件与配置证据

- 入口固定为 EdgeOne IPv4 `43.174.246.63:443`；SNI、Host 为 `tx.vpn.riko.asia`，路径为 `/vless-xhttp/`，ALPN 为 `h2`。
- Android 当前处理配置：一个 VLESS XHTTP 节点，一个 `select` 组，一条 `MATCH` 规则；显式 `stream-one`，`no-grpc-header: true`，`log-level: debug`；没有自动测速组。
- 云端 Xray 的两个 XHTTP 入站仍为 `stream-one`：10082 `/vless-xhttp`，10083 `/vless-aws-xhttp`；日志级别仍为 `warning`。
- Nginx 的 `/vless-xhttp` 使用 `grpc_pass grpc://xray:10082`。
- EdgeOne 国际站站点为免费版。全局 HTTP/2 和 gRPC 已开启；全局 HTTP/2 回源关闭，但已启用的 `Riko tx WS + HTTP2 origin` 规则对 `HOST=tx.vpn.riko.asia` 开启 HTTP/2 访问、HTTP/2 回源和 WebSocket。普通 GET 和有限 POST 在源站均记录为 HTTP/2。
- 没有在 A/B 期间切换 gRPC、HTTP/2 回源、转发方式、SNI、Host、路径或服务器模式。

官方 gRPC 文档与控制台说明均仅列出 Simple RPC、Server-side streaming RPC 两种支持模式；开启 gRPC 不能据此推导出客户端流式上传或双向流能力。[EdgeOne gRPC 文档](https://edgeone.ai/document/zh/53377)

## 观测通道恢复

本轮发现临时 POST 接收器的 `MAX_FILES=200` 已达到上限：日志目录有 200 个正文文件和 200 个元信息文件，接收器持续返回 HTTP 507。EdgeOne 控制台仍显示“推送中”，因此控制台状态不能单独证明投递成功。

400 个既有文件、合计 129,877 字节，已移到服务器私有诊断归档，逐文件 SHA-256 校验全部通过；没有截断或删除历史日志。归档后投递恢复。

新普通 GET 使用唯一查询标记，正常证书校验通过、返回预期 404；源站出现对应查询参数，EdgeOne 记录为 `2026-10-07T22:42:17Z`、边缘 404、源站 404、`no_exception`。这个自检仅证明普通请求与观测通道，未作为 XHTTP 双向流成功证据。

## Android 诊断构建

原生 debug 日志只有规则命中与取消结果，不能给出请求体写入阶段。因此制作了独立诊断产物，增加以下事件，全部只记录阶段、状态和长度：

- `RoundTrip` 开始和结束；
- 传输连接取得；
- 隧道首次写入开始/完成；
- HTTP 传输首次读取请求体和累计读取字节数；
- 响应头状态。

不记录 UUID、请求体内容或认证头。基础核心提交为 `88dcbf7f1614a67c3b36b848ee3592dfa92ada36`，保留此前的本地核心改动；诊断构建记录了八个修改/测试文件的摘要，保留固定 JDK/Go/Android 工具链校验。标准构建脚本没有修改。

验证结果：

- WSL Docker / Go 1.26.3：`adapter/outbound`、`tunnel`、`transport/xhttp` 测试通过。
- WSL Compose：新诊断核心经 Nginx gRPC ingress → Xray → WebDAV 完成 8,388,608 字节 PUT/GET；两端 SHA-256 均为 `75bc9160b202153c278b7dbe2c515643a62bee497b1715db368fee76392a2e96`。
- 本次 WSL Google HTTPS 探针发生目标 TLS EOF，记为失败；本地受控文件传输证明包装器未改变请求字节，但不是公网 Google 验收。
- Android Alpha Debug 构建、核心单测和 Lint 通过；x86_64 APK SHA-256 为 `30392b6a8147c972d24bdc51f55d32076d910465c5fb9154ad4b80c2d67a6919`。安装前后证书验证通过且相同。
- 模拟器安装因空间不足失败一次；Android 清理可回收缓存后原地更新成功。未卸载、未清应用数据。安装前后导入配置、数据库、偏好等 20 个文件摘要均一致。

应用 30 秒 Google HTTPS 探针（经显式 SOCKS、无 `--noproxy '*'`）的脱敏片段：

```text
22:28:30.773Z RoundTrip started method=POST
22:28:31.875Z transport connection ready elapsed=1.1024889s
22:28:31.876Z tunnel write started bytes=498
22:28:31.876Z request body first data read bytes=498
22:28:31.876Z tunnel write completed bytes=498 error=false
22:29:00.770Z RoundTrip failed body_read_bytes=498 elapsed=29.9977863s
curl: HTTP 000; Google TLS handshake not completed
```

这证明 Mihomo 产生了上行内容，HTTP 传输消费了内容；它本身还不能证明这些字节已发成网络上的 DATA 帧。后面的独立 HTTP/2 对照专门补上这一证据。

此前还执行过一次 90 秒诊断请求：curl 在约 90.005 秒以 HTTP 000 结束；EdgeOne 对应 `15:54:18Z` 记录耗时 89,615 ms，边缘/响应正文为 0、回源字段为 -1。这轮发生在接收器达到文件上限之前。该结果说明仅增加短请求超时不足以修复，但单独不用于解释请求体发送方式。

早期启动瞬间出现过多个节点健康检查，相关 Nginx/Xray Google 记录不与单次探针混用。当前单节点运行配置重新读取核验后，才进行独立探针。

## HTTP/2 `END_STREAM` 单变量对照

独立探针使用物理 USB 网络适配器的源地址，并设置 Windows `IP_UNICAST_IF` 为该适配器索引；固定真实 EdgeOne IP，正常校验证书，协商 TLS 1.3 / h2。没有修改或启动用户 Clash，也不通过 Fake-IP 解析外层地址。

同一组内使用相同的 XHTTP 请求头（由当前 Mihomo 的头部生成逻辑产生并复制）、相同 UUID、相同 109 字节 VLESS + 目标请求内容。UUID 从模拟器私有配置读入并通过 stdin 传给探针，不写到源码或结果日志。

两种状态唯一差异：

1. `stream_open`：发出 HEADERS 和 109 字节 DATA，保持请求体打开。
2. `end_stream`：发出相同 HEADERS、相同 DATA，再发出 0 字节、带 `END_STREAM` 的 DATA。

探针在 TLS 之后只观察 HTTP/2 帧头与长度，跳过 HPACK 和数据载荷。它确认 DATA 已实际写入 TLS 连接，也看到对端 SETTINGS / WINDOW_UPDATE。它不是截取或转储 VLESS 载荷。

为避免把目标 TLS 的多轮握手混入“上传是否结束”的对照，内层目标特意使用 `www.google.com:80` 的 `HEAD /generate_204`；外层 TX TLS 正常校验。**这些 HTTP 204 是诊断结果，不是计划要求的 Google HTTPS / 正常 VPN 验收。**

### 三组客户端结果

| 组 | 开放请求体 | 结束请求体 |
|---|---|---|
| 1 | DATA 109 已发送，15 秒无响应头 | 外层 200，内层 `HTTP/1.1 204 No Content` |
| 2 | DATA 109 已发送，15 秒无响应头 | 外层 200，内层 `HTTP/1.1 204 No Content` |
| 3 | DATA 109 已发送，15 秒无响应头 | 外层 200，内层 `HTTP/1.1 204 No Content` |

第一组在接收器恢复前进行，只使用客户端和源站证据；不虚构该组的实时边缘日志。第二、第三组在普通 GET 投递自检通过后进行。

### 带 gRPC 头分支的范围核验

在已开启 gRPC 的同一 EdgeOne 配置下，把探针的 `no-grpc-header` 改为 false，使头部生成器添加 `Content-Type: application/grpc`。其余入口、路径、物理网卡、VLESS 内容和目标保持相同；每组仍只比较是否发送 `END_STREAM`。

| 分支 | 开放体 | 结束体 |
|---|---:|---:|
| 无 gRPC 头 | 0/3，均 15 秒无响应头 | 3/3，外层 200、目标 HTTP 204 |
| 有 gRPC 头 | 0/3，均 15 秒无响应头 | 3/3，外层 200、目标 HTTP 204 |

带头分支的三个结束体请求分别对应：

```text
Nginx 23:07:14Z POST /vless-xhttp/ HTTP/2.0 200 148
Xray  23:07:14.142098Z accepted tcp:www.google.com:80
Nginx 23:07:30Z POST /vless-xhttp/ HTTP/2.0 200 148
Xray  23:07:30.199141Z accepted tcp:www.google.com:80
Nginx 23:07:46Z POST /vless-xhttp/ HTTP/2.0 200 148
Xray  23:07:46.392347Z accepted tcp:www.google.com:80
```

开放体仍没有对应源站请求。该补充在临时实时日志任务清理后进行，因此只有客户端帧和源站记录，未声称包含实时边缘证据。结果限定为当前入口下切换该字段不能解除“等待请求体结束”的行为，不外推到其他供应商或其他请求形态。

### 第二、第三组的四层记录

| EdgeOne UTC | 请求状态 | 客户端 DATA | 边缘请求体统计 | 边缘/源站状态 | 源站记录 |
|---|---|---:|---:|---|---|
| 22:44:13 | 开放体，随后客户端超时/reset | 109 | 0 | 0 / -1，无回源响应头时延 | 无对应 Nginx POST / Xray 接收 |
| 22:44:28 | `END_STREAM` | 109 | 109 | 200 / 200，回源响应头 22.016 ms | Nginx POST 200、148 字节；Xray Google:80 |
| 22:44:29 | 开放体，随后客户端超时/reset | 109 | 0 | 0 / -1，无回源响应头时延 | 无对应 Nginx POST / Xray 接收 |
| 22:44:44 | `END_STREAM` | 109 | 109 | 200 / 200，回源响应头 20.710 ms | Nginx POST 200、148 字节；Xray Google:80 |

对应 RequestID 的 SHA-256 短标签分别为 `08855e54`、`1bc576d8`、`bbed0f9b`、`08710b04`，不公开原始请求标识或客户端地址。

```text
Nginx 22:44:28Z POST /vless-xhttp/ HTTP/2.0 200 148
Xray  22:44:28.730741Z accepted tcp:www.google.com:80
Nginx 22:44:44Z POST /vless-xhttp/ HTTP/2.0 200 148
Xray  22:44:44.657295Z accepted tcp:www.google.com:80
```

EdgeOne 的开放体记录分别在 14,292 / 14,282 ms 后出现 `client_request_exception.http2_reset`；该 reset 与探针的超时取消相符，不能把它当成最初的故障原因。结束体分别在 205 / 185 ms 内响应，状态为正常结束、`no_exception`，边缘响应正文 148 字节，与源站/客户端一致。

EdgeOne 官方字段把 `OriginResponseStatusCode=-1`、回源时延 `-1` 描述为没有回源。此处与同时间窗的源站阴性、以及只改变 `END_STREAM` 后的源站阳性共同使用，不能仅靠一次“无日志”推断。[实时日志字段](https://edgeone.ai/document/zh/56253)

开放体的 `RequestBodyBytes=0` 也不能解释成客户端完全没有发送：本轮明确观测到 DATA 109 已发送和对端流控帧。该差异体现的是当前边缘处理/统计在请求体完成前没有进入回源阶段。

## 关联边界与未执行项

- Android 与 Windows 时钟存在约 5.6 秒偏差；分别记录其时间，源站/EdgeOne使用自身 UTC。关联以串行顺序、校正后的窗口、路径、目标和字节数为依据，未声称客户端 RequestID 贯穿 Nginx/Xray。
- 没有使用不存在的随机域名，没有关闭 TLS 校验，没有使用 `--noproxy '*'` 搭配显式 SOCKS。
- 源站未安装 tcpdump/tshark，因此没有生成源站 443/后端 PCAP；替代证据是客户端 TLS 内侧 HTTP/2 帧元信息、恢复后的 EdgeOne 回源字段、源站访问/接收记录和 `END_STREAM` 对照。没有声称从加密的 443 抓包读出了 ALPN。
- 未启用云端 Xray debug：现有访问日志已能记录结束体请求，故障定位在其之前，不需要增加线上重启或日志级别变更。
- 公网正常 `stream-one` Google HTTPS 未通过，因此未进行公网 8 MiB / 哈希验收，也未宣称 Android TUN 可用。
- 既有本地 `no-grpc-header` 两态均通过的结果仍有效；公网两态此前均失败只说明单独切换该字段不足以修复，不能扩大为该字段在所有环境无影响。

## 修复范围与后续条件

当前找不到可据证实施的客户端/Nginx/Xray功能修复：这三个组件能在本地保持开放请求体取得响应；当前 TX 路径则要求请求体结束才回源。增加客户端超时不会解除这个等待；提前结束上传也不能提供持续双向 TCP。

若继续保留 TX + `stream-one`，需要 EdgeOne 为此入口提供明确的流式上传/完整双向 HTTP/2 回源能力。文档中现有 gRPC 两种支持模式不足以作为该能力证明。未自动升级套餐、未换协议、未改写尚未验收的原 File 配置或订阅数据。

## 清理状态

已执行并重新读取验证：

- EdgeOne 临时任务 `tx-xhttp-diag-20261007` 先停用，再删除；控制台列表为 0 条。清理状态截图保留在忽略目录 `.local/diagnostic-20261008/edgeone-log-task-cleanup.png`。
- 临时接收容器停止并删除。诊断运行目录从 `/run` 退役；448 个文件分别完成备份与退役后的 SHA-256 校验，保存在服务器私有路径 `/opt/riko-backups/tx-xhttp-evidence-20261008/snapshot-20261007T225905Z`。
- 当前 Nginx 配置先备份，再仅删除临时 POST location；`nginx -t` 和平滑重载通过，四个既有 VPN location 的逐块摘要完全未变。
- 初次从服务器 loopback 访问退役路径超时，未当作成功；改从真实 EdgeOne 入口验证后，该路径返回 HTTP 404、系统 CA 校验结果 0、HTTP/2。诊断 upstream 在运行配置中已不存在。
- 云端 Nginx、Xray、Cloudflare Tunnel 均为 running；Xray 日志仍为 warning，两条 XHTTP 入站仍为 stream-one。
- 模拟器恢复本轮前的原 APK，安装文件 SHA-256 为 `f320748909c1e5db47b7788a884bad3c8a84fbec1bba83192baf620fb3702c23`，已核对安装后的实际文件摘要。
- 模拟器原活动配置名称未变，13 个导入配置文件的摘要与本轮前全部一致；UI 为 Stopped，TunService 不存在，ADB 转发列表为空。未操作实体手机，未清应用数据。
- 本轮的请求体诊断包装器和对应测试已从正式核心源码撤下；此前已有核心改动保持。撤下后 `transport/xhttp` Go 测试通过。诊断源码快照、构建摘要和诊断 APK 保留在忽略目录供复核。
- 本地 `riko-xhttp-diag-20261008` Compose 的四个容器和专属网络已移除；用户其他 Docker 服务和 Windows Clash 未修改。
- 本轮生成的 Android UI XML 已删除。应用状态备份保留在忽略的私有目录，文件 ACL 限制为当前用户、SYSTEM 和 Administrators。原始私有证据不进入版本控制。

本轮交付的是明确的停止点与兼容性证据；没有把 TX Google HTTPS、公网 8 MiB 或 Android TUN 的未通过验收标记为完成。
