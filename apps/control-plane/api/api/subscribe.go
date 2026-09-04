package api

import (
	"encoding/base64"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"
	"net/url"
	"strconv"
	"strings"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
	"trojan-panel/service"
	"trojan-panel/util"
)

func ExportOptions(c *gin.Context) {
	options, err := service.ExportOptions()
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(options, c)
}

func ExportSubscribe(c *gin.Context) {
	client := strings.ToLower(c.Query("client"))
	templateId := c.Query("template")
	if !validExportSelection(client, templateId) {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	accountId, username, ok := exportAccount(c)
	if !ok {
		return
	}
	password, err := service.SelectConnectPassword(accountId, username)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(buildSubscribePath(password, client, templateId), c)
}

func ExportQRCode(c *gin.Context) {
	client := strings.ToLower(c.Query("client"))
	templateId := c.Query("template")
	if (client != "v2ray" && client != "shadowrocket") || !validExportSelection(client, templateId) {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	accountId, username, ok := exportAccount(c)
	if !ok {
		return
	}
	password, err := service.SelectConnectPassword(accountId, username)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	scheme := c.GetHeader("X-Forwarded-Proto")
	if scheme == "" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	subscribeUrl := fmt.Sprintf("%s://%s%s", scheme, c.Request.Host, buildSubscribePath(password, client, templateId))
	qrCode, err := qrcode.Encode(subscribeUrl, qrcode.Medium, 256)
	if err != nil {
		vo.Fail(constant.SysError, c)
		return
	}
	vo.Success(base64.StdEncoding.EncodeToString(qrCode), c)
}

// Subscribe 订阅
func Subscribe(c *gin.Context) {
	token := c.Param("token")
	//userAgent := c.Request.Header.Get("User-Agent")
	tokenDecode, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		vo.Fail(constant.SysError, c)
		return
	}
	pass := string(tokenDecode)

	client := strings.ToLower(c.Query("client"))
	templateId := c.Query("template")
	if !validExportSelection(client, templateId) {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	if client == "sing-box" {
		account, userInfo, singBoxConfigJson, err := service.SubscribeSingBox(pass, templateId)
		if err != nil {
			vo.Fail(err.Error(), c)
			return
		}
		c.Header("content-disposition", fmt.Sprintf("attachment; filename=%s-sing-box.json", *account.Username))
		c.Header("content-type", "application/json; charset=utf-8")
		c.Header("profile-update-interval", "12")
		c.Header("subscription-userinfo", userInfo)
		c.String(200, string(singBoxConfigJson))
		return
	}
	if client == "v2ray" || client == "shadowrocket" {
		account, userInfo, v2rayConfig, err := service.SubscribeURI(pass, c.GetHeader("User-Agent"), client)
		if err != nil {
			vo.Fail(err.Error(), c)
			return
		}
		c.Header("vary", "User-Agent")
		c.Header("content-disposition", fmt.Sprintf("attachment; filename=%s-%s.txt", *account.Username, client))
		c.Header("content-type", "text/plain; charset=utf-8")
		c.Header("profile-update-interval", "12")
		c.Header("subscription-userinfo", userInfo)
		c.String(200, string(v2rayConfig))
		return
	}

	account, userInfo, clashConfigYaml, systemConfig, err := service.SubscribeClash(pass)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	result := fmt.Sprintf(`%s
%s`, string(clashConfigYaml), systemConfig.ClashRule)

	c.Header("content-disposition", fmt.Sprintf("attachment; filename=%s.yaml", *account.Username))
	c.Header("content-type", "application/yaml; charset=utf-8")
	c.Header("profile-update-interval", "12")
	c.Header("subscription-userinfo", userInfo)
	c.String(200, result)
	return
	//}
	//vo.Fail("This client is not supported", c)
}

func exportAccount(c *gin.Context) (*uint, *string, bool) {
	accountVo := service.GetCurrentAccount(c)
	if accountVo == nil {
		return nil, nil, false
	}
	if idStr := c.Query("id"); idStr != "" {
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil || id == 0 {
			vo.Fail(constant.ValidateFailed, c)
			return nil, nil, false
		}
		accountId := uint(id)
		if accountId != accountVo.Id && !util.IsAdmin(accountVo.Roles) {
			vo.Fail(constant.ForbiddenError, c)
			return nil, nil, false
		}
		if accountId == accountVo.Id {
			return &accountId, &accountVo.Username, true
		}
		return &accountId, nil, true
	}
	return &accountVo.Id, &accountVo.Username, true
}

func buildSubscribePath(password string, client string, templateId string) string {
	token := base64.RawURLEncoding.EncodeToString([]byte(password))
	query := url.Values{}
	query.Set("client", client)
	query.Set("template", templateId)
	return fmt.Sprintf("/api/auth/subscribe/%s?%s", token, query.Encode())
}

func validExportSelection(client string, templateId string) bool {
	switch strings.ToLower(client) {
	case "sing-box":
		return templateId == "tun" || templateId == "outbound"
	case "clash-meta", "v2ray", "shadowrocket":
		return templateId == "default"
	default:
		return false
	}
}
