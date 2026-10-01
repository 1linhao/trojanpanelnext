package dao

func migrateNodeDeploymentPermissions() error {
	for _, permission := range []struct{ path, method string }{
		{"/api/nodeServer/deployment", "GET"},
		{"/api/nodeServer/downloadDeployment", "POST"},
	} {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM casbin_rule WHERE p_type='p' AND v0='sysadmin' AND v1=? AND v2=?`, permission.path, permission.method).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			if _, err := db.Exec(`INSERT INTO casbin_rule (p_type,v0,v1,v2,v3,v4,v5) VALUES ('p','sysadmin',?,?,'','','')`, permission.path, permission.method); err != nil {
				return err
			}
		}
	}
	return nil
}
