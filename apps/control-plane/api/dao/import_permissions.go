package dao

func migrateImportTemplatePermissions() error {
	for _, path := range []string{"/api/account/downloadImportTemplate", "/api/nodeServer/downloadImportTemplate"} {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM casbin_rule WHERE p_type='p' AND v0='sysadmin' AND v1=? AND v2='POST'`, path).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			if _, err := db.Exec(`INSERT INTO casbin_rule (p_type,v0,v1,v2,v3,v4,v5) VALUES ('p','sysadmin',?,'POST','','','')`, path); err != nil {
				return err
			}
		}
	}
	return nil
}
