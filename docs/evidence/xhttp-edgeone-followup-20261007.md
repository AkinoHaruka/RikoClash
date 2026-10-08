# XHTTP `stream-one` EdgeOne follow-up

Date: 2026-10-07 (Asia/Singapore)

## Scope

Continued from the local WSL `no-grpc-header` A/B. The intended next step was a small live EdgeOne test using an independent Mihomo process, followed by correlation with origin-side logs. No EdgeOne, server, subscription, phone-app data, or user Clash settings were changed.

## Findings

- Windows has an active Mihomo TUN adapter using DNS server `198.18.0.2`; the Clash Verge service is running. WSL resolved `tx.vpn.riko.asia` to `198.18.0.194`, a Mihomo fake-IP address. A WSL request using the default resolver would therefore not be an independent direct-path test.
- The phone address last supplied, `10.31.3.223:46325`, is `offline` in ADB. The remembered ports `44007` and `33085` at that IP are also unavailable, and ADB mDNS discovery returned no devices. No phone configuration or data was changed.
- The only saved production-style diagnostic profile available locally was captured on 2026-10-06 and configures the TX XHTTP node as `packet-up` with `no-grpc-header: true`. It is stale for evaluating the requested `stream-one` path, so it was not treated as the current production configuration.
- A historical read-only server inspection on 2026-10-05 found both live XHTTP inbounds set to `packet-up`. That snapshot is also stale; the active server mode must be read again before interpreting any live `stream-one` test.
- A separate Windows Mihomo process was started on loopback ports `17896`/`19096`, with its outbound interface bound to the physical WLAN. The initial probe failed before receiving any HTTP response: Google, download, and upload requests all returned curl error 35, while Mihomo logged the TX node dial being cancelled. The profile's system DNS had resolved the TX host through the active Windows TUN, so this run is not evidence of an EdgeOne or XHTTP protocol failure.
- A second attempt configured the isolated Mihomo instance to query `223.5.5.5` directly over WLAN. Its DNS query timed out. A direct WLAN DoH probe to `8.8.8.8` also timed out. This machine currently does not provide a verified independent direct egress path for the live test.
- The current session exposes no Baota MCP tools and no SSH host alias, so origin-side Nginx/Xray logs could not be read or correlated.

## Interpretation

The local WSL A/B remains valid: both `stream-one` header variants pass against the local Nginx `grpc_pass` → Xray stack. The live EdgeOne A/B has not yet run. The failed Windows probes do not show that EdgeOne rejects `stream-one`; they show that this PC's current DNS/routing setup and the stale test profile cannot provide a clean test path.

The next useful test requires a currently online phone ADB connection (using the Wireless debugging **IP address & port**, not the pairing-code port) or another direct network path that bypasses the Windows Mihomo TUN. Then read-only origin logs should be captured during a fresh request made with the current `stream-one` subscription configuration.

## Cleanup

The temporary test Mihomo processes were stopped. The Ubuntu WSL instance started for the probe was terminated. The Windows Mihomo TUN, Clash Verge service, Docker Desktop, and all user configuration were left unchanged. Temporary configs containing the copied subscription credential were removed after recording this report.
