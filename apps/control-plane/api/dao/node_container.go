package dao

// Only called after an authenticated host explicitly rejects /remove before
// starting its uninstall, and only if this attempt introduced the flag.
func CancelUnstartedNodeServerRemoval(id uint) error {
	_, err := db.Exec(`UPDATE node_server SET removing=0 WHERE id=? AND removing=1 AND NOT EXISTS (SELECT 1 FROM node_removal_cleanup WHERE node_server_id=?)`, id, id)
	return err
}

func NodeHasActiveKernelTask(id uint) (bool, error) {
	var count uint
	err := db.QueryRow(`SELECT COUNT(*) FROM kernel_upgrade_task_item WHERE node_server_id=? AND result=''`, id).Scan(&count)
	return count != 0, err
}

func migrateNodeContainerPermissions() error {
	for _, permission := range []struct{ path, method string }{
		{"/api/container/inventory", "GET"}, {"/api/container/update", "POST"},
	} {
		var count uint
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
