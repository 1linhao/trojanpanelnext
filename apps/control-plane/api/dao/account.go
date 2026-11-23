package dao

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/didi/gendry/builder"
	"github.com/didi/gendry/scanner"
	"github.com/sirupsen/logrus"
	"trojan-panel/model"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
	"trojan-panel/util"
)

func SelectAccountById(id *uint, includeRemark ...bool) (*model.Account, error) {
	var account model.Account

	where := map[string]interface{}{"id": *id}
	selectFields := []string{"id", "username", "role_id", "email", "preset_expire", "preset_quota", "expire_time", "deleted", accountPlanQuota,
		"download", "upload"}
	if len(includeRemark) > 0 && includeRemark[0] {
		selectFields = append(selectFields, "remark")
	}
	buildSelect, values, err := builder.BuildSelect("account", where, selectFields)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	rows, err := db.Query(buildSelect, values...)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	defer rows.Close()

	if err = scanner.Scan(rows, &account); err == scanner.ErrEmptyResult {
		return nil, errors.New(constant.UnauthorizedError)
	} else if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	return &account, nil
}

func CreateAccount(account *model.Account) error {
	// 密码加密
	encryPass := util.Sha1String(fmt.Sprintf("%s%s", *account.Username, *account.Pass))
	hash := util.SHA224String(encryPass)

	accountCreate := map[string]interface{}{
		"username": *account.Username,
		"`pass`":   encryPass,
		"`hash`":   hash,
	}
	if account.RoleId != nil {
		accountCreate["`role_id`"] = *account.RoleId
	}
	if account.Email != nil && *account.Email != "" {
		accountCreate["`email`"] = *account.Email
	}
	if account.PresetExpire != nil {
		accountCreate["preset_expire"] = *account.PresetExpire
	}
	if account.PresetQuota != nil {
		accountCreate["preset_quota"] = *account.PresetQuota
	}
	if account.LastLoginTime != nil {
		accountCreate["last_login_time"] = *account.LastLoginTime
	}
	if account.ExpireTime != nil {
		accountCreate["expire_time"] = *account.ExpireTime
	}
	if account.Deleted != nil {
		accountCreate["deleted"] = *account.Deleted
	}
	if account.Quota != nil {
		setAccountQuota(accountCreate, *account.Quota, account.Deleted != nil && *account.Deleted != 0)
	}
	if account.IpLimit != nil {
		accountCreate["ip_limit"] = *account.IpLimit
	}
	if account.UploadSpeedLimit != nil {
		accountCreate["upload_speed_limit"] = *account.UploadSpeedLimit
	}
	if account.DownloadSpeedLimit != nil {
		accountCreate["download_speed_limit"] = *account.DownloadSpeedLimit
	}
	var data []map[string]interface{}
	clearQuotaIfPastExpiry(accountCreate, account.ExpireTime)
	data = append(data, accountCreate)

	buildInsert, values, err := builder.BuildInsert("account", data)
	if err != nil {
		logrus.Errorln(err.Error())
		return errors.New(constant.SysError)
	}
	if _, err = db.Exec(buildInsert, values...); err != nil {
		logrus.Errorln(err.Error())
		return errors.New(constant.SysError)
	}
	return nil
}

func CountAccountByUsername(username *string) (int, error) {
	var count int

	where := map[string]interface{}{}
	if username != nil {
		where["username"] = *username
	}

	selectFields := []string{"count(1)"}
	buildSelect, values, err := builder.BuildSelect("account", where, selectFields)
	if err != nil {
		logrus.Errorln(err.Error())
		return 0, errors.New(constant.SysError)
	}

	if err = db.QueryRow(buildSelect, values...).Scan(&count); err != nil {
		logrus.Errorln(err.Error())
		return 0, errors.New(constant.SysError)
	}
	return count, nil
}

func SelectAccountPage(
	queryUsername *string,
	deleted *uint,
	lastLoginTime *uint,
	orderFields *string,
	orderBy *string,
	pageNum *uint,
	pageSize *uint,
	includeRemark ...bool) (*vo.AccountPageVo, error) {
	var (
		total    uint
		accounts []model.Account
	)

	// 查询总数
	var whereCount = map[string]interface{}{}
	if queryUsername != nil && *queryUsername != "" {
		whereCount["username like"] = fmt.Sprintf("%%%s%%", *queryUsername)
	}
	if deleted != nil {
		whereCount["deleted"] = *deleted
	}
	if lastLoginTime != nil {
		if *lastLoginTime == 0 {
			whereCount["last_login_time"] = 0
		} else {
			whereCount["last_login_time <>"] = 0
		}
	}
	selectFieldsCount := []string{"count(1)"}
	buildSelect, values, err := builder.BuildSelect("account", whereCount, selectFieldsCount)
	if err = db.QueryRow(buildSelect, values...).Scan(&total); err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	// 分页查询
	where := map[string]interface{}{
		"_limit": []uint{(*pageNum - 1) * *pageSize, *pageSize}}
	if queryUsername != nil && *queryUsername != "" {
		where["username like"] = fmt.Sprintf("%%%s%%", *queryUsername)
	}
	if deleted != nil {
		where["deleted"] = *deleted
	}
	if lastLoginTime != nil {
		if *lastLoginTime == 0 {
			where["last_login_time"] = 0
		} else {
			where["last_login_time <>"] = 0
		}
	}
	if orderFields != nil && *orderFields != "" {
		orderByStr := *orderFields
		if orderBy != nil && *orderBy != "" {
			orderByStr = fmt.Sprintf("%s %s", orderByStr, *orderBy)
		}
		where["_orderby"] = orderByStr
	}
	selectFields := []string{"id", "username", "role_id", "email", "preset_expire", "preset_quota", "last_login_time", "expire_time", "deleted",
		accountPlanQuota, "upload", "download", "create_time"}
	if len(includeRemark) > 0 && includeRemark[0] {
		selectFields = append(selectFields, "remark")
	}
	selectSQL, values, err := builder.BuildSelect("account", where, selectFields)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	rows, err := db.Query(selectSQL, values...)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	defer rows.Close()

	if err = scanner.Scan(rows, &accounts); err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	var accountVos = make([]vo.AccountVo, 0)
	for _, item := range accounts {
		accountVos = append(accountVos, vo.AccountVo{
			Id:            *item.Id,
			Username:      *item.Username,
			RoleId:        *item.RoleId,
			Email:         *item.Email,
			Remark:        item.Remark,
			PresetExpire:  *item.PresetExpire,
			PresetQuota:   *item.PresetQuota,
			LastLoginTime: *item.LastLoginTime,
			ExpireTime:    *item.ExpireTime,
			Deleted:       *item.Deleted,
			Quota:         *item.Quota,
			Download:      *item.Download,
			Upload:        *item.Upload,
			CreateTime:    *item.CreateTime,
		})
	}

	accountPageVo := vo.AccountPageVo{
		BaseVoPage: vo.BaseVoPage{
			PageNum:  *pageNum,
			PageSize: *pageSize,
			Total:    total,
		},
		AccountVos: accountVos,
	}
	return &accountPageVo, nil
}

func DeleteAccountById(id *uint) error {
	buildDelete, values, err := builder.BuildDelete("account", map[string]interface{}{"id": *id})
	if err != nil {
		logrus.Errorln(err.Error())
		return errors.New(constant.SysError)
	}

	if _, err = db.Exec(buildDelete, values...); err != nil {
		logrus.Errorln(err.Error())
		return errors.New(constant.SysError)
	}
	return nil
}

func SelectAccountByUsername(username *string) (*model.Account, error) {
	return selectAccountIdentity(map[string]interface{}{"username": *username})
}

func SelectAccountIdentity(id uint) (*model.Account, error) {
	return selectAccountIdentity(map[string]interface{}{"id": id})
}

func selectAccountIdentity(where map[string]interface{}) (*model.Account, error) {
	var account model.Account
	selectFields := []string{"id", "username", "pass", "login_verifier", "role_id", "preset_expire", "preset_quota", "last_login_time", "deleted", "session_version"}
	buildSelect, values, err := builder.BuildSelect("account", where, selectFields)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	rows, err := db.Query(buildSelect, values...)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	defer rows.Close()

	if err = scanner.Scan(rows, &account); err == scanner.ErrEmptyResult {
		return nil, errors.New(constant.UsernameOrPassError)
	} else if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	return &account, nil
}

func UpdateAccountPass(account *model.Account, newPass string) error {
	verifier := util.Sha1String(*account.Username + newPass)
	return updateAccountIdentity(account, map[string]interface{}{"pass": verifier, "hash": util.SHA224String(verifier), "login_verifier": ""})
}

func UpdateAccountProperty(account *model.Account, pass, username, email *string) error {
	update := map[string]interface{}{}
	if username != nil && *username != "" && *username != *account.Username {
		update["username"] = *username
		verifier := util.Sha1String(*username + *pass)
		update["login_verifier"] = ""
		update["pass"] = verifier
		update["hash"] = util.SHA224String(verifier)
	}
	if email != nil && *email != "" {
		update["email"] = *email
	}
	if len(update) == 0 {
		return nil
	}
	return updateAccountIdentity(account, update)
}

// Keep a request bound to the signed account ID and the verifier checked by the
// service, including when a concurrent rename or password change wins the race.
func updateAccountIdentity(account *model.Account, update map[string]interface{}) error {
	where := map[string]interface{}{"id": *account.Id, "pass": *account.Pass}
	query, args, err := builder.BuildUpdate("account", where, update)
	if err != nil {
		return err
	}
	result, err := db.Exec(query, args...)
	if err != nil {
		return errors.New(constant.SysError)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		var matches int
		if err = db.QueryRow("SELECT COUNT(*) FROM account WHERE id=? AND pass=?", *account.Id, *account.Pass).Scan(&matches); err != nil {
			return err
		}
		if matches == 0 {
			return errors.New(constant.OriPassError)
		}
	}
	return nil
}

func UpdateAccountById(account *model.Account) error {
	update := map[string]interface{}{}
	if account.Remark != nil {
		update["remark"] = *account.Remark
	}
	if account.Pass != nil && *account.Pass != "" {
		sha1String := util.Sha1String(fmt.Sprintf("%s%s", *account.Username, *account.Pass))
		update["login_verifier"] = ""
		update["pass"] = sha1String
		update["hash"] = util.SHA224String(sha1String)
	}
	if account.RoleId != nil {
		update["role_id"] = *account.RoleId
	}
	if account.Email != nil {
		update["email"] = *account.Email
	}
	if account.PresetExpire != nil && *account.PresetExpire != 0 {
		update["preset_expire"] = *account.PresetExpire
	}
	if account.PresetQuota != nil && *account.PresetQuota != 0 {
		update["preset_quota"] = *account.PresetQuota
	}
	if account.LastLoginTime != nil && *account.LastLoginTime != 0 {
		update["last_login_time"] = *account.LastLoginTime
	}
	if account.ExpireTime != nil {
		update["expire_time"] = *account.ExpireTime
	}
	if account.IpLimit != nil {
		update["ip_limit"] = *account.IpLimit
	}
	if account.UploadSpeedLimit != nil {
		update["upload_speed_limit"] = *account.UploadSpeedLimit
	}
	if account.DownloadSpeedLimit != nil {
		update["download_speed_limit"] = *account.DownloadSpeedLimit
	}

	if err := updateAccountFields(account, update, map[string]interface{}{"id": *account.Id}); err != nil {
		logrus.Error(err)
		return errors.New(constant.SysError)
	}
	return nil
}

func SelectConnectPassword(id *uint, username *string) (string, error) {
	var account model.Account
	where := map[string]interface{}{}
	if id != nil && *id != 0 {
		where["id"] = *id
	}
	if username != nil && *username != "" {
		where["username"] = *username
	}
	if len(where) > 0 {
		selectFields := []string{"username", "pass"}
		buildSelect, values, err := builder.BuildSelect("account", where, selectFields)
		if err != nil {
			logrus.Errorln(err.Error())
			return "", errors.New(constant.SysError)
		}

		rows, err := db.Query(buildSelect, values...)
		if err != nil {
			logrus.Errorln(err.Error())
			return "", errors.New(constant.SysError)
		}
		defer rows.Close()

		if err = scanner.Scan(rows, &account); err == scanner.ErrEmptyResult {
			return "", errors.New(constant.UnauthorizedError)
		} else if err != nil {
			logrus.Errorln(err.Error())
			return "", errors.New(constant.SysError)
		}

		if account.Username == nil || *account.Username == "" || account.Pass == nil || *account.Pass == "" {
			return "", errors.New(constant.SysError)
		}
		return *account.Pass, nil
	}
	return "", errors.New(constant.SysError)
}

// SelectAccountsByExpireTime 用于发邮件
func SelectAccountsByExpireTime(expireTime uint) ([]model.Account, error) {
	buildSelect, values, err := builder.NamedQuery("select username,email from account where (quota < 0 or quota > download + upload) and expire_time <= {{expire_time}}",
		map[string]interface{}{"expire_time": expireTime})
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	rows, err := db.Query(buildSelect, values...)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	defer rows.Close()

	var accounts []model.Account
	if err = scanner.Scan(rows, &accounts); err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	return accounts, nil
}

// ResetAccountDownloadAndUpload clears the counters used for account quotas.
// The single UPDATE serializes with the node agent's row-locked increments.
// Lifetime and daily traffic ledgers are cumulative across quota resets.
func ResetAccountDownloadAndUpload(id *uint, roleIds *[]uint) error {
	where := map[string]interface{}{
		"quota <>": 0,
	}
	if id != nil {
		where["id"] = *id
	}
	if roleIds != nil && len(*roleIds) > 0 {
		where["role_id in"] = *roleIds
	}
	update := map[string]interface{}{"download": 0, "upload": 0}
	buildUpdate, values, err := builder.BuildUpdate("account", where, update)
	if err != nil {
		logrus.Errorln(err.Error())
		return errors.New(constant.SysError)
	}

	if _, err = db.Exec(buildUpdate, values...); err != nil {
		logrus.Errorln(err.Error())
		return errors.New(constant.SysError)
	}
	return nil
}

func SelectAccountClashSubscribe(pass string) (*model.Account, error) {
	if pass == "" {
		return nil, errors.New(constant.UnauthorizedError)
	}
	var account model.Account

	where := map[string]interface{}{"pass": pass}
	selectFields := []string{"id", "username", "pass", "deleted", "expire_time", "quota", "download", "upload"}
	buildSelect, values, err := builder.BuildSelect("account", where, selectFields)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	rows, err := db.Query(buildSelect, values...)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	defer rows.Close()

	if err = scanner.Scan(rows, &account); err == scanner.ErrEmptyResult {
		return nil, errors.New(constant.UnauthorizedError)
	} else if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	if account.Pass == nil || *account.Pass != pass {
		return nil, errors.New(constant.UnauthorizedError)
	}
	if account.Deleted == nil || *account.Deleted != 0 {
		return nil, errors.New(constant.AccountDisabled)
	}
	return &account, nil
}

// SelectAccountUnused 查询未使用的账户
func SelectAccountUnused() ([]vo.AccountExportVo, error) {
	var accountExportVo []vo.AccountExportVo
	selectFields := []string{"username", "pass", "login_verifier", "hash", "role_id", "email", "preset_expire", "preset_quota", "last_login_time", "expire_time", "deleted",
		accountPlanQuota, "download", "upload", "create_time"}
	where := map[string]interface{}{"last_login_time": 0}
	buildSelect, values, err := builder.BuildSelect("account", where, selectFields)
	if err != nil {
		logrus.Errorln(err.Error())
		return accountExportVo, errors.New(constant.SysError)
	}

	rows, err := db.Query(buildSelect, values...)
	if err != nil {
		logrus.Errorln(err.Error())
		return accountExportVo, errors.New(constant.SysError)
	}
	defer rows.Close()

	if err = scanner.Scan(rows, &accountExportVo); err != nil {
		logrus.Errorln(err.Error())
		return accountExportVo, errors.New(constant.SysError)
	}
	return accountExportVo, nil
}

func SelectAccountAll() ([]vo.AccountExportVo, error) {
	var accountExportVo []vo.AccountExportVo
	selectFields := []string{"username", "pass", "login_verifier", "hash", "role_id", "email", "preset_expire", "preset_quota", "last_login_time", "expire_time", "deleted",
		accountPlanQuota, "download", "upload", "create_time"}
	buildSelect, values, err := builder.BuildSelect("account", nil, selectFields)
	if err != nil {
		logrus.Errorln(err.Error())
		return accountExportVo, errors.New(constant.SysError)
	}

	rows, err := db.Query(buildSelect, values...)
	if err != nil {
		logrus.Errorln(err.Error())
		return accountExportVo, errors.New(constant.SysError)
	}
	defer rows.Close()

	if err = scanner.Scan(rows, &accountExportVo); err != nil {
		logrus.Errorln(err.Error())
		return accountExportVo, errors.New(constant.SysError)
	}
	return accountExportVo, nil
}

// ErrAccountImportSkipped indicates that the row did not write an account.
var ErrAccountImportSkipped = errors.New("existing or protected account skipped")

// CreateOrUpdateAccount imports by username, preserving the protected administrator.
func CreateOrUpdateAccount(accountModule model.Account, cover uint) error {
	// 如果存在则更新
	account, err := SelectAccountByUsername(accountModule.Username)
	if err != nil && err.Error() != constant.UsernameOrPassError {
		logrus.Errorln(err.Error())
		return errors.New(constant.SysError)
	}
	if account != nil && (cover != 1 || (account.RoleId != nil && *account.RoleId == constant.SYSADMIN)) {
		return ErrAccountImportSkipped
	}
	if accountModule.Pass != nil && *accountModule.Pass != "" {
		var id uint
		if account != nil {
			id = *account.Id
		}
		if err := checkImportedCredential(*accountModule.Pass, id); err != nil {
			return err
		}
	}
	if account != nil {
		accountWhere := map[string]interface{}{
			"id":         *account.Id,
			"username":   *accountModule.Username,
			"role_id <>": constant.SYSADMIN,
		}
		accountUpdate := map[string]interface{}{}
		if accountModule.Pass != nil && *accountModule.Pass != "" {
			accountUpdate["`pass`"] = *accountModule.Pass
			accountUpdate["login_verifier"] = ""
			if accountModule.LegacyLoginVerifier != nil {
				accountUpdate["login_verifier"] = *accountModule.LegacyLoginVerifier
			}
		}
		if accountModule.Hash != nil && *accountModule.Hash != "" {
			accountUpdate["hash"] = *accountModule.Hash
		}
		if accountModule.RoleId != nil && *accountModule.RoleId != 0 {
			accountUpdate["role_id"] = *accountModule.RoleId
		}
		if accountModule.Email != nil {
			accountUpdate["email"] = *accountModule.Email
		}
		if accountModule.PresetExpire != nil && *accountModule.PresetExpire != 0 {
			accountUpdate["preset_expire"] = *accountModule.PresetExpire
		}
		if accountModule.PresetQuota != nil && *accountModule.PresetQuota != 0 {
			accountUpdate["preset_quota"] = *accountModule.PresetQuota
		}
		if accountModule.LastLoginTime != nil && *accountModule.LastLoginTime != 0 {
			accountUpdate["last_login_time"] = *accountModule.LastLoginTime
		}
		if accountModule.ExpireTime != nil && *accountModule.ExpireTime != 0 {
			accountUpdate["expire_time"] = *accountModule.ExpireTime
		}
		if accountModule.Download != nil {
			accountUpdate["download"] = *accountModule.Download
		}
		if accountModule.Upload != nil {
			accountUpdate["upload"] = *accountModule.Upload
		}
		if err := updateAccountFields(&accountModule, accountUpdate, accountWhere); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrAccountImportSkipped
			}
			if insertCredentialError(err) == errSharedCredential {
				return errSharedCredential
			}
			logrus.Error(err)
			return errors.New(constant.SysError)
		}
	} else {
		// 如果存在则忽略，不存在则添加
		if account == nil {
			var data []map[string]interface{}
			accountCreate := map[string]interface{}{}
			if accountModule.Username != nil && *accountModule.Username != "" {
				accountCreate["username"] = *accountModule.Username
			}
			if accountModule.Pass != nil && *accountModule.Pass != "" {
				accountCreate["`pass`"] = *accountModule.Pass
				if accountModule.LegacyLoginVerifier != nil {
					accountCreate["login_verifier"] = *accountModule.LegacyLoginVerifier
				}
			}
			if accountModule.Hash != nil && *accountModule.Hash != "" {
				accountCreate["hash"] = *accountModule.Hash
			}
			if accountModule.RoleId != nil && *accountModule.RoleId != 0 {
				accountCreate["role_id"] = *accountModule.RoleId
			}
			if accountModule.Email != nil {
				accountCreate["email"] = *accountModule.Email
			}
			if accountModule.PresetExpire != nil && *accountModule.PresetExpire != 0 {
				accountCreate["preset_expire"] = *accountModule.PresetExpire
			}
			if accountModule.PresetQuota != nil && *accountModule.PresetQuota != 0 {
				accountCreate["preset_quota"] = *accountModule.PresetQuota
			}
			if accountModule.LastLoginTime != nil && *accountModule.LastLoginTime != 0 {
				accountCreate["last_login_time"] = *accountModule.LastLoginTime
			}
			if accountModule.ExpireTime != nil && *accountModule.ExpireTime != 0 {
				accountCreate["expire_time"] = *accountModule.ExpireTime
			}
			if accountModule.Deleted != nil {
				accountCreate["deleted"] = *accountModule.Deleted
			}
			if accountModule.Quota != nil {
				setAccountQuota(accountCreate, *accountModule.Quota, accountModule.Deleted != nil && *accountModule.Deleted != 0)
			}
			if accountModule.Download != nil {
				accountCreate["download"] = *accountModule.Download
			}
			if accountModule.Upload != nil {
				accountCreate["upload"] = *accountModule.Upload
			}
			if len(accountCreate) > 0 {
				clearQuotaIfPastExpiry(accountCreate, accountModule.ExpireTime)
				data = append(data, accountCreate)
				buildInsert, values, err := builder.BuildInsert("account", data)
				if err != nil {
					logrus.Errorln(err.Error())
					return errors.New(constant.SysError)
				}
				if _, err = db.Exec(buildInsert, values...); err != nil {
					if insertCredentialError(err) == errSharedCredential {
						return errSharedCredential
					}
					logrus.Errorln(err.Error())
					return errors.New(constant.SysError)
				}
			}
		}
	}
	return nil
}
