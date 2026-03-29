package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// clientLimiter хранит лимитер для конкретного клиента
// и время его последней активности.
// Это нужно, чтобы:
// 1. ограничивать запросы отдельно для каждого IP
// 2. очищать неактивных клиентов из памяти
type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimitMiddleware создаёт middleware для ограничения количества запросов.
// requestsPerMinute — сколько запросов в минуту разрешено одному клиенту.
//
// Возвращаемое значение — стандартный chi/http middleware:
// func(http.Handler) http.Handler
func RateLimitMiddleware(requestsPerMinute int) func(http.Handler) http.Handler {
	// Если в конфиге пришло некорректное значение (0 или меньше),
	// ставим безопасный дефолт.
	if requestsPerMinute <= 0 {
		requestsPerMinute = 200
	}

	var (
		// mutex нужен, потому что map clients будет использоваться
		// одновременно из нескольких goroutine / HTTP-запросов.
		mu sync.Mutex

		// clients хранит лимитер на каждый IP-адрес клиента.
		// key   -> IP клиента
		// value -> его лимитер и время последней активности
		clients = make(map[string]*clientLimiter)

		// rps = requests per second.
		// golang.org/x/time/rate работает в запросах в секунду,
		// поэтому переводим "запросы в минуту" в "запросы в секунду".
		rps = rate.Limit(float64(requestsPerMinute) / 60)

		// burst — максимальный "всплеск" запросов,
		// который можно пропустить мгновенно.
		// Здесь разрешаем burst равный минутному лимиту.
		burst = requestsPerMinute
	)

	// Фоновая горутина для очистки старых клиентов.
	// Раз в минуту проверяем map clients и удаляем тех,
	// кто не делал запросы более 3 минут.
	// Это защищает от бесконечного роста map в памяти.
	go func() {
		for {
			time.Sleep(time.Minute)

			mu.Lock()
			for ip, client := range clients {
				if time.Since(client.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	// getLimiter возвращает лимитер для конкретного IP.
	// Если лимитера ещё нет — создаёт новый.
	getLimiter := func(ip string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()

		client, exists := clients[ip]
		if !exists {
			// Для нового клиента создаём собственный limiter.
			limiter := rate.NewLimiter(rps, burst)
			clients[ip] = &clientLimiter{
				limiter:  limiter,
				lastSeen: time.Now(),
			}
			return limiter
		}

		// Если клиент уже есть — обновляем время последней активности
		// и возвращаем его текущий limiter.
		client.lastSeen = time.Now()
		return client.limiter
	}

	// Возвращаем сам middleware.
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// r.RemoteAddr обычно приходит в формате "IP:port",
			// например "127.0.0.1:54321".
			// SplitHostPort отделяет IP от порта.
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				// Если по какой-то причине разобрать не удалось,
				// используем как есть.
				host = r.RemoteAddr
			}

			// Получаем limiter для текущего клиента.
			limiter := getLimiter(host)

			// Проверяем: можно ли пропустить запрос прямо сейчас.
			// Если лимит превышен — возвращаем 429 Too Many Requests.
			if !limiter.Allow() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded","details":"too many requests, please try again later"}`))
				return
			}

			// Если лимит не превышен — передаём запрос дальше.
			next.ServeHTTP(w, r)
		})
	}
}
