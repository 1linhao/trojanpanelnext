package dao

import (
	"context"
	"database/sql"
)

var nodeIdentitySchema = []string{
	`CREATE TABLE IF NOT EXISTS node_identity (
		identity_id char(36) NOT NULL,
		node_server_id bigint unsigned NOT NULL,
		name varchar(64) NOT NULL,
		domain varchar(253) NOT NULL,
		public_ip varchar(64) NOT NULL,
		generation bigint unsigned NOT NULL DEFAULT 1,
		mariadb_username varchar(32) NOT NULL,
		redis_username varchar(64) NOT NULL,
		redis_auth_username varchar(64) NOT NULL,
		credential_path varchar(1024) NOT NULL,
		credential_sha256 char(64) NOT NULL,
		status varchar(16) NOT NULL DEFAULT 'provisioning',
		create_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
		update_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		PRIMARY KEY (identity_id),
		UNIQUE KEY uk_node_identity_server (node_server_id),
		UNIQUE KEY uk_node_identity_name (name),
		UNIQUE KEY uk_node_identity_domain (domain)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`ALTER TABLE node_identity ADD COLUMN IF NOT EXISTS redis_auth_username varchar(64) NOT NULL DEFAULT '' AFTER redis_username`,
	`ALTER TABLE node_identity ADD COLUMN IF NOT EXISTS credential_sha256 char(64) NOT NULL DEFAULT '' AFTER credential_path`,
	`CREATE TABLE IF NOT EXISTS node_identity_event (
		id bigint unsigned NOT NULL AUTO_INCREMENT,
		identity_id char(36) NOT NULL,
		generation bigint unsigned NOT NULL,
		action varchar(16) NOT NULL,
		result varchar(16) NOT NULL,
		error_code varchar(64) NOT NULL DEFAULT '',
		create_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (id),
		KEY idx_node_identity_event (identity_id, create_time)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
}

func EnsureNodeIdentitySchema(ctx context.Context, database *sql.DB) error {
	for _, statement := range nodeIdentitySchema {
		if _, err := database.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func migrateNodeIdentitySchema() error {
	return EnsureNodeIdentitySchema(context.Background(), db)
}
