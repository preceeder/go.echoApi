package middlers

import (
	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"
	"log/slog"
	"time"
)

// 2小时清理一次数据
func cleanupVisitors() {
	for {
		time.Sleep(time.Hour * 2)
		func() {
			defer func() {
				if err := recover(); err != nil {
					slog.Error("[RateLimit GC] Panic recovered", "error", err)
				}
			}()
			limit.GC()
		}()
	}
}

var limit *RateLimiterTrie

func init() {
	limit = NewSensitiveTrie()
	go cleanupVisitors()

}

func RateLimitMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			rateVal, ok := c.Get("Rate").(float64)
			if !ok || rateVal == 0 {
				return next(c)
			}

			burstVal, ok := c.Get("Burst").(int)
			if !ok || burstVal == 0 {
				return next(c)
			}

			//ip := c.RealIP()
			node, newAdd := limit.GetAdd(c.Path(), c.RealIP())

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
				return c.JSON(429,
					map[string]any{"code": 429, "message": "请求太快了，请稍后再试"})
			}

			return next(c)
		}
	}

}
