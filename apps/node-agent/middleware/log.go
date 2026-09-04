package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"strings"
	"time"
	"trojan-panel-core/core"
)

func InitLog() {
	// logging with rolling compression
	logConfig := core.Config.LogConfig
	logrus.SetOutput(&lumberjack.Logger{
		Filename:   logConfig.FileName,
		MaxSize:    logConfig.MaxSize,
		MaxBackups: logConfig.MaxBackups,
		MaxAge:     logConfig.MaxAge,
		Compress:   logConfig.Compress,
		LocalTime:  true,
	})
	//logrus.SetReportCaller(true)
	logrus.SetFormatter(&logrus.JSONFormatter{TimestampFormat: "2006-01-02 15:04:05"})
	logrus.AddHook(newSecretRedactionHook(
		core.Config.MySQLConfig.Password,
		core.Config.RedisConfig.Password,
	))
	// set logging level
	logrus.SetLevel(logrus.WarnLevel)
}

type secretRedactionHook struct {
	secrets []string
}

func newSecretRedactionHook(values ...string) *secretRedactionHook {
	hook := &secretRedactionHook{}
	for _, value := range values {
		if len(value) >= 4 {
			hook.secrets = append(hook.secrets, value)
		}
	}
	return hook
}

func (h *secretRedactionHook) Levels() []logrus.Level { return logrus.AllLevels }

func (h *secretRedactionHook) Fire(entry *logrus.Entry) error {
	for _, secret := range h.secrets {
		entry.Message = strings.ReplaceAll(entry.Message, secret, "[REDACTED]")
		for key, value := range entry.Data {
			if text, ok := value.(string); ok {
				entry.Data[key] = strings.ReplaceAll(text, secret, "[REDACTED]")
			}
		}
	}
	return nil
}

func LogHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		c.Next()
		endTime := time.Now()
		latencyTime := endTime.Sub(startTime)
		reqMethod := c.Request.Method
		reqUri := c.Request.RequestURI
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()

		logrus.WithFields(logrus.Fields{
			"status_code":  statusCode,
			"latency_time": latencyTime,
			"client_ip":    clientIP,
			"req_method":   reqMethod,
			"req_uri":      reqUri,
		}).Info()
	}
}
