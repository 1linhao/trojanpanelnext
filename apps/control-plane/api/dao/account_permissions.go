package dao

func migrateAccountResetPermissions() error {
	for _, path := range []string{
		"/api/account/resetAccountLoginLimit",
		"/api/account/resetAllAccountDownloadAndUpload",
	} {
		if _, err := db.Exec(`INSERT INTO casbin_rule (p_type,v0,v1,v2,v3,v4,v5)
			SELECT 'p','sysadmin',?,'POST','','',''
			WHERE NOT EXISTS (
				SELECT 1 FROM casbin_rule WHERE p_type='p' AND v0='sysadmin' AND v1=? AND v2='POST'
			)`, path, path); err != nil {
			return err
		}
	}
	return nil
}
