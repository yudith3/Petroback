package middleware

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

var errTooManyRequests = myerror.New(myerror.TypeAuthorization, "TOO_MANY_REQUESTS", "too many requests, please try again later")

// Entri kedaluwarsa dibersihkan setelah map sebesar ini, supaya memori tidak
// tumbuh terus oleh IP yang hanya datang sekali.
const rateLimitSweepSize = 10_000

type rateLimitCounter struct {
	count   int
	resetAt time.Time
}

// RateLimit membatasi request per IP dengan fixed window. Hitungan disimpan
// di memori, jadi berlaku per instance backend dan hilang saat restart.
// Pasang satu handler yang sama di beberapa route kalau batasnya mau dibagi.
func (m Middleware) RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	counters := make(map[string]*rateLimitCounter)

	return func(ctx *gin.Context) {
		now := time.Now()
		key := ctx.ClientIP()

		mu.Lock()
		if len(counters) >= rateLimitSweepSize {
			for ip, counter := range counters {
				if !now.Before(counter.resetAt) {
					delete(counters, ip)
				}
			}
		}
		counter, ok := counters[key]
		if !ok || !now.Before(counter.resetAt) {
			counter = &rateLimitCounter{resetAt: now.Add(window)}
			counters[key] = counter
		}
		counter.count++
		allowed := counter.count <= limit
		retryAfter := counter.resetAt.Sub(now)
		mu.Unlock()

		if !allowed {
			ctx.Header("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
			response.BuildResponseFailed("too many requests", errTooManyRequests).
				SetStatus(http.StatusTooManyRequests).
				SendWithAbort(ctx)
			return
		}
		ctx.Next()
	}
}
