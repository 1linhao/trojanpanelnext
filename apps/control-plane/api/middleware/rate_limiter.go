package middleware

import (
	"fmt"
	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"github.com/gin-gonic/gin"
	redisgo "github.com/gomodule/redigo/redis"
	"time"
	"trojan-panel/dao"
	"trojan-panel/dao/redis"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
)

var limit *limiter.Limiter

// 限流中间件
func RateLimiterHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// IP黑名单
		if blackListIp(c) {
			return
		}
		// 限流
		httpError := tollbooth.LimitByRequest(limit, c.Writer, c.Request)
		if httpError != nil {
			vo.Fail(constant.RateLimiterError, c)
			c.Abort()
			return
		}
		c.Next()
	}
}

// 限流初始化
func InitRateLimiter() {
	limit = tollbooth.NewLimiter(5, nil)
}

// IP黑名单
func blackListIp(c *gin.Context) bool {
	ip := c.ClientIP()
	get := redis.Client.String.Get(fmt.Sprintf("trojan-panel:black-list:%s", ip))
	result, err := get.String()
	if err != nil && err != redisgo.ErrNil {
		vo.Fail(constant.IllegalTokenError, c)
		c.Abort()
		return true
	}
	if result != "" {
		redis.Client.String.Set(fmt.Sprintf("trojan-panel:black-list:%s", ip), "in-black-list", time.Hour.Milliseconds()/1000)
		vo.Fail(constant.BlackListError, c)
		c.Abort()
		return true
	} else {
		ipCount, err := dao.CountBlackListByIp(&ip)
		if err != nil {
			vo.Fail(err.Error(), c)
			c.Abort()
			return true
		}
		if ipCount > 0 {
			redis.Client.String.Set(fmt.Sprintf("trojan-panel:black-list:%s", ip), "in-black-list", time.Hour.Milliseconds()/1000)
			vo.Fail(constant.BlackListError, c)
			c.Abort()
			return true
		}
		return false
	}
}
