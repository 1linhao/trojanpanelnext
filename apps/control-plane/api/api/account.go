package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"
	"time"
	"trojan-panel/dao"
	"trojan-panel/dao/redis"
	"trojan-panel/model"
	"trojan-panel/model/constant"
	"trojan-panel/model/dto"
	"trojan-panel/model/vo"
	"trojan-panel/service"
	"trojan-panel/util"
)

func Login(c *gin.Context) {
	var accountLoginDto dto.AccountLoginDto
	_ = c.ShouldBindJSON(&accountLoginDto)
	if err := validate.Struct(&accountLoginDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	if err := service.LoginVerify(*accountLoginDto.Username); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	systemName := constant.SystemName
	systemVo, err := service.SelectSystemByName(&systemName)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	if systemVo.CaptchaEnable == 1 && !util.VerifyCaptcha(*accountLoginDto.CaptchaId, *accountLoginDto.CaptchaCode) {
		vo.Fail(constant.CaptchaError, c)
		return
	}
	account, err := service.SelectAccountByUsername(accountLoginDto.Username)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	if account != nil {
		if !util.Sha1Match(*account.Pass, fmt.Sprintf("%s%s", *accountLoginDto.Username, *accountLoginDto.Pass)) {
			vo.Fail(constant.UsernameOrPassError, c)
			service.LoginLimit(*account.Username)
			return
		}
		if *account.Deleted != 0 {
			vo.Fail(constant.AccountDisabled, c)
			return
		}
		roles, err := dao.SelectRoleNameByParentId(account.RoleId, true)
		if err != nil {
			vo.Fail(constant.SysError, c)
			return
		}
		accountVo := vo.AccountVo{
			Id:       *account.Id,
			Username: *account.Username,
			RoleId:   *account.RoleId,
			Deleted:  *account.Deleted,
			Roles:    roles,
		}
		tokenStr, err := service.GenToken(accountVo)
		if err != nil {
			vo.Fail(constant.SysError, c)
		} else {
			if _, err := redis.Client.String.
				Set(fmt.Sprintf("trojan-panel:token:%s", *accountLoginDto.Username), tokenStr,
					time.Hour.Milliseconds()*12/1000).Result(); err != nil {
				vo.Fail(constant.SysError, c)
			} else {
				milli := uint(time.Now().UnixMilli())
				// 记录最后登录时间
				accountUpdate := model.Account{
					Id:            account.Id,
					LastLoginTime: &milli,
				}
				// 第一次登录且有预设
				if account.LastLoginTime != nil && *account.LastLoginTime == 0 &&
					account.PresetExpire != nil && *account.PresetExpire >= 1 &&
					account.PresetQuota != nil && *account.PresetQuota >= -1 {
					expireTime := milli + *account.PresetExpire*24*60*60*1000
					accountUpdate.ExpireTime = &expireTime
					accountUpdate.Quota = account.PresetQuota
				}
				if err := service.UpdateAccountById(tokenStr, &accountUpdate); err != nil {
					vo.Fail(constant.SysError, c)
					return
				}
				accountLoginVo := vo.AccountLoginVo{
					Token: tokenStr,
				}
				vo.Success(accountLoginVo, c)
			}
		}
		return
	}
	vo.Fail(constant.UsernameOrPassError, c)
}

// GenerateCaptcha 验证码
func GenerateCaptcha(c *gin.Context) {
	driver := base64Captcha.NewDriverMath(80, 240, 8, 3, nil, nil, []string{"wqy-microhei.ttc"})
	captcha := base64Captcha.NewCaptcha(driver, base64Captcha.DefaultMemStore)
	// 生成验证码图片和答案
	id, b64s, err := captcha.Generate()
	if err != nil {
		vo.Fail(constant.CaptchaGenerateError, c)
		return
	}
	captureVo := vo.CaptureVo{
		CaptchaId:  id,
		CaptchaImg: b64s,
	}
	vo.Success(captureVo, c)
}

func Register(c *gin.Context) {
	var accountRegisterDto dto.AccountRegisterDto
	_ = c.ShouldBindJSON(&accountRegisterDto)
	if err := validate.Struct(&accountRegisterDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	systemName := constant.SystemName
	systemVo, err := service.SelectSystemByName(&systemName)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	if systemVo.CaptchaEnable == 1 && !util.VerifyCaptcha(*accountRegisterDto.CaptchaId, *accountRegisterDto.CaptchaCode) {
		vo.Fail(constant.CaptchaError, c)
		return
	}
	if err := service.Register(accountRegisterDto); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}

func Logout(c *gin.Context) {
	account := service.GetCurrentAccount(c)
	if err := redis.Client.Key.RetryDel(fmt.Sprintf("trojan-panel:token:%s", account.Username)); err != nil {
		vo.Fail(constant.LogOutError, c)
		return
	}
	vo.Success(nil, c)
}

func CreateAccount(c *gin.Context) {
	var accountCreateDto dto.AccountCreateDto
	_ = c.ShouldBindJSON(&accountCreateDto)
	if err := validate.Struct(&accountCreateDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	if err := service.CreateAccount(accountCreateDto); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}

func SelectAccountById(c *gin.Context) {
	var accountRequiredIdDto dto.RequiredIdDto
	_ = c.ShouldBindQuery(&accountRequiredIdDto)
	if err := validate.Struct(&accountRequiredIdDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	account, err := service.SelectAccountById(accountRequiredIdDto.Id)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	accountVo := vo.AccountVo{
		Id:           *account.Id,
		Username:     *account.Username,
		RoleId:       *account.RoleId,
		Email:        *account.Email,
		PresetExpire: *account.PresetExpire,
		PresetQuota:  *account.PresetQuota,
		ExpireTime:   *account.ExpireTime,
		Deleted:      *account.Deleted,
		Quota:        *account.Quota,
		Download:     *account.Download,
		Upload:       *account.Upload,
	}
	vo.Success(accountVo, c)
}

func SelectAccountPage(c *gin.Context) {
	var accountPageDto dto.AccountPageDto
	_ = c.ShouldBindQuery(&accountPageDto)
	if err := validate.Struct(&accountPageDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	page, err := service.SelectAccountPage(
		accountPageDto.Username,
		accountPageDto.Deleted,
		accountPageDto.LastLoginTime,
		accountPageDto.OrderFields,
		accountPageDto.OrderBy,
		accountPageDto.PageNum,
		accountPageDto.PageSize)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(page, c)
}

func DeleteAccountById(c *gin.Context) {
	var accountRequiredIdDto dto.RequiredIdDto
	_ = c.ShouldBindJSON(&accountRequiredIdDto)
	if err := validate.Struct(&accountRequiredIdDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	account, err := service.SelectAccountById(accountRequiredIdDto.Id)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	if *account.RoleId == constant.SYSADMIN {
		vo.Fail(constant.NoDeleteSysadmin, c)
		return
	}
	if err := service.DeleteAccountById(util.GetToken(c), accountRequiredIdDto.Id); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}

func UpdateAccountPass(c *gin.Context) {
	var accountUpdatePassDto dto.AccountUpdatePassDto
	_ = c.ShouldBindJSON(&accountUpdatePassDto)
	if err := validate.Struct(&accountUpdatePassDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	account := service.GetCurrentAccount(c)
	if err := service.UpdateAccountPass(util.GetToken(c), accountUpdatePassDto.OldPass, accountUpdatePassDto.NewPass,
		&account.Username); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}

func UpdateAccountProperty(c *gin.Context) {
	var accountUpdatePropertyDto dto.AccountUpdatePropertyDto
	_ = c.ShouldBindJSON(&accountUpdatePropertyDto)
	if err := validate.Struct(&accountUpdatePropertyDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	account := service.GetCurrentAccount(c)
	// 校验用户名是否重复
	count, err := service.CountAccountByUsername(accountUpdatePropertyDto.Username)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	if count > 0 {
		vo.Fail(constant.UsernameExist, c)
		return
	}

	if err := service.UpdateAccountProperty(util.GetToken(c), &account.Username,
		accountUpdatePropertyDto.Pass, accountUpdatePropertyDto.Username, accountUpdatePropertyDto.Email); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}

func GetAccountInfo(c *gin.Context) {
	accountInfo, err := service.GetAccountInfo(c)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(accountInfo, c)
}

func UpdateAccountById(c *gin.Context) {
	var accountUpdateDto dto.AccountUpdateDto
	_ = c.ShouldBindJSON(&accountUpdateDto)
	if err := validate.Struct(&accountUpdateDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}

	if accountUpdateDto.Deleted != nil && *accountUpdateDto.Deleted != 0 {
		// sysadmin cannot not be disabled
		account, err := service.SelectAccountById(accountUpdateDto.Id)
		if err != nil {
			vo.Fail(err.Error(), c)
			return
		}
		if *account.RoleId == constant.SYSADMIN {
			vo.Fail(constant.NoDisableSysadmin, c)
			return
		}
	}

	toByte := util.ToByte(*accountUpdateDto.Quota)
	account := model.Account{
		Id:         accountUpdateDto.Id,
		Quota:      &toByte,
		Username:   accountUpdateDto.Username,
		Pass:       accountUpdateDto.Pass,
		Email:      accountUpdateDto.Email,
		RoleId:     accountUpdateDto.RoleId,
		Deleted:    accountUpdateDto.Deleted,
		ExpireTime: accountUpdateDto.ExpireTime,
		//IpLimit:            accountUpdateDto.IpLimit,
		//UploadSpeedLimit:   accountUpdateDto.UploadSpeedLimit,
		//DownloadSpeedLimit: accountUpdateDto.DownloadSpeedLimit,
	}
	if err := service.UpdateAccountById(util.GetToken(c), &account); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}

// ResetAccountDownloadAndUpload 重设下载和上传流量
func ResetAccountDownloadAndUpload(c *gin.Context) {
	var accountRequiredIdDto dto.RequiredIdDto
	_ = c.ShouldBindJSON(&accountRequiredIdDto)
	if err := validate.Struct(&accountRequiredIdDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	if err := service.ResetAccountDownloadAndUpload(accountRequiredIdDto.Id, nil); err != nil {
		vo.Fail(constant.SysError, c)
		return
	}
	vo.Success(nil, c)
}

func CreateAccountBatch(c *gin.Context) {
	var createAccountBatchDto dto.CreateAccountBatchDto
	_ = c.ShouldBindJSON(&createAccountBatchDto)
	if err := validate.Struct(&createAccountBatchDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	account := service.GetCurrentAccount(c)
	if err := service.CreateAccountBatch(account.Id, account.Username, createAccountBatchDto); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}
