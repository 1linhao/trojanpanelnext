package util

import (
	"github.com/mojocn/base64Captcha"
)

// VerifyCaptcha 验证码是否正确
func VerifyCaptcha(captchaId, captchaCode string) bool {
	return base64Captcha.Store.Verify(base64Captcha.DefaultMemStore, captchaId, captchaCode, true)
}
