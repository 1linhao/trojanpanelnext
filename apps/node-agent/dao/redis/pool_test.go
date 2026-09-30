package redis

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	redisgo "github.com/gomodule/redigo/redis"
	"trojan-panel-core/core"
)

// This RESP fixture exercises the real Redigo Dial path, including Redis PING
// rejection after a failed snapshot. It can recover without replacing the pool.
func redisFixture(t *testing.T, mode *atomic.Int32) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			c, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				r := bufio.NewReader(c)
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					count, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "*")))
					if err != nil || count < 1 {
						return
					}
					args := make([]string, count)
					for i := range args {
						header, err := r.ReadString('\n')
						if err != nil {
							return
						}
						size, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(header, "$")))
						if err != nil || size < 0 {
							return
						}
						b := make([]byte, size+2)
						if _, err = io.ReadFull(r, b); err != nil {
							return
						}
						args[i] = string(b[:size])
					}
					reply := "+OK\r\n"
					switch strings.ToUpper(args[0]) {
					case "PING":
						switch mode.Load() {
						case 0:
							reply = "-MISCONF snapshot failure fixture\r\n"
						case 1:
							reply = "+UNEXPECTED\r\n"
						default:
							reply = "+PONG\r\n"
						}
					case "GET":
						reply = "$5\r\nready\r\n"
					}
					if _, err = io.WriteString(c, reply); err != nil {
						return
					}
				}
			}()
		}
	}()
	return listener.Addr().String()
}

func configurePool(t *testing.T, address string) {
	t.Helper()
	oldConfig, oldPool, oldRS := core.Config.RedisConfig, pool, rs
	host, p, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		t.Fatal(err)
	}
	core.Config.RedisConfig = core.RedisConfig{Host: host, Port: port, MaxActive: 1, Wait: true}
	InitRedis()
	t.Cleanup(func() { CloseRedis(); pool = oldPool; rs = oldRS; core.Config.RedisConfig = oldConfig })
}

func borrow(ctx context.Context) (c redisgo.Conn, err error, panicked bool) {
	completed := false
	defer func() {
		if !completed {
			_ = recover()
			panicked = true
		}
	}()
	c, err = pool.GetContext(ctx)
	completed = true
	return
}

func assertFailedDialReleasesCapacity(t *testing.T) {
	t.Helper()
	for i := 0; i < 4; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		c, err, panicked := borrow(ctx)
		timedOut := ctx.Err() != nil
		cancel()
		if panicked {
			t.Fatal("Redis failure panicked inside pool Dial and leaked its capacity")
		}
		if c != nil {
			c.Close()
		}
		if err == nil {
			t.Fatal("Redis failure was not returned")
		}
		if timedOut {
			t.Fatal("failed dial exhausted the Redis connection pool")
		}
		if pool.Stats().ActiveCount != 0 {
			t.Fatal("failed dial retained an active pool slot")
		}
	}
}

func TestPoolRecoversAfterRedisPingFailure(t *testing.T) {
	for _, modeValue := range []int32{0, 1} {
		t.Run(fmt.Sprintf("ping_response_%d", modeValue), func(t *testing.T) {
			var mode atomic.Int32
			mode.Store(modeValue)
			configurePool(t, redisFixture(t, &mode))
			assertFailedDialReleasesCapacity(t)
			mode.Store(2)
			value, err := Client.String.Get("fixture").String()
			if err != nil || value != "ready" {
				t.Fatalf("same pool did not recover: value=%q err=%v", value, err)
			}
		})
	}
}

func TestPoolDialFailureReturnsErrorWithoutLeakingCapacity(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	configurePool(t, address)
	assertFailedDialReleasesCapacity(t)
}
