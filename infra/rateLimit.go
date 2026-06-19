package infra

import (
	"log/slog"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	clients sync.Map
	rate    rate.Limit
	burst   int
}
type ClientLimiter struct {
	Limiter *rate.Limiter
	// LastSeen atomic.Int64
}

func NewRateLimiter(limitPerMinute int) *RateLimiter {
	return &RateLimiter{
		rate:    rate.Limit(float64(limitPerMinute) / 60.0),
		burst:   limitPerMinute,
		clients: sync.Map{},
	}

}
func (rl *RateLimiter) Update(limitPerMinute int) {
	newRate := rate.Limit(float64(limitPerMinute) / 60.0)
	newBurst := limitPerMinute

	rl.rate = newRate
	rl.burst = newBurst

	updated := 0

	rl.clients.Range(func(_, value any) bool {
		client := value.(*ClientLimiter)

		client.Limiter.SetLimit(newRate)
		client.Limiter.SetBurst(newBurst)

		updated++
		return true
	})

	slog.Info(
		"rate limiter updated",
		"limit_per_minute", limitPerMinute,
		"clients_updated", updated,
	)
}
func (rl *RateLimiter) Get(ip string) *rate.Limiter {
	if v, ok := rl.clients.Load(ip); ok {
		client := v.(*ClientLimiter)
		// client.LastSeen.Store(time.Now().Unix())
		return client.Limiter
	}

	client := &ClientLimiter{
		Limiter: rate.NewLimiter(rl.rate, rl.burst),
	}
	// client.LastSeen.Store(time.Now().Unix())

	actual, _ := rl.clients.LoadOrStore(ip, client)

	return actual.(*ClientLimiter).Limiter
}

func (rl *RateLimiter) Allow(ip string) bool {
	return rl.Get(ip).Allow()
}

func (rl *RateLimiter) Cleanup(ticker *time.Ticker) {
	for range ticker.C {
		before := 0
		deleted := 0

		rl.clients.Range(func(key, value any) bool {
			before++

			// client := value.(*ClientLimiter)

			// if time.Since(time.Unix(client.LastSeen.Load(), 0)) > time.Second*5 {
			rl.clients.Delete(key)
			deleted++
			// }

			return true
		})

		slog.Info(
			"Rate limiter cleanup",
			"before", before,
			"deleted", deleted,
			"after", before-deleted,
		)
	}
}
