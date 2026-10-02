# Proxy node port forwarding

[Documentation](README_EN.md) · [Web user guide](user-guide.md#proxies) · [简体中文](port-forwarding.md)

## Contents

- [External and actual ports](#port-fields)
- [Configure in Web](#web-configuration)
- [Route TCP 443 by TLS SNI with Nginx](#tls-sni)
- [Hysteria2 UDP and port hopping](#udp-and-hopping)
- [Checks and maintenance](#validation)

<a id="port-fields"></a>
## External and actual ports

With Nginx, NAT, or another forwarding service in front of a proxy, clients can use an external port while Node listens on another actual port.

| Field | Range | Purpose |
| --- | --- | --- |
| Actual port `port` | `101–29999` | Port Node uses to create and manage the proxy listener |
| External port `externalPort` | `1–65535` when enabled | Entry port in generated subscriptions, connection links, and client configuration |
| `externalPort: 0` | Forwarding configuration disabled | Client configuration uses the actual port |

For actual port `11443` and external port `443`, clients connect to public port `443`, and an external service must forward traffic to Node port `11443`. Changing the external port does not change Node's listener; its actual port must remain available on the host.

**This feature records the mapping and generates client endpoints. It does not deploy forwarding services or configure Nginx, NAT, firewalls, or certificates.** Administrators maintain the forwarding path. Proxy mappings do not change server gRPC or host maintenance ports.

<a id="web-configuration"></a>
## Configure in Web

1. Open **Node management**, create or edit a proxy, and choose its server and protocol.
2. Enable **Port forwarding**, then enter **External port** and **Actual port**, such as `443` → `11443`.
3. Configure protocol, domain, TLS / SNI, and account parameters, then save.
4. Configure forwarding for the correct protocol on the host or external gateway, check firewall access, and refresh subscriptions or import the node again.

Disabling forwarding makes generated client configuration use the actual port. Already imported configuration does not change automatically; ensure clients can reach the actual port before switching. Removing or editing a proxy does not update external forwarding rules.

<a id="tls-sni"></a>
## Route TCP 443 by TLS SNI with Nginx

This example combines **one TPNext Xray Trojan/TLS TCP proxy** with **one independent TLS service**. Both domains resolve to the same public host. Nginx passes original TLS traffic to the selected backend, which presents the certificate. `ssl_preread` reads ClientHello SNI without terminating TLS and requires available stream/ssl_preread modules. [Nginx documentation](https://nginx.org/en/docs/stream/ngx_stream_ssl_preread_module.html)

| Service | Client domain | Public entry | Backend |
| --- | --- | --- | --- |
| TPNext Trojan/TLS | `proxy.example.com` | TCP `443` | Node TCP `11443` on this host |
| Independent TLS service | `site.example.com` | TCP `443` | Service TCP `8443` on this host |

Enable forwarding in TPNext with external port `443`, actual port `11443`, TCP transport, and TLS. Client SNI is `proxy.example.com`. Distinct domains select distinct backends; this example does not route two services sharing the same SNI.

### Certificates and listeners

- Node's certificate must cover `proxy.example.com`; client TLS server name and routing SNI must match. Keep certificate verification enabled.
- Node uses its deployment certificate paths. Port mapping does not add separate certificate management for each proxy. When several TLS domains share Node's default certificate, it must cover every domain.
- The independent service handles TLS on `8443` with its own certificate for `site.example.com`, managed independently from TPNext.
- Nginx stream owns public TCP `443`. Move an independent service already listening there to its backend port. Restrict backend access to the forwarding entrypoint or trusted management sources.
- If host services own issuance ports, prepare certificates and use [Node external certificate mode](deployment_EN.md#external-certificates). Port mapping does not resolve ACME port conflicts.

See [Xray Trojan documentation](https://xtls.github.io/en/config/inbounds/trojan.html) for protocol TLS settings.

### Host configuration example

Place `stream` at the same level as `http` in the main Nginx configuration, outside website HTTP configuration. If a `stream` block exists, merge the map/server definitions into it. Load modules according to the host distribution.

```nginx
stream {
    map $ssl_preread_server_name $tls_destination {
        proxy.example.com 127.0.0.1:11443;
        site.example.com  127.0.0.1:8443;
        default           127.0.0.1:8443;
    }

    server {
        listen 443;
        # Add listen [::]:443; if a public IPv6 entry is needed.
        ssl_preread on;
        proxy_connect_timeout 5s;
        proxy_timeout 1h;
        proxy_pass $tls_destination;
    }
}
```

Unknown or absent SNI goes to the independent service in this example; choose the appropriate default backend. `listen` does not enable `ssl`: Nginx neither reads the proxy certificate nor terminates/recreates client TLS. For a backend on another host, replace `127.0.0.1` with its trusted address. [Stream context](https://nginx.org/en/docs/stream/ngx_stream_core_module.html#stream)

The example does not send PROXY protocol. Enable `proxy_protocol` only when the backend explicitly supports it and is configured to accept it; an ordinary TLS backend cannot consume a PROXY header as TLS ClientHello. [Nginx proxy protocol directive](https://nginx.org/en/docs/stream/ngx_stream_proxy_module.html#proxy_protocol)

Validate and reload on the host:

```bash
nginx -t
systemctl reload nginx
```

Check both public domains and confirm their respective backend certificates:

```bash
openssl s_client -connect proxy.example.com:443 -servername proxy.example.com -verify_hostname proxy.example.com -verify_return_error </dev/null
openssl s_client -connect site.example.com:443 -servername site.example.com -verify_hostname site.example.com -verify_return_error </dev/null
```

Successful TLS handshakes establish routing/certificate connectivity; also verify authentication and forwarding with a real proxy client.

<a id="udp-and-hopping"></a>
## Hysteria2 UDP and port hopping

Hysteria2's client-to-server transport uses QUIC / UDP even when the proxied destination traffic is TCP. It therefore needs UDP forwarding and cannot directly use the TCP `ssl_preread` example to route by domain. [Hysteria2 protocol](https://v2.hysteria.network/docs/developers/Protocol/)

TCP `443` and UDP `443` are separate listener resources. One host can use TCP `443` for the TLS routing above and UDP `443` for one Hysteria2 backend. Different domain names alone do not allow multiple Hysteria2 services to share this UDP entrypoint. Check for existing HTTP/3 or other QUIC services using UDP `443`; give conflicting UDP services separate public ports.

For Hysteria2 actual port `11445` and external port `443`, configure **UDP `443` → UDP `11445`** on the gateway. Host NAT or Nginx stream UDP proxying can provide this. Nginx needs `listen ... udp` without `ssl_preread`; a TCP rule does not also forward UDP. [Nginx UDP proxying](https://nginx.org/en/docs/stream/ngx_stream_proxy_module.html)

### Public port hopping ranges

Hysteria2 **Port hopping** defines the public port set/range clients can reach, such as `30000-30100`. It does not change Node's single actual listener or automatically configure forwarding.

With actual port `11445` and public range `30000-30100`, forward **every UDP port** in the range to UDP `11445` and allow the full range through firewalls. If external single-port `443` remains an exported entry, keep its UDP forwarding available too. Forwarding only `443` does not provide the hopping range. Client support for hopping extensions varies; inspect actual exported configuration.

Clients select and switch ports from the public set, so every member must be reachable. Administrators maintain firewall, NAT, and forwarding rules; TPNext writes hopping parameters into client formats that support them. [Hysteria2 port hopping](https://v2.hysteria.network/docs/advanced/Port-Hopping/)

<a id="validation"></a>
## Checks and maintenance

| Check | Requirement |
| --- | --- |
| Node listener | Actual port matches the panel and has no conflict |
| Forwarding protocol | Forward TCP for TCP proxies, UDP for Hysteria2; configure both independently when needed |
| Client entry | Regenerated subscriptions/configuration use external port, or actual port when disabled |
| TLS domains | Routing SNI, client TLS server name, and backend certificate coverage agree |
| Public reachability | DNS, IPv4 / IPv6, firewalls, and the complete forwarding path work |
| Port hopping | Every public hopping port reaches the actual UDP backend |
| Management access | gRPC and host maintenance remain under their original port/access rules |

An online panel status does not establish public forwarding readiness. After external-port changes, check rules and refresh client subscriptions; actual-port changes also require new forwarding targets. Project removal does not remove independently managed host Nginx, NAT, or forwarding services.
