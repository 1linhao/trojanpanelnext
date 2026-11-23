package dao

import (
	"database/sql"
	"errors"

	"github.com/didi/gendry/builder"
	"github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"
	"trojan-panel/model"
	"trojan-panel/model/constant"
	"trojan-panel/util"
)

// Older agents read quota directly. Keep that effective limit at zero while
// disabled, and expose the saved limit to management and backup callers.
const accountPlanQuota = "COALESCE(disabled_quota,quota) AS quota"

func migrateAccountStatus() error {
	for _, column := range []string{"disabled_quota BIGINT NULL", "session_version BIGINT UNSIGNED NOT NULL DEFAULT 0"} {
		_, err := db.Exec("ALTER TABLE account ADD COLUMN " + column)
		var duplicate *mysql.MySQLError
		if err != nil && !(errors.As(err, &duplicate) && duplicate.Number == 1060) {
			return err
		}
	}
	return nil
}

func CheckAccountSession(id uint, username string, version uint64) error {
	var disabled bool
	var currentVersion uint64
	err := db.QueryRow("SELECT deleted,session_version FROM account WHERE id=? AND username=?", id, username).Scan(&disabled, &currentVersion)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return errors.New(constant.UnauthorizedError)
	case err != nil:
		logrus.Error(err)
		return errors.New(constant.SysError)
	case disabled:
		return errors.New(constant.AccountDisabled)
	case currentVersion != version:
		return errors.New(constant.IllegalTokenError)
	}
	return nil
}

func setAccountQuota(fields map[string]interface{}, quota int, disabled bool) {
	fields["quota"] = quota
	fields["disabled_quota"] = nil
	if disabled {
		fields["quota"] = 0
		fields["disabled_quota"] = quota
	}
}

func clearAccountQuota(fields map[string]interface{}) {
	fields["quota"] = 0
	fields["disabled_quota"] = nil
	fields["download"] = 0
	fields["upload"] = 0
}

func clearQuotaIfPastExpiry(fields map[string]interface{}, expiry *uint) {
	if expiry != nil && *expiry <= util.NowMilli() {
		clearAccountQuota(fields)
	}
}

func updateAccountFields(account *model.Account, fields, where map[string]interface{}) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var disabled bool
	var quota int
	var version uint64
	var oldExpiry uint
	query, args, err := builder.BuildSelect("account", where, []string{"deleted", accountPlanQuota, "session_version", "expire_time"})
	if err != nil {
		return err
	}
	if err := tx.QueryRow(query+" FOR UPDATE", args...).Scan(&disabled, &quota, &version, &oldExpiry); err != nil {
		return err
	}
	if account.Deleted != nil || account.Quota != nil || account.ExpireTime != nil {
		if account.Deleted != nil {
			nextDisabled := *account.Deleted != 0
			if !disabled && nextDisabled {
				fields["session_version"] = version + 1
			}
			disabled = nextDisabled
			fields["deleted"] = *account.Deleted
		}
		if account.ExpireTime != nil && oldExpiry > 0 && oldExpiry <= util.NowMilli() && quota != 0 {
			quota = 0
			clearAccountQuota(fields)
		}
		if account.Quota != nil {
			quota = *account.Quota
		}
		setAccountQuota(fields, quota, disabled)
		clearQuotaIfPastExpiry(fields, account.ExpireTime)
	}
	if len(fields) > 0 {
		query, args, err := builder.BuildUpdate("account", where, fields)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(query, args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Check expiry in the UPDATE so a renewal committed before the scan cannot
// be cleared using a stale list of usernames.
func ClearExpiredQuotas() error {
	_, err := db.Exec(`UPDATE account SET quota=0, disabled_quota=NULL, download=0, upload=0
		WHERE COALESCE(disabled_quota,quota) <> 0 AND expire_time <= UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000`)
	return err
}
