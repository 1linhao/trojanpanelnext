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
	if account.Deleted != nil && *account.Deleted == 1 {
		if err = PullAccountWhiteOrBlackByUsername([]string{*account.Username}, true); err != nil {
			return err
		}
	} else if *account.ExpireTime <= util.NowMilli() {
		if err = DisableAccount([]string{*account.Username}); err != nil {
			return err
		}
	}
	return nil
}

func SelectAccountById(id *uint) (*model.Account, error) {
	return dao.SelectAccountById(id)
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
	pageSize *uint) (*vo.AccountPageVo, error) {
	return dao.SelectAccountPage(username, deleted, lastLoginTime, orderFields, orderBy, pageNum, pageSize)
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

func UpdateAccountPass(token string, oldPass *string, newPass *string, username *string) error {
	mutex, err := redis.RsLock(constant.UpdateAccountPassLock)
	if err != nil {
		return err
	}

	account, err := SelectAccountByUsername(username)
	if err != nil || !util.Sha1Match(*account.Pass, fmt.Sprintf("%s%s", *username, *oldPass)) {
		return errors.New(constant.OriPassError)
	}

	if err = RemoveAccount(token, *account.Pass); err != nil {
		return err
	}

	if err := dao.UpdateAccountPass(oldPass, newPass, username); err != nil {
		return err
	}
	redis.RsUnLock(mutex)
	return nil
}

func UpdateAccountProperty(token string, oldUsername *string, pass *string, username *string, email *string) error {
	mutex, err := redis.RsLock(constant.UpdateAccountPropertyLock)
	if err != nil {
		return err
	}
	account, err := SelectAccountByUsername(oldUsername)
	if err != nil || !util.Sha1Match(*account.Pass, fmt.Sprintf("%s%s", *oldUsername, *pass)) {
		return errors.New(constant.OriPassError)
	}

	if pass != nil && *pass != "" && username != nil && *username != "" {
		if err = RemoveAccount(token, *account.Pass); err != nil {
			return err
		}
	}

	if err := dao.UpdateAccountProperty(oldUsername, pass, username, email); err != nil {
		return err
	}
	redis.RsUnLock(mutex)
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

func UpdateAccountById(token string, account *model.Account) error {
	mutex, err := redis.RsLock(constant.UpdateAccountByIdLock)
	if err != nil {
		return err
	}
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
	if account.Deleted != nil && *account.Deleted == 1 {
		if err := PullAccountWhiteOrBlackByUsername([]string{*account.Username}, true); err != nil {
			return err
		}
	} else if account.ExpireTime != nil && *account.ExpireTime <= util.NowMilli() {
		if err := DisableAccount([]string{*account.Username}); err != nil {
			return err
		}
	}
	redis.RsUnLock(mutex)
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

// PullAccountWhiteOrBlackByUsername 拉白或者拉黑用户 此操作会清空用户流量
func PullAccountWhiteOrBlackByUsername(usernames []string, isBlack bool) error {
	if len(usernames) > 0 {
		var deleted uint
		if isBlack {
			deleted = 1
		} else {
			deleted = 0
		}
		if err := dao.UpdateAccountByUsernames(usernames, new(int), new(uint), new(uint), &deleted); err != nil {
			return err
		}
	}
	return nil
}

// DisableAccount 清空流量
func DisableAccount(usernames []string) error {
	if len(usernames) > 0 {
		if err := dao.UpdateAccountByUsernames(usernames, new(int), new(uint), new(uint), nil); err != nil {
			return err
		}
	}
	return nil
}

// CronScanAccounts 定时任务：扫描无效用户
func CronScanAccounts() {
	usernames, err := dao.SelectAccountUsernameByDeletedOrExpireTime()
	if err != nil {
		return
	}

	if len(usernames) > 0 {
		if err = DisableAccount(usernames); err != nil {
			logrus.Errorf("定时任务：扫描无效用户异常 usernames: %s error: %v", usernames, err)
		}
		logrus.Infof("定时任务：扫描无效用户 usernames: %s", usernames)
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

func CreateAccountBatch(accountId uint, accountUsername string, dto dto.CreateAccountBatchDto) error {
	var exportVos []vo.AccountUnusedExportVo
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
		exportVo := vo.AccountUnusedExportVo{
			Username: randStr,
			Pass:     randStr,
		}
		exportVos = append(exportVos, exportVo)
		if err := dao.CreateAccount(&account); err != nil {
			logrus.Errorf("batch create account err: %v", err)
			continue
		}
	}
	if len(exportVos) > 0 {
		if err := ExportTaskJson(accountId, accountUsername, constant.TaskTypeAccountExport, "batchCreateAccountExport", exportVos); err != nil {
			return err
		}
	}
	return nil
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
