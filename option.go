package updatecache

import "golang.org/x/time/rate"

type Option func(cache *LocalCache)

// WithRateLimit create a new Limiter that allows events up to rate r and permits
// bursts of at most b tokens.
// rate limiter Limit the execution frequency of GetValueFunc
func WithRateLimit(r float64, b int) Option {
	return func(c *LocalCache) {
		c.limiter = rate.NewLimiter(rate.Limit(r), b)
	}
}
