package test

import (
	"encoding/json"
	"goph-profile-avatars/internal/middleware"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type errorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

func TestRateLimitMiddleware_WithValidLimit(t *testing.T) {
	// Создаем middleware с лимитом 60 запросов в минуту (1 запрос в секунду)
	limitMiddleware := middleware.RateLimitMiddleware(60)

	// Создаем тестовый обработчик
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	handler := limitMiddleware(testHandler)

	// Тестовый IP
	remoteAddr := "192.168.1.1:12345"

	// Первые 2 запроса должны пройти
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = remoteAddr
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "OK", rr.Body.String())
	}

	// Ждем 1 секунду чтобы пополнить токены
	time.Sleep(1100 * time.Millisecond)

	// Следующий запрос должен пройти
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = remoteAddr
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRateLimitMiddleware_ExceedsLimit(t *testing.T) {
	// Создаем middleware с лимитом 3 запроса в минуту (0.05 запросов в секунду)
	limitMiddleware := middleware.RateLimitMiddleware(3)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	handler := limitMiddleware(testHandler)
	remoteAddr := "192.168.1.2:12345"

	// Делаем 4 запроса подряд
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = remoteAddr
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if i < 3 {
			assert.Equal(t, http.StatusOK, rr.Code, "Request %d should pass", i+1)
		} else {
			assert.Equal(t, http.StatusTooManyRequests, rr.Code, "Request 4 should be rate limited")

			var response errorResponse
			err := json.NewDecoder(rr.Body).Decode(&response)
			require.NoError(t, err)
			assert.Contains(t, response.Error, "rate limit exceeded")
		}
	}
}

func TestRateLimitMiddleware_DifferentIPs(t *testing.T) {
	limitMiddleware := middleware.RateLimitMiddleware(2) // 2 запроса в минуту

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	handler := limitMiddleware(testHandler)

	// Разные IP адреса
	ips := []string{"192.168.1.1:12345", "192.168.1.2:12345", "192.168.1.3:12345"}

	// Каждый IP делает по 3 запроса
	for _, ip := range ips {
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = ip
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if i < 2 {
				assert.Equal(t, http.StatusOK, rr.Code, "IP %s request %d should pass", ip, i+1)
			} else {
				assert.Equal(t, http.StatusTooManyRequests, rr.Code, "IP %s request 3 should be limited", ip)
			}
		}
	}
}

func TestRateLimitMiddleware_InvalidRemoteAddr(t *testing.T) {
	limitMiddleware := middleware.RateLimitMiddleware(5)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	handler := limitMiddleware(testHandler)

	// Невалидный RemoteAddr (без порта)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "invalid-addr"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Должен пройти, т.к. использует RemoteAddr как есть
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRateLimitMiddleware_DefaultLimit(t *testing.T) {
	// Передаем некорректное значение (0 или отрицательное)
	limitMiddleware := middleware.RateLimitMiddleware(0)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	handler := limitMiddleware(testHandler)
	remoteAddr := "192.168.1.4:12345"

	// Должен работать с дефолтным лимитом 200 запросов в минуту
	// Делаем 250 запросов, но из-за burst могут пройти все
	for i := 0; i < 250; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = remoteAddr
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		// Не проверяем статус, просто убеждаемся что не падает
		assert.NotEqual(t, http.StatusInternalServerError, rr.Code)
	}
}

func TestRateLimitMiddleware_ConcurrentRequests(t *testing.T) {
	limitMiddleware := middleware.RateLimitMiddleware(100)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	handler := limitMiddleware(testHandler)

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	// Запускаем 50 параллельных запросов с одного IP
	remoteAddr := "192.168.1.5:12345"

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = remoteAddr
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			mu.Lock()
			if rr.Code == http.StatusOK {
				successCount++
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	// Некоторые запросы могут быть ограничены, но не все
	assert.Greater(t, successCount, 0)
	assert.LessOrEqual(t, successCount, 100) // burst = 100
}

func TestRateLimitMiddleware_BurstHandling(t *testing.T) {
	// Создаем middleware с burst равным лимиту
	limitMiddleware := middleware.RateLimitMiddleware(5) // 5 запросов в минуту

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	handler := limitMiddleware(testHandler)
	remoteAddr := "192.168.1.6:12345"

	// Делаем burst запросов (должны пройти все)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = remoteAddr
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "Burst request %d should pass", i+1)
	}

	// 6-й запрос должен быть ограничен
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = remoteAddr
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestRateLimitMiddleware_ResponseFormat(t *testing.T) {
	limitMiddleware := middleware.RateLimitMiddleware(1)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := limitMiddleware(testHandler)
	remoteAddr := "192.168.1.7:12345"

	// Первый запрос проходит
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = remoteAddr
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusOK, rr1.Code)

	// Второй запрос ограничен
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = remoteAddr
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	assert.Equal(t, http.StatusTooManyRequests, rr2.Code)
	assert.Equal(t, "application/json", rr2.Header().Get("Content-Type"))

	var response errorResponse
	err := json.NewDecoder(rr2.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "rate limit exceeded", response.Error)
	assert.Equal(t, "too many requests, please try again later", response.Details)
}
