package middlers

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRateLimitMiddleware(t *testing.T) {
	e := echo.New()

	// 模拟一个限流中间件包装的路由
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "Success")
	}, RateLimitMiddleware(func(c echo.Context) (float64, int, []string) {
		rateVal, _ := c.Get("Rate").(float64)
		burstVal, _ := c.Get("Burst").(int)
		ip := c.RealIP()
		return rateVal, burstVal, []string{ip}
	}, func(c echo.Context, limit *rate.Limiter) error {
		fmt.Println("tokens", limit.Tokens())
		return c.JSON(429, map[string]any{"message": "太快了，请稍后再试"})
	}))

	// 模拟设置限流参数的中间件
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 每秒只允许1个请求，最多突发1个
			c.Set("Rate", float64(1))
			c.Set("Burst", 1)
			return next(c)
		}
	})

	// 发起第一次请求，应该成功
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec1 := httptest.NewRecorder()
	//c1 := e.NewContext(req1, rec1)
	req1.RemoteAddr = "1.2.3.4"
	//c1.SetRealIP("1.2.3.4") // 模拟客户端IP

	e.ServeHTTP(rec1, req1)
	assert.Equal(t, http.StatusOK, rec1.Code)
	assert.Equal(t, "Success", rec1.Body.String())

	// 发起第二次请求（立即连续），应触发限流
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec2 := httptest.NewRecorder()
	req1.RemoteAddr = "1.2.3.4"

	e.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusTooManyRequests, rec2.Code)

	// 等待1.1秒后重试，应成功
	time.Sleep(1100 * time.Millisecond)

	req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec3 := httptest.NewRecorder()
	e.ServeHTTP(rec3, req3)
	assert.Equal(t, http.StatusOK, rec3.Code)
}

func TestRateLimitMiddleware_Concurrent(t *testing.T) {
	e := echo.New()

	// 路由设置
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	}, RateLimitMiddleware(func(c echo.Context) (float64, int, []string) {
		rateVal, _ := c.Get("Rate").(float64)
		burstVal, _ := c.Get("Burst").(int)
		ip := c.RealIP()
		return rateVal, burstVal, []string{ip}
	}, func(c echo.Context, limit *rate.Limiter) error {
		fmt.Println("tokens", limit.Tokens())
		return c.JSON(429, map[string]any{"message": "太快了，请稍后再试"})

	}))

	// 注入 rate 和 burst
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("Rate", float64(5)) // 每秒5个
			c.Set("Burst", 5)         // 突发2个
			return next(c)
		}
	})

	var wg sync.WaitGroup
	successCount := int32(0)
	tooManyReqCount := int32(0)

	// 模拟并发 20 个请求
	concurrentReq := 20
	for i := 0; i < concurrentReq; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code == http.StatusOK {
				atomic.AddInt32(&successCount, 1)
			} else if rec.Code == http.StatusTooManyRequests {
				atomic.AddInt32(&tooManyReqCount, 1)
			} else {
				t.Errorf("unexpected status: %d", rec.Code)
			}
		}(i)
	}

	wg.Wait()
	t.Logf("Success: %d, RateLimited: %d", successCount, tooManyReqCount)
	assert.Greater(t, tooManyReqCount, int32(0)) // 应有部分被限流
}

func BenchmarkRateLimitMiddleware(b *testing.B) {
	e := echo.New()

	e.GET("/bench", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	}, RateLimitMiddleware(func(c echo.Context) (float64, int, []string) {
		rateVal, _ := c.Get("Rate").(float64)
		burstVal, _ := c.Get("Burst").(int)
		ip := c.RealIP()
		return rateVal, burstVal, []string{ip}
	}, func(c echo.Context, limit *rate.Limiter) error {
		//fmt.Println("tokens", limit.Limit(), limit.Tokens(), limit.Burst())
		return c.JSON(429, map[string]any{"message": "太快了，请稍后再试"})

	}))

	// 固定 rate/burst
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("Rate", float64(5)) // 每秒可以生成5个token
			c.Set("Burst", 5)         // 初始有5个token, 最大也只有5个
			return next(c)
		}
	})

	successCount := int32(0)
	tooManyReqCount := int32(0)

	req := httptest.NewRequest(http.MethodGet, "/bench", nil)
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			atomic.AddInt32(&successCount, 1)
		} else if rec.Code == http.StatusTooManyRequests {
			atomic.AddInt32(&tooManyReqCount, 1)
		} else {
			b.Errorf("unexpected status: %d", rec.Code)
		}
		//time.Sleep(10 * time.Millisecond)
	}
	fmt.Println("Success:", successCount, "RateLimited:", tooManyReqCount)

}
