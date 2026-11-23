package service

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"time"
	"trojan-panel/core"
	"trojan-panel/dao"
	"trojan-panel/dao/redis"
	"trojan-panel/model"
	"trojan-panel/model/constant"
	"trojan-panel/model/dto"
	"trojan-panel/model/vo"
	"trojan-panel/util"
	"unicode/utf8"
)

func CreateAccount(accountCreateDto dto.AccountCreateDto) error {
	count, err := dao.CountAccountByUsername(accountCreateDto.Username)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New(constant.UsernameExist)
	}
	toByte := util.ToByte(*accountCreateDto.Quota)
	lastLoginTime := uint(time.Now().UnixMilli())
	account := model.Account{
		Username:      accountCreateDto.Username,
		Pass:          accountCreateDto.Pass,
		RoleId:        accountCreateDto.RoleId,
		Email:         accountCreateDto.Email,
		LastLoginTime: &lastLoginTime,
		ExpireTime:    accountCreateDto.ExpireTime,
		Deleted:       accountCreateDto.Deleted,
		Quota:         &toByte,
		//IpLimit:            accountCreateDto.IpLimit,
		//DownloadSpeedLimit: accountCreateDto.DownloadSpeedLimit,
		//UploadSpeedLimit:   accountCreateDto.UploadSpeedLimit,
	}
	if err = dao.CreateAccount(&account); err != nil {
		return err
	}

	return nil
}

func SelectAccountById(id *uint, includeRemark ...bool) (*model.Account, error) {
	return dao.SelectAccountById(id, includeRemark...)
}

func UpdateAccountNodeAccess(id uint, access dao.AccountNodeAccess) (uint64, error) {
	nodeLifecycle.Lock()
	defer nodeLifecycle.Unlock()
	return dao.UpdateAccountNodeAccess(id, access)
}

func CountAccountByUsername(username *string) (int, error) {
	return dao.CountAccountByUsername(username)
}

func SelectAccountPage(
	username *string,
	deleted *uint,
	lastLoginTime *uint,
	orderFields *string,
	orderBy *string,
	pageNum *uint,
	pageSize *uint,
	includeRemark ...bool) (*vo.AccountPageVo, error) {
	return dao.SelectAccountPage(username, deleted, lastLoginTime, orderFields, orderBy, pageNum, pageSize, includeRemark...)
}

func DeleteAccountById(token string, id *uint) error {
	mutex, err := redis.RsLock(constant.DeleteAccountByIdLock)
	if err != nil {
		return err
	}
	password, err := dao.SelectConnectPassword(id, nil)
	if err != nil {
		return err
	}
	if err = RemoveAccount(token, password); err != nil {
		return err
	}
	if err = dao.DeleteAccountById(id); err != nil {
		return err
	}
	redis.RsUnLock(mutex)
	return nil
}

func SelectAccountByUsername(username *string) (*model.Account, error) {
	return dao.SelectAccountByUsername(username)
}

func UpdateAccountPass(token string, oldPass *string, newPass *string, id uint) error {
	mutex, err := redis.RsLock(constant.UpdateAccountPassLock)
	if err != nil {
		return err
	}

	defer redis.RsUnLock(mutex)
	account, err := dao.SelectAccountIdentity(id)
	if err != nil || !util.Sha1Match(account.LoginVerifier(), fmt.Sprintf("%s%s", *account.Username, *oldPass)) {
		return errors.New(constant.OriPassError)
	}

	if err = RemoveAccount(token, *account.Pass); err != nil {
		return err
	}

	if err := dao.UpdateAccountPass(account, *newPass); err != nil {
		return err
	}
	return nil
}

func UpdateAccountProperty(token string, id uint, pass *string, username *string, email *string) error {
	mutex, err := redis.RsLock(constant.UpdateAccountPropertyLock)
	if err != nil {
		return err
	}
	defer redis.RsUnLock(mutex)
	account, err := dao.SelectAccountIdentity(id)
	if err != nil || !util.Sha1Match(account.LoginVerifier(), fmt.Sprintf("%s%s", *account.Username, *pass)) {
		return errors.New(constant.OriPassError)
	}

	if pass != nil && *pass != "" && username != nil && *username != "" && *username != *account.Username {
		if err = RemoveAccount(token, *account.Pass); err != nil {
			return err
		}
	}

	if err := dao.UpdateAccountProperty(account, pass, username, email); err != nil {
		return err
	}
	return nil
}

// GetAccountInfo 获取当前请求账户信息
func GetAccountInfo(c *gin.Context) (*vo.AccountInfo, error) {
	accountVo := GetCurrentAccount(c)
	roles, err := dao.SelectRoleNameByParentId(&accountVo.RoleId, true)
	if err != nil {
		return nil, err
	}
	userInfo := vo.AccountInfo{
		Id:       accountVo.Id,
		Username: accountVo.Username,
		Roles:    roles,
	}
	return &userInfo, nil
}

// IsCurrentSysAdmin checks the signed identity and its current role.
// Account roles in a JWT may outlive a role change made during that session.
func IsCurrentSysAdmin(token string) (bool, error) {
	claims, err := ParseToken(token)
	if err != nil {
		return false, err
	}
	if !util.IsSysAdmin(claims.AccountVo.Roles) {
		return false, nil
	}
	account, err := dao.SelectAccountById(&claims.AccountVo.Id)
	if err != nil {
		return false, err
	}
	return account.RoleId != nil && *account.RoleId == constant.SYSADMIN && account.Deleted != nil && *account.Deleted == 0, nil
}

func UpdateAccountById(token string, account *model.Account) error {
	if account.Remark != nil {
		if utf8.RuneCountInString(*account.Remark) > 500 {
			return errors.New(constant.ValidateFailed)
		}
	}
	// Management edits always contain RoleId. Login bookkeeping does not:
	// it only records login time and activates preset quota/expiry fields.
	if account.RoleId != nil || account.Remark != nil {
		allowed, err := IsCurrentSysAdmin(token)
		if err != nil {
			return err
		}
		if !allowed {
			return errors.New(constant.ForbiddenError)
		}
	}
	mutex, err := redis.RsLock(constant.UpdateAccountByIdLock)
	if err != nil {
		return err
	}
	defer redis.RsUnLock(mutex)
	if account.Pass != nil && *account.Pass != "" {
		password, err := dao.SelectConnectPassword(account.Id, nil)
		if err != nil {
			return err
		}
		if err = RemoveAccount(token, password); err != nil {
			return err
		}
	}
	if err := dao.UpdateAccountById(account); err != nil {
		return err
	}

	return nil
}

func Register(accountRegisterDto dto.AccountRegisterDto) error {
	name := constant.SystemName
	systemVo, err := SelectSystemByName(&name)
	if err != nil {
		return err
	}
	if systemVo.RegisterEnable == 0 {
		return errors.New(constant.AccountRegisterClosed)
	}

	count, err := dao.CountAccountByUsername(accountRegisterDto.Username)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New(constant.UsernameExist)
	}
	u := constant.USER
	milli := util.DayToMilli(systemVo.RegisterExpireDays)
	registerQuota := util.ToByte(systemVo.RegisterQuota)
	lastLoginTime := uint(time.Now().UnixMilli())
	account := model.Account{
		Quota:         &registerQuota,
		Username:      accountRegisterDto.Username,
		Pass:          accountRegisterDto.Pass,
		RoleId:        &u,
		Deleted:       new(uint),
		LastLoginTime: &lastLoginTime,
		ExpireTime:    &milli,
	}
	if err = dao.CreateAccount(&account); err != nil {
		return err
	}
	return nil
}

// CronScanAccounts applies natural expiry without changing account login state.
func CronScanAccounts() {
	if err := dao.ClearExpiredQuotas(); err != nil {
		logrus.WithError(err).Error("clear expired quotas")
	}
}

// CronScanAccountExpireWarn 定时任务：到期警告
func CronScanAccountExpireWarn() {
	systemName := constant.SystemName
	systemVo, err := SelectSystemByName(&systemName)
	if err != nil {
		return
	}
	if systemVo.EmailEnable == 0 || systemVo.ExpireWarnEnable == 0 {
		return
	}
	expireWarnDay := systemVo.ExpireWarnDay
	accounts, err := dao.SelectAccountsByExpireTime(util.DayToMilli(expireWarnDay))
	if err != nil {
		logrus.Errorln(err.Error())
		return
	}
	if len(accounts) > 0 {
		for _, account := range accounts {
			if account.Email != nil && *account.Email != "" {
				// 发送到期邮件
				emailDto := dto.SendEmailDto{
					FromEmailName: systemVo.SystemName,
					ToEmails:      []string{*account.Email},
					Subject:       "账号到期提醒",
					Content:       fmt.Sprintf("您的账户: %s,还有%d天到期,请及时续期", *account.Username, expireWarnDay),
				}
				if err = SendEmail(&emailDto); err != nil {
					logrus.Errorln(fmt.Sprintf("到期警告邮件发送失败 err: %v", err))
				}
			}
		}
	}
}

// CronResetDownloadAndUploadMonth 定时任务：每月重设除管理员之外的用户下载和上传流量
func CronResetDownloadAndUploadMonth() {
	name := constant.SystemName
	systemConfig, err := SelectSystemByName(&name)
	if err != nil {
		logrus.Errorf("每月重设除管理员之外的用户下载和上传流量 查询系统设置异常 error: %v", err)
		return
	}
	if systemConfig.ResetDownloadAndUploadMonth == 1 {
		roleIds := []uint{constant.USER}
		if err := dao.ResetAccountDownloadAndUpload(nil, &roleIds); err != nil {
			logrus.Errorf("每月重设除管理员之外的用户下载和上传流量异常 roleIds: %v error: %v", roleIds, err)
		}
	}
}

func RemoveAccount(token string, password string) error {
	nodeServers, err := dao.SelectNodeServersForControl()
	if err != nil {
		return err
	}
	for i := range nodeServers {
		nodeServer := &nodeServers[i]
		removeDto := core.AccountRemoveDto{
			Password: password,
		}
		_ = core.RemoveAccount(token, *nodeServer.Ip, *nodeServer.GrpcPort, &removeDto, nodeTransport(nodeServer))
	}
	return nil
}

func SelectConnectPassword(id *uint, username *string) (string, error) {
	return dao.SelectConnectPassword(id, username)
}

// ResetAccountDownloadAndUpload 重设下载和上传流量
func ResetAccountDownloadAndUpload(id *uint, roleIds *[]uint) error {
	return dao.ResetAccountDownloadAndUpload(id, roleIds)
}

func ResetAllAccountDownloadAndUpload(token string) error {
	allowed, err := IsCurrentSysAdmin(token)
	if err != nil {
		return err
	}
	if !allowed {
		return errors.New(constant.ForbiddenError)
	}
	return dao.ResetAccountDownloadAndUpload(nil, nil)
}

func ResetAccountLoginLimit(token string, id *uint) error {
	if id == nil || *id == 0 {
		return errors.New(constant.ValidateFailed)
	}
	allowed, err := IsCurrentSysAdmin(token)
	if err != nil {
		return err
	}
	if !allowed {
		return errors.New(constant.ForbiddenError)
	}
	account, err := dao.SelectAccountById(id)
	if err != nil {
		return err
	}
	if _, err := redis.Client.Key.Del(fmt.Sprintf("trojan-panel:login-limit:%s", *account.Username)).Result(); err != nil {
		return errors.New(constant.SysError)
	}
	return nil
}

func LoginLimit(username string) {
	redis.Client.String.
		Incr(fmt.Sprintf("trojan-panel:login-limit:%s", username))
}

// LoginVerify 密码输入错误3次以上 将账户锁定30分钟
func LoginVerify(username string) error {
	get := redis.Client.String.
		Get(fmt.Sprintf("trojan-panel:login-limit:%s", username))
	reply, err := get.Result()
	if err != nil {
		return errors.New(constant.SysError)
	}
	if reply != nil {
		result, err := get.Int()
		if err != nil {
			return errors.New(constant.SysError)
		}
		if result >= 3 {
			redis.Client.String.Set(fmt.Sprintf("trojan-panel:login-limit:%s", username), -1, time.Minute.Milliseconds()*30/1000)
		}
		if result >= 3 || result == -1 {
			return errors.New(constant.LoginLimitError)
		}
	}
	return nil
}

func CreateAccountBatch(dto dto.CreateAccountBatchDto) ([]vo.AccountUnusedExportVo, error) {
	exportVos := make([]vo.AccountUnusedExportVo, 0, *dto.Num)
	for i := 0; i < *dto.Num; i++ {
		randStr := util.RandString(12)
		role := constant.USER
		toByte := util.ToByte(*dto.PresetQuota)
		account := model.Account{
			Username:     &randStr,
			Pass:         &randStr,
			RoleId:       &role,
			PresetExpire: dto.PresetExpire,
			PresetQuota:  &toByte,
		}
		if err := dao.CreateAccount(&account); err != nil {
			logrus.Errorf("batch create account err: %v", err)
			continue
		}
		exportVos = append(exportVos, vo.AccountUnusedExportVo{
			Username: randStr,
			Pass:     randStr,
		})
	}
	if len(exportVos) == 0 {
		return nil, errors.New(constant.SysError)
	}
	return exportVos, nil
}

// GetCurrentAccount 获取当前用户
func GetCurrentAccount(c *gin.Context) *vo.AccountVo {
	// 解析token获取当前用户信息
	myClaims, err := ParseToken(util.GetToken(c))
	if err != nil {
		vo.Fail(err.Error(), c)
		return nil
	}
	accountVo := myClaims.AccountVo
	return &accountVo
}
