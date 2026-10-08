# Android ChatGPT + Ali WS SSL 诊断进展

日期：2026-10-08。状态：**Android App 层仍未复现，不能判定 SSL 根因。** 本记录把已验证的电脑侧事实与缺失的 Android 证据分开。

## 已有公网证据

此前 Ali WS 基线记录使用独立 Windows Mihomo SOCKS，经 USB 网络到 Ali ESA 边缘，再通过 `/vless-ws` 转发到 Xray。对 Ali 边缘证书执行了严格校验：SAN 包含 `ali.vpn.riko.asia`，签发者为 Let's Encrypt YE2，证书有效期截至 2026-12-21；TLS 1.3、ALPN `http/1.1`。四组 Google 204 测试各为 3/3 成功，未跳过证书校验。详见 [Ali WS 公网基线](ali-ws-adaptation-20261008.md)。

同一电脑侧诊断中，ChatGPT 与 Claude 各执行了一次目标请求：TLS 握手正常，随后 HTTP 403，响应含 `cf-mitigated: challenge`。样本各只有一次；这证明那两次请求不是证书握手失败，不能证明 Android App 也会如此，更不能单凭 403 推断出口 IP 信誉是唯一原因。

## 本轮 Android 与源码核对

- 当前 ADB 只连接 Android Studio 的 Pixel_10 模拟器；用户手机未连接且保持未操作。模拟器没有安装 ChatGPT App。
- RikoClash Debug 包运行配置仍是 XHTTP 示例配置：55 个示例出站，没有 `ali.vpn.riko.asia`。本地配置数据库仍为原来的 13 个配置；本轮没有保存临时配置，当前模拟器 VPN 已恢复为原运行状态。
- 独立 `RikoAliWsDiag` AVD 冷启动后持续 ADB `offline`，无法执行网络测试。本轮停止了该 AVD 进程，没有清除或重置它的数据。
- Antigravity 当前工作树改动涉及 APK 更新下载选择、Release 签名及构建流程；`ProxyActivity.kt`、`TunService.kt` 和 `network_security_config.xml` 均无本地改动。因此，已检查到的 Antigravity 源码差异没有证据表明会在 WS 节点切换时重启 TUN，或更改 RikoClash 的证书信任配置。该源码观察不等同于已安装 APK 与当前工作树版本一致。

## 当前判断与下一步

目前没有证据证明 Android 的提示来自 Ali 边缘证书错误，也没有证据证明 Android 的 ChatGPT TLS 已正常。电脑侧证书与目标 TLS 正常只能作为对照；真实问题仍需在出现错误的 Android ChatGPT App 和 Ali WS 同一会话中采集域名、目标 IP、选中节点、TLS 错误及 Mihomo 日志。

下一轮设备可用时，按同一 Wi-Fi 串行对照 Ali WS 与腾讯 WS；先确认实际失败域名，再严格验证该目标域名的证书 SAN、签发链和有效期，同时记录 Mihomo/App 错误。若目标 TLS 成功后出现挑战或 WebSocket 失败，分开检查 OpenAI 官方列出的依赖域名及 `wss://ws.chatgpt.com` 的 TCP/443 WebSocket Upgrade；这类连接失败不能归类为证书验证失败。[OpenAI 网络建议](https://help.openai.com/en/articles/9247338-network-recommendations-for-chatgpt-errors-on-web-and-apps)

本轮未修改服务器、EdgeOne 规则、订阅、应用源码或用户手机数据；未安装 CA，也未关闭证书校验。
