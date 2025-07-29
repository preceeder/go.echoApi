package middlers

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"
	"log/slog"
	"time"
)

var (
	DefaultGcDuration   = time.Hour
	DefaultDumpFile     = true
	DefaultDumpFilePath = "rate/rate-%s.json"
)

// 2小时清理一次数据
func cleanupVisitors(limit *RateLimiterTrie) {
	for {
		time.Sleep(DefaultGcDuration)
		func() {
			defer func() {
				if err := recover(); err != nil {
					slog.Error("[RateLimit GC] Panic recovered", "error", err)
				}
			}()
			filePath := fmt.Sprintf(DefaultDumpFilePath, time.Now().Format("20060102150405"))
			limit.GC(DefaultDumpFile, filePath)
		}()
	}
}

func Init() *RateLimiterTrie {
	limit := NewSensitiveTrie()
	go cleanupVisitors(limit)
	return limit

}

func RateLimitMiddleware(before func(c echo.Context) (float64, int, []string),
	after func(c echo.Context, limit *rate.Limiter) error) echo.MiddlewareFunc {
	limit := Init()
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			//rateVal, ok := c.Get("Rate").(float64)
			//if !ok || rateVal == 0 {
			//	return next(c)
			//}
			//
			//burstVal, ok := c.Get("Burst").(int)
			//if !ok || burstVal == 0 {
			//	return next(c)
			//}
			//ip := c.RealIP()
			rateVal, burstVal, outKeys := before(c)
			if burstVal == 0 || rateVal == 0 {
				return next(c)
			}
			keys := []string{c.Path(), c.Request().Method}
			keys = append(keys, outKeys...)
			node, newAdd := limit.GetAdd(keys...)
			node.Data.lastSeen = time.Now()
			if newAdd {
				node.mu.Lock()
				node.Data.limit.SetLimit(rate.Limit(rateVal))
				node.Data.limit.SetBurst(burstVal)
				node.mu.Unlock()
			}
			// 本身有锁， 而且node的删除是在这个节点3分钟不在调用的情况下才会有， 基本不会有并发问题
			allowed := node.Data.limit.Allow()
			if !allowed {
				return after(c, node.Data.limit)
				//return c.JSON(429,
				//	map[string]any{"code": 429, "message": "请求太快了，请稍后再试"})
			}

			return next(c)
		}
	}

}
