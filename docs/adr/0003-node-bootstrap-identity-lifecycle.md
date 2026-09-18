# Node 引导包绑定可独立撤销的 Node 身份

Web 主控负责登记 Node 并生成使用 `age` scrypt 口令加密的 Node 引导包；包可重复用于该 Node 的安装，但不得携带 Web CA 私钥或 Web mTLS 客户端私钥。每个 Node 使用独立的 MariaDB 用户、Redis ACL 用户和控制面登记，业务数据仍然共享；Web CLI 提供 `revoke`、`rotate` 与失联时的强制踢除，使旧引导包和数据层身份可以独立失效。该方案需要扩展当前 Redis 用户配置与身份生命周期实现，但避免共享 root/全局密码使单节点泄露扩大为整个控制面的数据层失守。
