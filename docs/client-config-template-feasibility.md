# 客户端节点组与路由规则可行性调查

可行。TPNext 可以让管理员按节点服务器创建节点组，再设置域名或目标 IP 规则指向这些组。用户刷新订阅后，客户端按规则自动选组。若组启用自动测速，客户端再在组内选节点。首期适合接入 Clash.Meta（Mihomo）和 sing-box TUN，两者都有原生的规则和测速组。 [Mihomo 流量规则](https://wiki.metacubex.one/en/config/rules/) · [Mihomo URL-Test](https://wiki.metacubex.one/en/config/proxy-groups/url-test/) · [sing-box 流量规则](https://sing-box.sagernet.org/configuration/route/rule/) · [sing-box URLTest](https://sing-box.sagernet.org/configuration/outbound/urltest/)

本次只新增调查文档，依据当前源码、上游资料及本地 sing-box 静态校验。实际支持的前端、内核版本及远程订阅更新行为须通过客户端验证后确定。

## 节点组与自动分流

建议把**节点组**和**流量路由规则**做成两张关联的管理表单。节点组包含显示名称、节点服务器和组内选择方式。最小版本可按服务器自动纳入所有符合条件的节点。若同一服务器上的节点需要分组使用，再开放指定成员。路由规则包含匹配类型、匹配值、目标节点组和顺序，首期可支持完整域名、域名后缀、单个目标 IP 和 IPv4／IPv6 CIDR。规则和组之间使用稳定的组 ID 引用，改名时无需改规则。

| 匹配条件示例 | 目标节点组 |
| --- | --- |
| 域名后缀 `example.com` | 服务器 A 组 |
| 目标 IP 在 `203.0.113.0/24` 内 | 服务器 B 组 |
| 其余流量 | 管理员指定的默认组 |

- **按规则选组**。TPNext 把规则编译为 Mihomo 的 `rules` 或 sing-box 的 `route.rules`。客户端按顺序处理，具体规则放在默认规则之前。应明确是匹配连接中已知的目标 IP，还是可先解析域名再匹配 IP，并用实际客户端验证两种行为。Mihomo 的 `no-resolve` 会影响 IP 规则是否触发 DNS 解析，sing-box 可用 `resolve` 动作显式解析。客户端的全局或直连模式也要在界面上说明，现有 sing-box TUN 的模式规则可使普通分流规则不被执行。 [Mihomo 匹配顺序与 IP 解析](https://wiki.metacubex.one/en/config/rules/) · [sing-box 路由动作](https://sing-box.sagernet.org/configuration/route/rule_action/) · [现有 TUN 模式规则](../apps/control-plane/api/service/subscribe.go#L711)
- **组内选节点**。测速自动选择可分别生成 Mihomo 的 `url-test` 和 sing-box 的 `urltest`。两者都在客户端运行。测速地址、探测间隔和切换容差应使用目标客户端支持的配置。同一服务器上的节点可以提供协议和端口的选择，整台服务器故障时，同组节点可能一起不可用。跨服务器故障切换需要另行定义。 [Mihomo URL-Test](https://wiki.metacubex.one/en/config/proxy-groups/url-test/) · [sing-box URLTest](https://sing-box.sagernet.org/configuration/outbound/urltest/)
- **按账户和客户端生成组**。Node 数据已有 `node_server_id`，订阅查询需要补读该字段。当前 Clash.Meta 和 sing-box TUN 各生成一个 `PROXY` 组，可以沿用原有节点生成器，再按服务器 ID 组装多个组。组成员取账户当前授权且能导出给该客户端的节点。规则组在没有可用成员时，可走管理员明确配置的可用备用组，或按规则拒绝这部分流量。组内没有成员与客户端版本不兼容是两种情况，应各自处理。 [节点模型](../apps/control-plane/api/model/node.go#L5) · [节点订阅查询](../apps/control-plane/api/dao/node.go#L317) · [Clash.Meta 代理组](../apps/control-plane/api/service/subscribe.go#L286) · [sing-box TUN 出站](../apps/control-plane/api/service/subscribe.go#L328)

基础模板和 DNS 路由模板可以继续按客户端独立维护。域名到了哪个代理组由流量规则决定，域名查哪个解析器由 DNS 策略决定。发布时合成并保存包含基础配置、节点组定义、流量规则和 DNS 策略的版本。用户刷新时，再按当前权限生成组成员和节点凭据。 [订阅入口](../apps/control-plane/api/api/subscribe.go#L78) · [节点授权筛选](../apps/control-plane/api/service/subscribe.go#L420)

## 客户端支持范围

| 客户端或模式 | 模板可行性 | TPNext 接入方式 |
| --- | --- | --- |
| Clash.Meta（Mihomo） | 支持原生 YAML。流量规则使用 `rules`、`rule-providers`，DNS 可通过 `dns.nameserver-policy` 等字段选解析器。 [流量规则](https://wiki.metacubex.one/en/config/rules/) · [DNS 配置](https://wiki.metacubex.one/en/config/dns/) | 将现有文本拼接改成结构化合成，发布时检查重复键，订阅时填充 `proxies` 和面板负责的代理组。 [当前订阅](../apps/control-plane/api/api/subscribe.go#L123) · [当前代理组生成](../apps/control-plane/api/service/subscribe.go#L286) |
| sing-box TUN | 支持原生 JSON，图形客户端规范支持从 URL 更新完整配置。流量路由使用 `route`，DNS 使用 `dns`。`route.rules` 的 `hijack-dns` 与 DNS 设置联动，DNS 和规则集下载字段存在内核版本差异。 [远程配置](https://sing-box.sagernet.org/clients/general/#remote) · [路由字段](https://sing-box.sagernet.org/configuration/route/) · [DNS 字段](https://sing-box.sagernet.org/configuration/dns/) · [路由动作](https://sing-box.sagernet.org/configuration/route/rule_action/) · [迁移说明](https://sing-box.sagernet.org/migration/) · [规则集下载](https://sing-box.sagernet.org/configuration/rule-set/) | 在现有生成器上接入合成结果，保留 TUN、DNS、内置模式规则和动态 `outbounds` 的明确归属，按目标版本校验。 [当前 TUN 生成器](../apps/control-plane/api/service/subscribe.go#L471) |
| sing-box Outbound only | 现有模式为每个节点生成一个本地 SOCKS 入站，并生成入站到节点的绑定规则。首期保留该模式的生成逻辑，若以后开放路由配置，需要定义绑定和自定义规则间的优先级。 [当前 Outbound only 生成器](../apps/control-plane/api/service/subscribe.go#L513) | 继续按 `outbound` 模板 ID 下发。 |
| V2Ray 类及 Shadowrocket | TPNext 当前给这两类下发 Base64 节点 URI 列表。v2rayN 的标准订阅支持节点分享链接，Shadowrocket 的开发商说明支持规则文件导入和 DNS，但这些资料无法确认 TPNext 的现有节点订阅能一并更新规则。 [当前 URI 生成器](../apps/control-plane/api/service/subscribe.go#L385) · [v2rayN 订阅说明](https://github.com/2dust/v2rayN/wiki/Description-of-subscription) · [Shadowrocket 开发商说明](https://apps.apple.com/us/app/shadowrocket/id932747118) | 现有入口继续下发节点列表。若以后支持某应用的完整配置，单独确定目标版本、导入入口、更新机制和回退文件。 |
| Xray Core | 支持完整配置的 `routing` 和 `dns`。 [Xray 路由](https://xtls.github.io/en/config/routing.html) · [Xray DNS](https://xtls.github.io/en/config/dns.html) | 当前面板的 Xray 模板用于 Node 服务端配置，若提供 Xray 客户端完整 JSON，需要单独建立客户端模板及导出目标。 [当前下发到 Node 的模板](../apps/control-plane/api/service/node.go#L203) |

可单独验证 v2rayNG 的完整配置订阅。当前上游 `master` 的订阅刷新流程尝试导入含 `inbounds`、`outbounds` 和 `routing` 的自定义 JSON，实际支持范围还要按发布版本和平台实测。该能力适合做成明确可选的应用专用订阅入口。 [v2rayNG 订阅刷新源码](https://github.com/2dust/v2rayNG/blob/master/V2rayNG/app/src/main/java/com/v2ray/ang/handler/AngConfigManager.kt)

## 合成和保存

1. **管理员维护三部分**。基础配置可包括日志、监听及 TUN 等参数，流量路由负责匹配、顺序和默认出站，DNS 路由负责解析器、匹配及默认解析策略。常用分流由同一份结构化规则生成各客户端的原生字段。同一份规则集可能同时被流量与 DNS 引用，合成器需要统一校验这些引用、出站组、解析器标签和 DNS 启动依赖。规则列表保留管理员设定的顺序，各部分有固定字段归属，冲突时提示。 [Mihomo 规则顺序](https://wiki.metacubex.one/en/config/rules/#priority) · [Mihomo DNS 启动依赖](https://wiki.metacubex.one/en/config/dns/#respect-rules) · [sing-box DNS 动作](https://sing-box.sagernet.org/configuration/dns/rule_action/)
2. **保存成功后发布**。一次确认保存时，Web 服务重新合成本次受影响的目标并校验。校验全部通过后，将模板原文、合成快照、操作者、时间和适用目标写入版本记录，再原子切换当前版本。校验或写入失败时，线上继续提供上一有效版本，管理员能看到失败原因。并发编辑应校验起始版本，避免后保存的人覆盖另一位管理员的发布。当前系统设置直接覆盖模板文件，版本记录与原子切换需要新增。 [现有保存路径](../apps/control-plane/api/service/system.go#L204)
3. **订阅时填入动态数据**。服务用订阅 URL 现有的 `client`、`template` 参数选择版本，按请求校验账户并读取当前节点授权、节点信息和凭据，最终返回带节点的 YAML 或 JSON。同一种客户端下，不同账户的节点权限和凭据可能不同，且节点改名、改端口、权限撤回或凭据更换都能独立于模板发生。全量提前生成最终文件需要为每个账户和目标生成一份，并处理上述变化的失效，成本高于保存公共配置快照并按需填充。 [现有目标选择](../apps/control-plane/api/api/subscribe.go#L165) · [现有账户与节点生成](../apps/control-plane/api/service/subscribe.go#L312) · [节点凭据生成](../apps/control-plane/api/service/subscribe.go#L930)

## 历史和回退

- **历史管理**。管理员可以查看、比较、恢复、修改说明和归档历史版本。编辑历史内容时从该版本复制出新版本，校验后发布，保留原发布记录。删除限于未使用的历史版本，当前版本和已指定的回退版本保留可用。回滚历史配置时，订阅仍按当前账户授权生成。若配置引用会持续更新的远程规则集，恢复历史配置只能恢复当时的引用。需要精确复现规则内容时，另行固定规则集的版本或内容。 [Mihomo 规则集更新周期](https://wiki.metacubex.one/en/config/rule-providers/#interval) · [sing-box 规则集更新周期](https://sing-box.sagernet.org/configuration/rule-set/#update_interval)
- **兼容回退**。对于已知缺少某项能力的目标，选择预先校验过的同客户端、同模式的兼容默认模板，并显示实际下发的版本。若回退会去掉管理员要求保留的阻断或 DNS 策略，则应保留仍符合要求的有效版本，或提示该目标不支持发布。URI 订阅目标继续使用其既有格式。管理员保存了语法错误或引用无效的模板时，应保留线上版本并给出错误，便于修正。
- **能力识别**。服务端能可靠识别的是订阅链接显式指定的输出类型，以及面板已确认的兼容目标。若需要同时支持一个客户端的不同内核版本，可以为其提供明确可选的兼容版本。当前接口的 User-Agent 仅用于 URI 输出细节，没有客户端载入结果回传，因此服务端无法普遍判断某个应用是否实际应用了配置。 [订阅分发与请求头](../apps/control-plane/api/api/subscribe.go#L89) · [URI 格式选择](../apps/control-plane/api/service/subscribe.go#L397)

## 首期范围和验证

本地使用 sing-box 1.14.4 进行静态检查。TPNext 内置 TUN 配置以及拆分重组后加入模拟 VLESS 出站、自定义流量与 DNS 规则的配置都通过 `sing-box check`。另两个样例也通过了检查，一个使用域名后缀和目标 CIDR 指向两个测速组，另一个模拟账户无权使用其中一组，改为匹配后拒绝。将早期样例的 DNS 规则目标改为不存在的 `missing-dns` 后，命令仍返回成功。因此，发布校验需要另外核对 DNS 服务器等跨模板标签引用。该检查未覆盖图形客户端导入、实际 DNS 解析、测速或转发。 [sing-box 配置校验](https://sing-box.sagernet.org/configuration/#check)

建议先在 Clash.Meta 和 sing-box TUN 做通按服务器分组、域名／IP 选组、组内测速及默认组，并延用原订阅 URL。未启用新规则的用户应保有原来的规则顺序、DNS、代理组和可见节点。完整模板拆分和历史管理可以随后接入同一发布流程。使用目标版本的 `mihomo -t`、`sing-box check` 验证合成产物，并用计划支持的客户端检查订阅刷新、域名和 IPv4／IPv6 匹配、空组回退、权限变化、规则优先级、DNS 分流、校验失败和回滚。 [Mihomo 校验入口](https://github.com/MetaCubeX/mihomo/blob/Meta/main.go) · [sing-box 配置校验](https://sing-box.sagernet.org/configuration/#check)
