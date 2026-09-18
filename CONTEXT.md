# TrojanPanel Next 领域上下文

本上下文定义部署入口、证书、安装交付与节点身份生命周期的共同语言，避免把某一种反向代理或管理系统的实现细节当成产品概念。

## 领域语言

**入口（Entry）**：
一组为某次部署提供证书、HTTP/HTTPS 接入与可选明文回落的宿主能力。入口不天然包含节点协议流量转发。
_避免使用_：反代容器、Caddy 模式

**入口控制器（EntryController）**：
把入口的期望状态对齐到宿主观测状态，并维护资源所有权与切换恢复信息的 Module。
_避免使用_：安装器 TLS 分支、代理管理器

**入口 Adapter（Entry Adapter）**：
在入口控制器内部实现某一种宿主入口方式的 Adapter；当前入口方式包括 Caddy legacy、nginx-certbot standalone 与 external driver。
_避免使用_：TLS 模式

**证书生产者（Certificate Provider）**：
对一个域名签发或取得证书、维持续订并发布证书代次的一方。
_避免使用_：证书目录

**入口提供者（Ingress Provider）**：
拥有声明的宿主监听与请求处理规则的一方；它可以终止 Web TLS、提供 ACME HTTP-01 或明文回落，但不必接管节点协议端口。
_避免使用_：反向代理

**证书引用（CertificateRef）**：
证书消费者可稳定读取的一对证书与私钥，以及标识内容版本的指纹和代次。
_避免使用_：Caddy 证书路径、certbot lineage

**入口部署（Entry Deployment）**：
由稳定 deployment ID 标识、可独立对齐和移除的一份入口声明及其受管资源集合。

**资源所有权（Resource Ownership）**：
入口部署对宿主文件、监听、容器、定时器和证书材料可执行变更或删除的权利；没有所有权的资源只能观测。

**观测状态（ObservedState）**：
入口部署在宿主上的当前事实，包括活动 Adapter、能力、资源、证书代次、监听、健康与未完成切换。

**内核直连（Direct Kernel Exposure）**：
节点代理内核直接拥有其协议端口并自行终止所需 TLS 的暴露方式。
_避免使用_：external L4 默认模式

**明文回落（Plain Fallback）**：
代理内核完成 TLS 后，把非代理 HTTP 字节流送往本机明文监听的行为。
_避免使用_：HTTPS 回落

**裸机安装（Bare VPS Installation）**：
不依赖外部 VPS 管理系统，从受支持的全新服务器状态建立 TrojanPanel Next 部署的产品能力。
_避免使用_：管理系统安装、源码部署

**部署模式（Deployment Mode）**：
一份安装配置声明的服务器职责，规范值为 `web`、`node` 或 `combined`。
_避免使用_：安装类型、服务器类型

**Web 主控（Web Control Plane）**：
拥有管理界面、控制 API、共享业务数据和 Node 身份生命周期的部署职责。
_避免使用_：面板端、服务端

**Node Agent**：
执行代理内核、提供受控 gRPC 能力并使用 Web 主控共享业务数据的部署职责。
_避免使用_：节点端、Core 端

**combined 部署（Combined Deployment）**：
在同一宿主上同时承担 Web 主控与 Node Agent 职责，并由一个共享入口管理两个域名的部署模式。
_避免使用_：同机双装

**版本化安装资产（Versioned Installation Assets）**：
同一发布版本中的 bootstrap、配置模板、安装器、清单与校验材料的完整集合。
_避免使用_：latest 脚本、仓库安装文件

**不可变发布清单（Immutable Release Manifest）**：
把版本化安装资产绑定到不可变容器镜像 digest 与资产摘要的发布事实。
_避免使用_：镜像 tag 列表

**Node 引导包（Node Bootstrap Bundle）**：
由 Web 主控为一个已登记 Node 生成、经口令加密且可转移到目标服务器的长期安装材料集合；它不包含 Web CA 私钥或 Web mTLS 客户端私钥。
_避免使用_：一次性包、Node 配置备份

**Node 身份（Node Identity）**：
一个 Node 在控制面登记、数据库身份、Redis ACL 身份和网络授权中的可独立识别与撤销边界。
_避免使用_：共享 Node 密码

**凭据轮换（Credential Rotation）**：
为同一 Node 身份签发新凭据和新引导包，并使旧引导包失效的生命周期动作。
_避免使用_：重新导出配置

**强制踢除（Forced Eviction）**：
Node 失联时，不等待远端响应即可从 Web 主控撤销其登记、数据层身份与控制面授权的动作；它不承诺删除失联宿主上的进程或数据。
_避免使用_：远程卸载、强制删除服务器

**同版本收敛（Same-version Reconciliation）**：
安装器在发布版本不变时，把非破坏性配置变化安全对齐到目标状态，并拒绝未获明确授权的破坏性变化。
_避免使用_：自动升级、强制重装

**网络放行清单（Network Allowlist Plan）**：
安装器根据部署拓扑生成的最小端口与来源地址要求；它是操作者配置主机防火墙和云安全组的依据，不表示安装器拥有这些规则。
_避免使用_：自动防火墙配置
