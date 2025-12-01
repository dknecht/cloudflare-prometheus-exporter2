// limiter/limiter.go
package limiter

import (
	"context"
	"time"

	"golang.org/x/time/rate"
)

// Cloudflare API rate limiter (4 requests/second with burst of 2)
var CloudflareLimiter = rate.NewLimiter(rate.Every(250*time.Millisecond), 2)

// Wait blocks until the limiter allows the request
func Wait(ctx context.Context) error {
	return CloudflareLimiter.Wait(ctx)
}
