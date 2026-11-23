package middleware

import (
	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"github.com/gin-gonic/gin"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
)

var limit *limiter.Limiter

// 限流中间件
func RateLimiterHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
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
