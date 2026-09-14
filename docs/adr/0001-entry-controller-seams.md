# 入口控制器对外单一、内部拆分证书与入口 seam

TrojanPanel Next 以 `EntryController` 作为安装器唯一学习的入口 Module，并在 Implementation 内部拆分 `CertificateProvider` 与 `IngressProvider` 两个 seam；Caddy legacy、nginx-certbot standalone 与 external driver 是三个真实 Adapter。这样既不把 Caddy 的 ACME/端口耦合固化为公共 Interface，也不让 Web TLS 反代与 Node 内核直连伪装成同一种流量模型；代价是必须持久化资源所有权和切换事务，换来可验证的两阶段切换、失败回滚与证书续订闭环。
