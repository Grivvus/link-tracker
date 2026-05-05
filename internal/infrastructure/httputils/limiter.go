package httputils

import (
	"context"
	"net"
	"net/http"
	"sync"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type RateLimiter struct {
	mu               sync.Mutex
	limiterPerClient map[string]*rate.Limiter
	limitN           int
	limitBurst       int
}

func NewRateLimiter(limitRPS, limitBurst int) *RateLimiter {
	return &RateLimiter{
		limitBurst:       limitBurst,
		limitN:           limitRPS,
		limiterPerClient: make(map[string]*rate.Limiter),
	}
}

func (r *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		limiter := r.getClientLimiter(getHTTPClientIP(req))
		if !limiter.Allow() {
			http.Error(w, "to many requests from this address", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, req)
	})
}

func (r *RateLimiter) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		clientIP := getGRPCClientIP(ctx)
		limiter := r.getClientLimiter(clientIP)

		if !limiter.Allow() {
			return nil, status.Error(codes.ResourceExhausted, "too many requests from this address")
		}

		return handler(ctx, req)
	}
}

func (r *RateLimiter) getClientLimiter(clientIP string) *rate.Limiter {
	r.mu.Lock()
	defer r.mu.Unlock()
	clientLimiter, ok := r.limiterPerClient[clientIP]
	if !ok {
		newLimiter := rate.NewLimiter(rate.Limit(r.limitN), r.limitBurst)
		clientLimiter = newLimiter
		r.limiterPerClient[clientIP] = newLimiter
	}
	return clientLimiter
}

func getHTTPClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func getGRPCClientIP(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return "unknown"
	}

	host, _, err := net.SplitHostPort(p.Addr.String())
	if err != nil {
		return p.Addr.String()
	}
	return host
}
