# XHTTP `stream-one` WSL verification

Date: 2026-10-06

## Scope

An isolated Docker Compose stack was run from WSL using the locally restored Linux images. It contained Mihomo as the client, Xray 26.3.27 as the VLESS/XHTTP server, Nginx 1.30.5 as the ingress, and a disposable Nginx WebDAV endpoint as the upload/download target. Test configuration used a dummy UUID and no production credentials. No phone, user Clash process, public server, or Cloudflare/EdgeOne setting was changed.

## Results

- Mihomo `stream-one` directly to the Xray inbound reached `https://www.google.com/generate_204`: HTTP 204.
- Through Nginx `proxy_pass`, the same client/server pair returned HTTP 400. Xray logged `http: invalid Read on closed Body`.
- Through Nginx `grpc_pass grpc://xray:10084`, Google returned HTTP 204.
- Through that `grpc_pass` route, an 8,388,608-byte random PUT to the internal DAV endpoint returned HTTP 201; downloading it returned HTTP 200 and 8,388,608 bytes. Source and downloaded SHA-256 both equaled `75BC9160B202153C278B7DBE2C515643A62BEE497B1715DB368FEE76392A2E96`.
- The local success demonstrates bidirectional `stream-one` data flow across the tested WSL stack. The measured local transfer times are not Internet speed measurements.

## Limits and next validation

This did not exercise TLS termination, the EdgeOne or Cloudflare edge, current production Nginx/Xray configuration, subscription delivery from the live service, or the Android TUN path. Before changing the live route, inspect and back up its active Nginx/Xray configurations, confirm the exact route, then apply the same `stream-one`/`grpc_pass` behavior and verify public TLS plus a real uploaded/downloaded payload. A phone test remains separate and is not claimed here.
