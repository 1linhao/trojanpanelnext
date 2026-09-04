package dao

import (
	"database/sql"
	"github.com/didi/gendry/manager"
	_ "github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"
	"net/url"
	"strings"
	"time"
	"trojan-panel/core"
)

var db *sql.DB

// InitMySQL 初始化数据库
func InitMySQL() {
	mySQLConfig := core.Config.MySQLConfig
	var err error

	db, err = manager.
		New("trojan_panel_db", mySQLConfig.User, mySQLConfig.Password, mySQLConfig.Host).
		Set(
			manager.SetCharset("utf8mb4"),
			manager.SetParseTime(true),
			manager.SetAllowCleartextPasswords(true),
			manager.SetInterpolateParams(true),
			manager.SetTimeout(1*time.Second),
			manager.SetReadTimeout(1*time.Second),
			manager.SetLoc(url.QueryEscape("Asia/Shanghai"))).
		Port(mySQLConfig.Port).Open(true)

	if err != nil {
		logrus.Errorf("database connection err: %v", err)
		panic(err)
	}

	var count int
	if err = db.QueryRow("SELECT COUNT(1) FROM information_schema.TABLES WHERE table_schema = 'trojan_panel_db' GROUP BY table_schema;").
		Scan(&count); err != nil && err != sql.ErrNoRows {
		logrus.Errorf("query database err: %v", err)
		panic(err)
	}
	if count == 0 {
		if err = SqlInit(sqlInitStr); err != nil {
			logrus.Errorf("database import err: %v", err)
			panic(err)
		}
	}
	if err = migrateNodeUotColumns(); err != nil {
		logrus.Errorf("database migration err: %v", err)
		panic(err)
	}
	if err = migrateNodeXrayClientOptionColumns(); err != nil {
		logrus.Errorf("database migration err: %v", err)
		panic(err)
	}
	if err = migrateNodeClientTypesColumn(); err != nil {
		logrus.Errorf("database migration err: %v", err)
		panic(err)
	}
	if err = migrateClientExportPermissions(); err != nil {
		logrus.Errorf("database migration err: %v", err)
		panic(err)
	}
	if err = migrateHysteria2PortHoppingColumns(); err != nil {
		logrus.Errorf("database migration err: %v", err)
		panic(err)
	}
	if err = migrateKernelUpgradeSchema(); err != nil {
		logrus.Errorf("kernel upgrade database migration err: %v", err)
		panic(err)
	}
	if err = migrateTrafficAccountingSchema(); err != nil {
		logrus.Errorf("traffic accounting database migration err: %v", err)
		panic(err)
	}
}

func migrateTrafficAccountingSchema() error {
	migrations := []string{
		"ALTER TABLE `node_server` ADD COLUMN `traffic_period` varchar(8) NOT NULL DEFAULT 'none' COMMENT 'none/day/month/year' AFTER `grpc_tls_server_name`",
		"ALTER TABLE `node_server` ADD COLUMN `traffic_limit_mode` varchar(8) NOT NULL DEFAULT 'combined' COMMENT 'combined/separate' AFTER `traffic_period`",
		"ALTER TABLE `node_server` ADD COLUMN `traffic_total_limit` bigint unsigned NOT NULL DEFAULT 0 COMMENT 'combined traffic byte limit' AFTER `traffic_limit_mode`",
		"ALTER TABLE `node_server` ADD COLUMN `traffic_upload_limit` bigint unsigned NOT NULL DEFAULT 0 COMMENT 'upload byte limit' AFTER `traffic_total_limit`",
		"ALTER TABLE `node_server` ADD COLUMN `traffic_download_limit` bigint unsigned NOT NULL DEFAULT 0 COMMENT 'download byte limit' AFTER `traffic_upload_limit`",
		`CREATE TABLE IF NOT EXISTS account_traffic_total (
			account_id bigint unsigned NOT NULL,
			upload bigint unsigned NOT NULL DEFAULT 0,
			download bigint unsigned NOT NULL DEFAULT 0,
			create_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
			update_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (account_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS account_server_traffic_daily (
			traffic_date date NOT NULL,
			account_id bigint unsigned NOT NULL,
			node_server_id bigint unsigned NOT NULL,
			upload bigint unsigned NOT NULL DEFAULT 0,
			download bigint unsigned NOT NULL DEFAULT 0,
			create_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
			update_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (traffic_date, account_id, node_server_id),
			KEY idx_server_date_account (node_server_id, traffic_date, account_id),
			KEY idx_account_date (account_id, traffic_date)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS account_traffic_daily (
			traffic_date date NOT NULL,
			account_id bigint unsigned NOT NULL,
			upload bigint unsigned NOT NULL DEFAULT 0,
			download bigint unsigned NOT NULL DEFAULT 0,
			create_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
			update_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (traffic_date, account_id),
			KEY idx_account_date (account_id, traffic_date)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			if strings.Contains(err.Error(), "Duplicate column name") {
				continue
			}
			return err
		}
	}
	if _, err := db.Exec(`INSERT INTO account_traffic_total (account_id, upload, download)
		SELECT id, upload, download FROM account
		ON DUPLICATE KEY UPDATE account_id=VALUES(account_id)`); err != nil {
		return err
	}
	var hasAccountDailyRows bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM account_traffic_daily LIMIT 1)`).Scan(&hasAccountDailyRows); err != nil {
		return err
	}
	if !hasAccountDailyRows {
		if _, err := db.Exec(`INSERT INTO account_traffic_daily (traffic_date, account_id, upload, download)
			SELECT traffic_date, account_id, SUM(upload), SUM(download)
			FROM account_server_traffic_daily GROUP BY traffic_date, account_id`); err != nil {
			return err
		}
	}
	for _, permissionPath := range []string{"/api/dashboard/serverTrafficUsage", "/api/dashboard/serverTrafficUserUsage"} {
		for _, role := range []string{"sysadmin", "admin"} {
			var count int
			if err := db.QueryRow("SELECT COUNT(1) FROM casbin_rule WHERE p_type='p' AND v0=? AND v1=? AND v2='GET'", role, permissionPath).Scan(&count); err != nil {
				return err
			}
			if count == 0 {
				if _, err := db.Exec("INSERT INTO casbin_rule (p_type,v0,v1,v2,v3,v4,v5) VALUES ('p',?,?,'GET','','','')", role, permissionPath); err != nil {
					return err
				}
			}
		}
	}
	for _, role := range []string{"sysadmin", "admin"} {
		var resetPermissionCount int
		if err := db.QueryRow("SELECT COUNT(1) FROM casbin_rule WHERE p_type='p' AND v0=? AND v1='/api/nodeServer/resetNodeServerTraffic' AND v2='POST'", role).Scan(&resetPermissionCount); err != nil {
			return err
		}
		if resetPermissionCount == 0 {
			if _, err := db.Exec("INSERT INTO casbin_rule (p_type,v0,v1,v2,v3,v4,v5) VALUES ('p',?,'/api/nodeServer/resetNodeServerTraffic','POST','','','')", role); err != nil {
				return err
			}
		}
	}
	return nil
}

func migrateKernelUpgradeSchema() error {
	migrations := []string{
		"ALTER TABLE `node_server` ADD COLUMN `grpc_tls_mode` varchar(16) NOT NULL DEFAULT 'legacy' COMMENT 'legacy/mtls' AFTER `grpc_port`",
		"ALTER TABLE `node_server` ADD COLUMN `grpc_tls_server_name` varchar(253) NOT NULL DEFAULT '' COMMENT 'gRPC TLS certificate name' AFTER `grpc_tls_mode`",
		`CREATE TABLE IF NOT EXISTS kernel_release_cache (
			kernel_name varchar(32) NOT NULL,
			channel_name varchar(16) NOT NULL,
			payload longtext NOT NULL,
			etag varchar(255) NOT NULL DEFAULT '',
			fetched_at datetime NOT NULL,
			error_message varchar(1024) NOT NULL DEFAULT '',
			PRIMARY KEY (kernel_name, channel_name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS kernel_upgrade_task (
			id bigint unsigned NOT NULL AUTO_INCREMENT,
			operator_id bigint unsigned NOT NULL,
			operator_name varchar(64) NOT NULL,
			canary_node_id bigint unsigned NOT NULL DEFAULT 0,
			status varchar(16) NOT NULL DEFAULT 'queued',
			create_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
			update_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			KEY idx_kernel_task_created (create_time)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS kernel_upgrade_task_item (
			id bigint unsigned NOT NULL AUTO_INCREMENT,
			task_id bigint unsigned NOT NULL,
			node_server_id bigint unsigned NOT NULL,
			node_server_name varchar(64) NOT NULL,
			kernel_name varchar(32) NOT NULL,
			from_version varchar(64) NOT NULL DEFAULT '',
			target_version varchar(64) NOT NULL,
			channel_name varchar(16) NOT NULL,
			action_name varchar(16) NOT NULL DEFAULT 'install',
			sha256 char(64) NOT NULL DEFAULT '',
			stage varchar(32) NOT NULL DEFAULT 'queued',
			result varchar(16) NOT NULL DEFAULT '',
			error_message varchar(2048) NOT NULL DEFAULT '',
			rollback_result varchar(32) NOT NULL DEFAULT '',
			core_operation_id varchar(64) NOT NULL DEFAULT '',
			idempotency_key varchar(128) NOT NULL,
			attempt int unsigned NOT NULL DEFAULT 1,
			create_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
			update_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uk_kernel_item_idempotency (idempotency_key),
			KEY idx_kernel_item_task (task_id),
			KEY idx_kernel_item_node_stage (node_server_id, stage)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		"ALTER TABLE `kernel_upgrade_task_item` ADD COLUMN `action_name` varchar(16) NOT NULL DEFAULT 'install' AFTER `channel_name`",
	}
	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			if strings.Contains(err.Error(), "Duplicate column name") {
				continue
			}
			return err
		}
	}
	paths := map[string]string{
		"/api/kernel/releases":       "GET",
		"/api/kernel/inventory":      "GET",
		"/api/kernel/createTask":     "POST",
		"/api/kernel/selectTaskPage": "GET",
		"/api/kernel/selectTaskById": "GET",
		"/api/kernel/retryTask":      "POST",
		"/api/kernel/probeMTLS":      "POST",
	}
	for path, method := range paths {
		var count int
		if err := db.QueryRow("SELECT COUNT(1) FROM casbin_rule WHERE p_type='p' AND v0='sysadmin' AND v1=? AND v2=?", path, method).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			if _, err := db.Exec("INSERT INTO casbin_rule (p_type,v0,v1,v2,v3,v4,v5) VALUES ('p','sysadmin',?,?,'','','')", path, method); err != nil {
				return err
			}
		}
	}
	return nil
}

func migrateClientExportPermissions() error {
	paths := []string{
		"/api/account/exportOptions",
		"/api/account/exportSubscribe",
		"/api/account/exportQRCode",
	}
	for _, role := range []string{"sysadmin", "admin", "user"} {
		for _, path := range paths {
			var count int
			if err := db.QueryRow("SELECT COUNT(1) FROM `casbin_rule` WHERE `p_type` = 'p' AND `v0` = ? AND `v1` = ? AND `v2` = 'GET'", role, path).
				Scan(&count); err != nil {
				return err
			}
			if count > 0 {
				continue
			}
			if _, err := db.Exec("INSERT INTO `casbin_rule` (`p_type`, `v0`, `v1`, `v2`, `v3`, `v4`, `v5`) VALUES ('p', ?, ?, 'GET', '', '', '')", role, path); err != nil {
				return err
			}
		}
	}
	return nil
}

func migrateNodeClientTypesColumn() error {
	_, err := db.Exec("ALTER TABLE `node` ADD COLUMN `client_types` varchar(64) NOT NULL DEFAULT 'sing-box,clash-meta,v2ray,shadowrocket' COMMENT '订阅适用客户端' AFTER `priority`")
	if err != nil && !strings.Contains(err.Error(), "Duplicate column name") {
		return err
	}
	// Preserve customized client selections. Only records still carrying the
	// former complete default gain the newly supported client automatically.
	if _, err = db.Exec("UPDATE `node` SET `client_types` = 'sing-box,clash-meta,v2ray,shadowrocket' WHERE `client_types` = 'sing-box,clash-meta,v2ray'"); err != nil {
		return err
	}
	_, err = db.Exec("ALTER TABLE `node` ALTER `client_types` SET DEFAULT 'sing-box,clash-meta,v2ray,shadowrocket'")
	return err
}

func migrateNodeUotColumns() error {
	migrations := []string{
		"ALTER TABLE `node` ADD COLUMN `naive_uot_enable` tinyint(1) unsigned NOT NULL DEFAULT '0' COMMENT 'NaiveProxy是否启用UoT 0/否 1/是' AFTER `priority`",
		"ALTER TABLE `node` ADD COLUMN `naive_uot_version` tinyint(1) unsigned NOT NULL DEFAULT '2' COMMENT 'NaiveProxy UoT版本 1/2' AFTER `naive_uot_enable`",
	}
	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			if strings.Contains(err.Error(), "Duplicate column name") {
				continue
			}
			return err
		}
	}
	return nil
}

func migrateNodeXrayClientOptionColumns() error {
	migrations := []string{
		"ALTER TABLE `node_xray` ADD COLUMN `uot_enable` tinyint(1) unsigned NOT NULL DEFAULT '0' COMMENT 'Shadowsocks 2022客户端UoT 0/否 1/是' AFTER `xray_ss_method`",
		"ALTER TABLE `node_xray` ADD COLUMN `uot_version` tinyint(1) unsigned NOT NULL DEFAULT '2' COMMENT 'Shadowsocks 2022客户端UoT版本 1/2' AFTER `uot_enable`",
		"ALTER TABLE `node_xray` ADD COLUMN `xudp_enable` tinyint(1) unsigned NOT NULL DEFAULT '0' COMMENT 'VLESS/VMess客户端XUDP 0/否 1/是' AFTER `uot_version`",
		"ALTER TABLE `node_xray` ADD COLUMN `mux_enable` tinyint(1) unsigned NOT NULL DEFAULT '0' COMMENT 'VLESS/VMess/Trojan客户端复用 0/否 1/是' AFTER `xudp_enable`",
	}
	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil && !strings.Contains(err.Error(), "Duplicate column name") {
			return err
		}
	}
	return nil
}

func migrateHysteria2PortHoppingColumns() error {
	migrations := []string{
		"ALTER TABLE `node_hysteria2` ADD COLUMN `port_hopping` varchar(128) NOT NULL DEFAULT '' COMMENT 'Hysteria2客户端端口跳跃范围' AFTER `insecure`",
		"ALTER TABLE `node_hysteria2` ADD COLUMN `hop_interval` int(10) unsigned NOT NULL DEFAULT '0' COMMENT 'Hysteria2客户端端口跳跃间隔秒数' AFTER `port_hopping`",
	}
	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			if strings.Contains(err.Error(), "Duplicate column name") {
				continue
			}
			return err
		}
	}
	return nil
}

func CloseDb() {
	if db != nil {
		if err := db.Close(); err != nil {
			logrus.Errorf("db close err: %v", err)
		}
	}
}

func SqlInit(sqlStr string) error {
	sqls := strings.Split(strings.Replace(sqlStr, "\r\n", "\n", -1), ";\n")
	for _, s := range sqls {
		s = strings.TrimSpace(s)
		if s != "" {
			if _, err := db.Exec(s); err != nil {
				logrus.Errorf("sql execution err: %v", err)
				return err
			}
		}
	}
	return nil
}

var sqlInitStr = "CREATE DATABASE IF NOT EXISTS `trojan_panel_db` DEFAULT CHARACTER SET utf8mb4;\nUSE `trojan_panel_db`;\nDROP TABLE IF EXISTS `account`;\nCREATE TABLE `account` (\n  `id` bigint(10) unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',\n  `username` varchar(64) NOT NULL DEFAULT '' COMMENT '登录用户名',\n  `pass` varchar(64) NOT NULL DEFAULT '' COMMENT '登录密码',\n  `hash` varchar(64) NOT NULL DEFAULT '' COMMENT 'pass的hash',\n  `quota` bigint(20) NOT NULL DEFAULT '0' COMMENT '配额 单位/byte',\n  `download` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '下载 单位/byte',\n  `upload` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '上传 单位/byte',\n  `ip_limit` tinyint(2) unsigned NOT NULL DEFAULT '3' COMMENT '限制IP设备数',\n  `upload_speed_limit` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '上传限速 单位/byte',\n  `download_speed_limit` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '下载限速 单位/byte',\n  `role_id` bigint(20) unsigned NOT NULL DEFAULT '3' COMMENT '角色id 1/系统管理员 3/普通用户',\n  `email` varchar(64) NOT NULL DEFAULT '' COMMENT '邮箱',\n  `preset_expire` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '预设过期时长',\n  `preset_quota` bigint(20) NOT NULL DEFAULT '0' COMMENT '预设配额',\n  `last_login_time` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '最后一次登录时间',\n  `expire_time` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '过期时间',\n  `deleted` tinyint(1) unsigned NOT NULL DEFAULT '0' COMMENT '是否禁用 0/正常 1/禁用',\n  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',\n  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',\n  PRIMARY KEY (`id`)\n) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb4 COMMENT='账户';\nLOCK TABLES `account` WRITE;\nINSERT INTO `account` VALUES (1,'sysadmin','tFjD2X1F6i9FfWp2GDU5Vbi1conuaChDKIYbw9zMFrqvMoSz','4366294571b8b267d9cf15b56660f0a70659568a86fc270a52fdc9e5',-1,0,0,3,0,0,1,'',0,0,0,4078656000000,0,'2022-04-01 00:00:00','2022-04-01 00:00:00');\nUNLOCK TABLES;\nDROP TABLE IF EXISTS `black_list`;\nCREATE TABLE `black_list` (\n  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',\n  `ip` varchar(64) NOT NULL DEFAULT '' COMMENT 'IP地址',\n  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',\n  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',\n  PRIMARY KEY (`id`)\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='黑名单';\nLOCK TABLES `black_list` WRITE;\nUNLOCK TABLES;\nDROP TABLE IF EXISTS `casbin_rule`;\nCREATE TABLE `casbin_rule` (\n  `p_type` varchar(32) NOT NULL DEFAULT '',\n  `v0` varchar(255) NOT NULL DEFAULT '',\n  `v1` varchar(255) NOT NULL DEFAULT '',\n  `v2` varchar(255) NOT NULL DEFAULT '',\n  `v3` varchar(255) NOT NULL DEFAULT '',\n  `v4` varchar(255) NOT NULL DEFAULT '',\n  `v5` varchar(255) NOT NULL DEFAULT '',\n  KEY `idx_casbin_rule` (`p_type`,`v0`,`v1`)\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;\nLOCK TABLES `casbin_rule` WRITE;\nINSERT INTO `casbin_rule` VALUES ('p','sysadmin','/api/account/selectAccountById','GET','','',''),('p','sysadmin','/api/account/createAccount','POST','','',''),('p','sysadmin','/api/account/getAccountInfo','GET','','',''),('p','sysadmin','/api/account/selectAccountPage','GET','','',''),('p','sysadmin','/api/account/deleteAccountById','POST','','',''),('p','sysadmin','/api/account/updateAccountPass','POST','','',''),('p','sysadmin','/api/account/updateAccountProperty','POST','','',''),('p','sysadmin','/api/account/updateAccountById','POST','','',''),('p','sysadmin','/api/account/logout','POST','','',''),('p','sysadmin','/api/account/clashSubscribe','GET','','',''),('p','sysadmin','/api/account/clashSubscribeForSb','GET','','',''),('p','sysadmin','/api/account/resetAccountDownloadAndUpload','POST','','',''),('p','sysadmin','/api/account/exportAccount','POST','','',''),('p','sysadmin','/api/account/importAccount','POST','','',''),('p','sysadmin','/api/account/createAccountBatch','POST','','',''),('p','sysadmin','/api/account/exportAccountUnused','POST','','',''),('p','sysadmin','/api/role/selectRoleList','GET','','',''),('p','sysadmin','/api/node/selectNodeById','GET','','',''),('p','sysadmin','/api/node/selectNodeInfo','GET','','',''),('p','sysadmin','/api/node/createNode','POST','','',''),('p','sysadmin','/api/node/selectNodePage','GET','','',''),('p','sysadmin','/api/node/deleteNodeById','POST','','',''),('p','sysadmin','/api/node/updateNodeById','POST','','',''),('p','sysadmin','/api/node/nodeQRCode','POST','','',''),('p','sysadmin','/api/node/nodeURL','POST','','',''),('p','sysadmin','/api/nodeType/selectNodeTypeList','GET','','',''),('p','sysadmin','/api/node/nodeDefault','GET','','',''),('p','sysadmin','/api/dashboard/panelGroup','GET','','',''),('p','sysadmin','/api/dashboard/trafficRank','GET','','',''),('p','sysadmin','/api/system/selectSystemByName','GET','','',''),('p','sysadmin','/api/system/updateSystemById','POST','','',''),('p','sysadmin','/api/system/uploadWebFile','POST','','',''),('p','sysadmin','/api/system/uploadLogo','POST','','',''),('p','sysadmin','/api/blackList/selectBlackListPage','GET','','',''),('p','sysadmin','/api/blackList/deleteBlackListByIp','POST','','',''),('p','sysadmin','/api/blackList/createBlackList','POST','','',''),('p','sysadmin','/api/emailRecord/selectEmailRecordPage','GET','','',''),('p','sysadmin','/api/nodeServer/selectNodeServerById','GET','','',''),('p','sysadmin','/api/nodeServer/createNodeServer','POST','','',''),('p','sysadmin','/api/nodeServer/selectNodeServerPage','GET','','',''),('p','sysadmin','/api/nodeServer/deleteNodeServerById','POST','','',''),('p','sysadmin','/api/nodeServer/updateNodeServerById','POST','','',''),('p','sysadmin','/api/nodeServer/selectNodeServerList','GET','','',''),('p','sysadmin','/api/nodeServer/nodeServerState','GET','','',''),('p','sysadmin','/api/nodeServer/exportNodeServer','POST','','',''),('p','sysadmin','/api/nodeServer/importNodeServer','POST','','',''),('p','sysadmin','/api/fileTask/selectFileTaskPage','GET','','',''),('p','sysadmin','/api/fileTask/deleteFileTaskById','POST','','',''),('p','sysadmin','/api/fileTask/downloadFileTask','POST','','',''),('p','sysadmin','/api/fileTask/downloadTemplate','POST','','',''),('p','user','/api/account/getAccountInfo','GET','','',''),('p','user','/api/account/updateAccountPass','POST','','',''),('p','user','/api/account/updateAccountProperty','POST','','',''),('p','user','/api/account/logout','POST','','',''),('p','user','/api/account/clashSubscribe','GET','','',''),('p','user','/api/node/selectNodeInfo','GET','','',''),('p','user','/api/node/selectNodePage','GET','','',''),('p','user','/api/node/nodeQRCode','POST','','',''),('p','user','/api/node/nodeURL','POST','','',''),('p','user','/api/nodeType/selectNodeTypeList','GET','','',''),('p','user','/api/node/nodeDefault','GET','','',''),('p','user','/api/dashboard/panelGroup','GET','','',''),('p','user','/api/dashboard/trafficRank','GET','','',''),('p','user','/api/nodeServer/selectNodeServerList','GET','','',''),('p','user','/api/nodeServer/nodeServerState','GET','','','');\nUNLOCK TABLES;\nDROP TABLE IF EXISTS `email_record`;\nCREATE TABLE `email_record` (\n  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',\n  `to_email` varchar(64) NOT NULL DEFAULT '' COMMENT '收件人邮箱',\n  `subject` varchar(64) NOT NULL DEFAULT '' COMMENT '主题',\n  `content` varchar(255) NOT NULL DEFAULT '' COMMENT '内容',\n  `state` tinyint(1) unsigned NOT NULL DEFAULT '0' COMMENT '状态 0/未发送 1/发送成功 -1/发送失败',\n  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',\n  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',\n  PRIMARY KEY (`id`)\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='邮件发送记录';\nLOCK TABLES `email_record` WRITE;\nUNLOCK TABLES;\nDROP TABLE IF EXISTS `file_task`;\nCREATE TABLE `file_task` (\n  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '自增主键',\n  `name` varchar(64) NOT NULL DEFAULT '' COMMENT '文件名称',\n  `path` varchar(128) NOT NULL DEFAULT '' COMMENT '文件路径',\n  `type` tinyint(2) unsigned NOT NULL DEFAULT '1' COMMENT '类型 1/用户导入 2/服务器导入 3/用户导出 4/服务器导出',\n  `status` tinyint(1) NOT NULL DEFAULT '0' COMMENT '状态 -1/失败 0/等待 1/正在执行 2/成功',\n  `err_msg` varchar(128) NOT NULL DEFAULT '' COMMENT '错误信息',\n  `account_id` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '账户id',\n  `account_username` varchar(64) NOT NULL DEFAULT '' COMMENT '账户登录用户名',\n  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',\n  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',\n  PRIMARY KEY (`id`)\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='文件任务';\nLOCK TABLES `file_task` WRITE;\nUNLOCK TABLES;\nDROP TABLE IF EXISTS `node`;\nCREATE TABLE `node` (\n  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',\n  `node_server_id` bigint(20) NOT NULL DEFAULT '0' COMMENT '服务器id',\n  `node_sub_id` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '节点分表id',\n  `node_type_id` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '节点类型id',\n  `name` varchar(64) NOT NULL DEFAULT '' COMMENT '名称',\n  `node_server_ip` varchar(64) NOT NULL DEFAULT '' COMMENT 'IP地址',\n  `node_server_grpc_port` int(10) unsigned NOT NULL DEFAULT '8100' COMMENT 'gRPC端口',\n  `domain` varchar(64) NOT NULL DEFAULT '' COMMENT '域名',\n  `port` int(10) unsigned NOT NULL DEFAULT '443' COMMENT '端口',\n  `priority` int(11) NOT NULL DEFAULT '100' COMMENT '优先级',\n  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',\n  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',\n  PRIMARY KEY (`id`)\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='节点';\nLOCK TABLES `node` WRITE;\nUNLOCK TABLES;\nDROP TABLE IF EXISTS `node_hysteria`;\nCREATE TABLE `node_hysteria` (\n  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',\n  `protocol` varchar(32) NOT NULL DEFAULT 'udp' COMMENT '协议名称 udp/faketcp',\n  `obfs` varchar(64) NOT NULL DEFAULT '' COMMENT '混淆密码',\n  `up_mbps` int(10) NOT NULL DEFAULT '100' COMMENT '单客户端最大上传速度 单位:Mbps',\n  `down_mbps` int(10) NOT NULL DEFAULT '100' COMMENT '单客户端最大下载速度 单位:Mbps',\n  `server_name` varchar(64) NOT NULL DEFAULT '' COMMENT '用于验证服务端证书的 hostname',\n  `insecure` tinyint(1) NOT NULL DEFAULT 0 COMMENT '忽略一切证书错误',\n  `fast_open` tinyint(1) NOT NULL DEFAULT 0 COMMENT '启用 Fast Open (降低连接建立延迟)',\n  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',\n  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',\n  PRIMARY KEY (`id`)\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Hysteria节点';\nLOCK TABLES `node_hysteria` WRITE;\nUNLOCK TABLES;\nDROP TABLE IF EXISTS `node_hysteria2`;\nCREATE TABLE `node_hysteria2` (\n  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',\n  `obfs_password` varchar(64) NOT NULL DEFAULT '' COMMENT '混淆密码',\n  `up_mbps` int(10) NOT NULL DEFAULT '100' COMMENT '单客户端最大上传速度 单位:Mbps',\n  `down_mbps` int(10) NOT NULL DEFAULT '100' COMMENT '单客户端最大下载速度 单位:Mbps',\n  `server_name` varchar(64) NOT NULL DEFAULT '' COMMENT '用于验证服务端证书的 hostname',\n  `insecure` tinyint(1) NOT NULL DEFAULT '0' COMMENT '忽略一切证书错误',\n  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',\n  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',\n  PRIMARY KEY (`id`)\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Hysteria2节点';\nLOCK TABLES `node_hysteria2` WRITE;\nUNLOCK TABLES;\nDROP TABLE IF EXISTS `node_server`;\nCREATE TABLE `node_server` (\n  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '自增主键',\n  `ip` varchar(64) NOT NULL DEFAULT '' COMMENT '服务器IP',\n  `name` varchar(64) NOT NULL DEFAULT '' COMMENT '服务器名称',\n  `grpc_port` int(10) unsigned NOT NULL DEFAULT '8100' COMMENT 'gRPC端口',\n  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',\n  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',\n  PRIMARY KEY (`id`)\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='服务器';\nLOCK TABLES `node_server` WRITE;\nUNLOCK TABLES;\nDROP TABLE IF EXISTS `node_trojan_go`;\nCREATE TABLE `node_trojan_go` (\n  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',\n  `sni` varchar(64) NOT NULL DEFAULT '' COMMENT 'sni',\n  `mux_enable` tinyint(1) unsigned NOT NULL DEFAULT '1' COMMENT '是否开启多路复用 0/关闭 1/开启',\n  `websocket_enable` tinyint(1) unsigned NOT NULL DEFAULT '0' COMMENT '是否开启websocket 0/否 1/是',\n  `websocket_path` varchar(64) NOT NULL DEFAULT 'trojan-panel-websocket-path' COMMENT 'websocket路径',\n  `websocket_host` varchar(64) NOT NULL DEFAULT '' COMMENT 'websocket host',\n  `ss_enable` tinyint(1) unsigned NOT NULL DEFAULT '0' COMMENT '是否开启ss加密 0/否 1/是',\n  `ss_method` varchar(32) NOT NULL DEFAULT 'AES-128-GCM' COMMENT 'ss加密方式',\n  `ss_password` varchar(64) NOT NULL DEFAULT '' COMMENT 'ss密码',\n  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',\n  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',\n  PRIMARY KEY (`id`)\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='TrojanGO节点';\nLOCK TABLES `node_trojan_go` WRITE;\nUNLOCK TABLES;\nDROP TABLE IF EXISTS `node_type`;\nCREATE TABLE `node_type` (\n  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',\n  `name` varchar(32) NOT NULL DEFAULT '' COMMENT '名称',\n  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',\n  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',\n  PRIMARY KEY (`id`)\n) ENGINE=InnoDB AUTO_INCREMENT=6 DEFAULT CHARSET=utf8mb4 COMMENT='节点类型';\nLOCK TABLES `node_type` WRITE;\nINSERT INTO `node_type` VALUES (1,'xray','2022-04-01 00:00:00','2022-04-01 00:00:00'),(2,'trojan-go','2022-04-01 00:00:00','2022-04-01 00:00:00'),(3,'hysteria','2022-04-01 00:00:00','2022-04-01 00:00:00'),(4,'naiveproxy','2022-04-01 00:00:00','2022-04-01 00:00:00'),(5,'hysteria2','2022-04-01 00:00:00','2022-04-01 00:00:00');\nUNLOCK TABLES;\nDROP TABLE IF EXISTS `node_xray`;\nCREATE TABLE `node_xray` (\n  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',\n  `protocol` varchar(32) NOT NULL DEFAULT '' COMMENT '协议名称',\n  `xray_flow` varchar(32) NOT NULL DEFAULT '' COMMENT 'Xray流控',\n  `xray_ss_method` varchar(32) NOT NULL DEFAULT 'aes-256-gcm' COMMENT 'Xray Shadowsocks加密方式',\n  `reality_pbk` varchar(64) NOT NULL DEFAULT '' COMMENT 'reality的公钥',\n  `settings` varchar(1024) NOT NULL DEFAULT '' COMMENT 'settings',\n  `stream_settings` varchar(1024) NOT NULL DEFAULT '' COMMENT 'streamSettings',\n  `tag` varchar(64) NOT NULL DEFAULT '' COMMENT 'tag',\n  `sniffing` varchar(256) NOT NULL DEFAULT '' COMMENT 'sniffing',\n  `allocate` varchar(256) NOT NULL DEFAULT '' COMMENT 'allocate',\n  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',\n  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',\n  PRIMARY KEY (`id`)\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Xray节点';\nLOCK TABLES `node_xray` WRITE;\nUNLOCK TABLES;\nDROP TABLE IF EXISTS `role`;\nCREATE TABLE `role` (\n  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',\n  `name` varchar(16) NOT NULL DEFAULT '' COMMENT '名称',\n  `desc` varchar(16) NOT NULL DEFAULT '' COMMENT '描述',\n  `parent_id` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '父级id',\n  `path` varchar(128) NOT NULL DEFAULT '' COMMENT '路径',\n  `level` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '等级',\n  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',\n  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',\n  PRIMARY KEY (`id`),\n  KEY `role_name_index` (`name`)\n) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8mb4 COMMENT='角色';\nLOCK TABLES `role` WRITE;\nINSERT INTO `role` VALUES (1,'sysadmin','System Admin',0,'',1,'2022-04-01 00:00:00','2022-04-01 00:00:00'),(2,'admin','Admin',1,'1-',2,'2022-04-01 00:00:00','2022-04-01 00:00:00'),(3,'user','User',2,'1-2-',3,'2022-04-01 00:00:00','2022-04-01 00:00:00');\nUNLOCK TABLES;\nDROP TABLE IF EXISTS `system`;\nCREATE TABLE `system` (\n  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',\n  `name` varchar(16) NOT NULL DEFAULT '' COMMENT '系统名称',\n  `account_config` varchar(512) NOT NULL DEFAULT '' COMMENT '用户设置',\n  `email_config` varchar(512) NOT NULL DEFAULT '' COMMENT '系统邮箱设置',\n  `template_config` varchar(512) NOT NULL DEFAULT '' COMMENT '模板设置',\n  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',\n  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',\n  PRIMARY KEY (`id`)\n) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb4 COMMENT='系统设置';\nLOCK TABLES `system` WRITE;\nINSERT INTO `system` VALUES (1,'trojan-panel','{\\\"registerEnable\\\":1,\\\"registerQuota\\\":0,\\\"registerExpireDays\\\":0,\\\"resetDownloadAndUploadMonth\\\":0,\\\"trafficRankEnable\\\":1,\\\"captchaEnable\\\":0}','{\\\"expireWarnEnable\\\":0,\\\"expireWarnDay\\\":0,\\\"emailEnable\\\":0,\\\"emailHost\\\":\\\"\\\",\\\"emailPort\\\":0,\\\"emailUsername\\\":\\\"\\\",\\\"emailPassword\\\":\\\"\\\"}','{\\\"systemName\\\":\\\"Trojan Panel\\\"}','2022-04-01 00:00:00','2022-04-01 00:00:00');\nUNLOCK TABLES;"
