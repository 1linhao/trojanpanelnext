package api

import (
	"encoding/base64"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"trojan-panel/model"
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
	if client == "clash-meta" && templateId != "default" {
		name := constant.SystemName
		config, err := service.SelectSystemByName(&name)
		if err != nil {
			vo.Fail(err.Error(), c)
			return
		}
		if _, ok := model.FindTemplate(config.TemplateLibrary.Clash, templateId); !ok {
			vo.Fail(constant.ValidateFailed, c)
			return
		}
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
	subscribeUrl := subscriptionQRCodeURL(c.Request, buildSubscribePath(password, client, templateId))
	qrCode, err := qrcode.Encode(subscribeUrl, qrcode.Medium, 256)
	if err != nil {
		vo.Fail(constant.SysError, c)
		return
	}
	vo.Success(base64.StdEncoding.EncodeToString(qrCode), c)
}

func subscriptionQRCodeURL(req *http.Request, path string) string {
	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}
	if forwarded := req.Header.Get("X-Forwarded-Proto"); forwarded == "http" || forwarded == "https" {
		scheme = forwarded
	}
	host := req.Host
	// The bundled UI proxy connects through loopback and rewrites Host to
	// the API listener. Caddy preserves the original authority in this header.
	if loopbackAuthority(req.RemoteAddr) && loopbackAuthority(host) {
		if values := req.Header.Values("X-Forwarded-Host"); len(values) == 1 && validForwardedAuthority(values[0]) {
			host = values[0]
		}
	}
	return scheme + "://" + host + path
}

func loopbackAuthority(authority string) bool {
	host, _, err := net.SplitHostPort(authority)
	if err != nil {
		host = strings.Trim(authority, "[]")
	}
	return strings.EqualFold(host, "localhost") || net.ParseIP(host).IsLoopback()
}

func validForwardedAuthority(authority string) bool {
	if authority == "" || len(authority) > 259 || strings.ContainsAny(authority, ",/\\?#@") || strings.HasSuffix(authority, ":") {
		return false
	}
	parsed, err := url.Parse("//" + authority)
	if err != nil || parsed.Host != authority || parsed.Hostname() == "" {
		return false
	}
	if port := parsed.Port(); port != "" {
		number, err := strconv.ParseUint(port, 10, 16)
		return err == nil && number != 0
	}
	return true
}

// Subscribe 订阅
func Subscribe(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
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
			failSubscription(c, err)
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
			failSubscription(c, err)
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

	account, userInfo, clashConfigYaml, err := service.SubscribeClash(pass, templateId)
	if err != nil {
		failSubscription(c, err)
		return
	}

	c.Header("content-disposition", fmt.Sprintf("attachment; filename=%s.yaml", *account.Username))
	c.Header("content-type", "application/yaml; charset=utf-8")
	c.Header("profile-update-interval", "12")
	c.Header("subscription-userinfo", userInfo)
	c.String(200, string(clashConfigYaml))
	return
	//}
	//vo.Fail("This client is not supported", c)
}

func failSubscription(c *gin.Context, err error) {
	if err.Error() == constant.AccountDisabled {
		c.String(http.StatusForbidden, constant.AccountDisabled)
		return
	}
	vo.Fail(err.Error(), c)
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
	case "clash-meta":
		return templateId != "" && len(templateId) <= 64
	case "v2ray", "shadowrocket":
		return templateId == "default"
	default:
		return false
	}
}
