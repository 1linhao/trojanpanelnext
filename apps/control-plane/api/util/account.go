package util

import (
	"github.com/gin-gonic/gin"
	"strings"
)

// GetToken 获取token
func GetToken(c *gin.Context) string {
	tokenStr := c.Request.Header.Get("Authorization")
	if tokenStr == "" {
		return ""
	}
	return strings.SplitN(tokenStr, " ", 2)[1]
}

func ToMB(b int) int {
	if b >= 0 {
		return b / 1024 / 1024
	} else {
		return -1
	}
}

func ToByte(b int) int {
	if b >= 0 {
		return b * 1024 * 1024
	} else {
		return -1
	}
}

func IsAdmin(roleNames []string) bool {
	for _, item := range roleNames {
		if item == "sysadmin" || item == "admin" {
			return true
		}
	}
	return false
}

func IsSysAdmin(roleNames []string) bool {
	for _, item := range roleNames {
		if item == "sysadmin" {
			return true
		}
	}
	return false
}
