# XHTTP `stream-one` `no-grpc-header` WSL A/B

Date: 2026-10-07 (Asia/Singapore)

## Scope

Ran an isolated WSL 2 Docker Compose test with the existing local Xray, Nginx gRPC ingress, and disposable WebDAV destination. The client was built from the current RikoClash Mihomo source and ran as a separate container on the isolated Compose network. No phone, user Clash process, public server, EdgeOne setting, subscription, or production credential was used.

The client source was `core/src/foss/golang/clash`, commit `88dcbf7f`, with uncommitted changes present at test time. The test binary reports Mihomo Meta 1.10.0, Linux amd64, Go 1.26.3. The explicit client mode was `stream-one` in both runs. The two client YAML files differ only in `no-grpc-header` (`false` vs `true`). Both use the disposable test UUID `11111111-1111-4111-8111-111111111111`.

The Compose stack used Xray 26.3.27, Nginx 1.30.5, and the local WebDAV Nginx target. The ingress config used `grpc_pass grpc://xray:10084`; the Mihomo container reached the ingress service on the isolated Compose bridge. The proxy listener was not published to the Windows host or LAN.

## Results

| Client setting | Google `/generate_204` | 8 MiB PUT to WebDAV | 8 MiB GET | SHA-256 |
|---|---:|---:|---:|---|
| `no-grpc-header: false` | HTTP 204 | HTTP 201, 8,388,608 bytes | HTTP 200, 8,388,608 bytes | Match |
| `no-grpc-header: true` | HTTP 204 | HTTP 201, 8,388,608 bytes | HTTP 200, 8,388,608 bytes | Match |

Source and both downloads had SHA-256 `75bc9160b202153c278b7dbe2c515643a62bee497b1715db368fee76392a2e96`.

The Xray logs show accepted VLESS requests for both Google and `dav:8080`. Nginx logged the XHTTP ingress as HTTP/2 POST requests, and the WebDAV server logged successful PUT and GET requests.

## Interpretation and limits

The tested local path accepts both the gRPC-header-present and no-gRPC-header `stream-one` forms. Therefore, `no-grpc-header: true` by itself does not explain the earlier failure through this local Nginx `grpc_pass` stack. It may still affect behavior at EdgeOne; this test did not include EdgeOne, TLS termination, the public origin route, Android TUN, or the live subscription configuration. The EdgeOne failure location and the phone's currently applied runtime configuration remain unverified by this test.

This is a WSL local-stack result, not a public-route or phone acceptance result.
