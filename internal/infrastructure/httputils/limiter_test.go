package httputils_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/httputils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func TestRateLimiterMiddleware_AllowsRequestsWithinBurst(t *testing.T) {
	limiter := httputils.NewRateLimiter(1, 2)
	requestsHandled := 0
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestsHandled++
		w.WriteHeader(http.StatusOK)
	}))

	resp := serveHTTP(handler, "192.0.2.1:1234")
	require.Equal(t, http.StatusOK, resp.Code)

	resp = serveHTTP(handler, "192.0.2.1:1234")
	require.Equal(t, http.StatusOK, resp.Code)

	require.Equal(t, 2, requestsHandled)
}

func TestRateLimiterMiddleware_ReturnsTooManyRequestsWhenBurstExceeded(t *testing.T) {
	limiter := httputils.NewRateLimiter(1, 1)
	requestsHandled := 0
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestsHandled++
		w.WriteHeader(http.StatusOK)
	}))

	resp := serveHTTP(handler, "192.0.2.1:1234")
	require.Equal(t, http.StatusOK, resp.Code)

	resp = serveHTTP(handler, "192.0.2.1:1234")
	require.Equal(t, http.StatusTooManyRequests, resp.Code)

	require.Equal(t, 1, requestsHandled)
}

func TestRateLimiterMiddleware_UsesSeparateLimitersPerClientIP(t *testing.T) {
	limiter := httputils.NewRateLimiter(1, 1)
	requestsHandled := 0
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestsHandled++
		w.WriteHeader(http.StatusOK)
	}))

	resp := serveHTTP(handler, "192.0.2.1:1234")
	require.Equal(t, http.StatusOK, resp.Code)

	resp = serveHTTP(handler, "192.0.2.2:1234")
	require.Equal(t, http.StatusOK, resp.Code)

	resp = serveHTTP(handler, "192.0.2.1:5678")
	require.Equal(t, http.StatusTooManyRequests, resp.Code)

	require.Equal(t, 2, requestsHandled)
}

func TestRateLimiterMiddleware_RefillsTokensOverTime(t *testing.T) {
	limiter := httputils.NewRateLimiter(10, 1)
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	resp := serveHTTP(handler, "192.0.2.1:1234")
	require.Equal(t, http.StatusOK, resp.Code)

	resp = serveHTTP(handler, "192.0.2.1:1234")
	require.Equal(t, http.StatusTooManyRequests, resp.Code)

	time.Sleep(150 * time.Millisecond)

	resp = serveHTTP(handler, "192.0.2.1:1234")
	require.Equal(t, http.StatusOK, resp.Code)
}

func TestUnaryServerInterceptor_ReturnsResourceExhaustedWhenBurstExceeded(t *testing.T) {
	limiter := httputils.NewRateLimiter(1, 1)
	interceptor := limiter.UnaryServerInterceptor()
	handlerCalls := 0
	handler := func(context.Context, any) (any, error) {
		handlerCalls++
		return "ok", nil
	}
	ctx := contextWithPeer("192.0.2.1", 1234)

	resp, err := interceptor(ctx, "request", &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, handler)
	require.NoError(t, err)
	require.Equal(t, "ok", resp)

	resp, err = interceptor(ctx, "request", &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, handler)
	require.Nil(t, resp)
	require.Error(t, err)
	require.Equal(t, codes.ResourceExhausted, status.Code(err))
	require.Equal(t, 1, handlerCalls)
}

func serveHTTP(handler http.Handler, remoteAddr string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

func contextWithPeer(ip string, port int) context.Context {
	return peer.NewContext(context.Background(), &peer.Peer{
		Addr: &net.TCPAddr{
			IP:   net.ParseIP(ip),
			Port: port,
		},
	})
}
