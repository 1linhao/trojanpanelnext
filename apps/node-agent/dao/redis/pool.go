package redis

import (
	"fmt"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/redigo"
	"github.com/gomodule/redigo/redis"
	"github.com/sirupsen/logrus"
	"time"
	"trojan-panel-core/core"
)

// 连接池
var pool *redis.Pool
var authPool *redis.Pool

// 分布式锁
var rs *redsync.Redsync

func InitRedis() {
	redisConfig := core.Config.RedisConfig
	pool = newPool(redisConfig.Username, redisConfig.Password)
	authPool = newPool(redisConfig.AuthUsername, redisConfig.AuthPassword)
	rs = redsync.New(redigo.NewPool(pool))
}

func newPool(username, password string) *redis.Pool {
	redisConfig := core.Config.RedisConfig
	return &redis.Pool{
		MaxIdle:     redisConfig.MaxIdle,
		MaxActive:   redisConfig.MaxActive,
		Wait:        redisConfig.Wait,
		IdleTimeout: 30 * time.Second,
		Dial: func() (redis.Conn, error) {
			options := []redis.DialOption{
				redis.DialPassword(password),
				redis.DialDatabase(redisConfig.Db),
			}
			if username != "" {
				options = append([]redis.DialOption{redis.DialUsername(username)}, options...)
			}
			conn, err := redis.Dial("tcp", fmt.Sprintf("%s:%d", redisConfig.Host, redisConfig.Port), options...)
			if err != nil {
				logrus.Errorf("Redis初始化失败 err: %v", err)
				panic(err)
			}
			result, err := redis.String(conn.Do("PING"))
			if err != nil || result != "PONG" {
				conn.Close()
				logrus.Errorf("Redis连接失败 err: %v", err)
				panic(err)
			}
			return conn, nil
		},
	}
}

func CloseRedis() {
	if pool != nil {
		if err := pool.Close(); err != nil {
			logrus.Errorf("redis close err: %v", err)
		}
	}
	if authPool != nil {
		if err := authPool.Close(); err != nil {
			logrus.Errorf("redis auth close err: %v", err)
		}
	}
}
