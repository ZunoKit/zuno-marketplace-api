package middleware

import (
	"context"
	"net/http"
	"strings"
)

// ContextKey type for context keys
type ContextKey string

const (
	// ResponseWriterKey is the context key for ResponseWriter
	ResponseWriterKey ContextKey = "response_writer"
	// RequestKey is the context key for HTTP Request
	RequestKey ContextKey = "http_request"
)

// CookieMiddleware adds ResponseWriter and Request to context for cookie handling
func CookieMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add ResponseWriter and updated Request to context so resolvers can access them
		ctxWithRW := context.WithValue(r.Context(), ResponseWriterKey, w)
		r = r.WithContext(ctxWithRW)
		ctxWithReq := context.WithValue(r.Context(), RequestKey, r)
		r = r.WithContext(ctxWithReq)

		next.ServeHTTP(w, r)
	})
}

// GetResponseWriter retrieves ResponseWriter from context
func GetResponseWriter(ctx context.Context) http.ResponseWriter {
	if rw, ok := ctx.Value(ResponseWriterKey).(http.ResponseWriter); ok {
		return rw
	}
	return nil
}

// GetRequest retrieves HTTP Request from context
func GetRequest(ctx context.Context) *http.Request {
	if req, ok := ctx.Value(RequestKey).(*http.Request); ok {
		return req
	}
	return nil
}

// SetRefreshTokenCookie sets httpOnly cookie for refresh token
func SetRefreshTokenCookie(w http.ResponseWriter, refreshToken string) {
	cookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteStrictMode,
		MaxAge:   30 * 24 * 60 * 60, // 30 days
	}
	http.SetCookie(w, cookie)
}

// ClearRefreshTokenCookie clears the refresh token cookie
func ClearRefreshTokenCookie(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1, // Delete cookie
	}
	http.SetCookie(w, cookie)
}

// GetRefreshTokenFromCookie retrieves refresh token from httpOnly cookie
func GetRefreshTokenFromCookie(r *http.Request) string {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		return ""
	}
	return cookie.Value
}

// GetClientInfo extracts client information for audit logging
func GetClientInfo(r *http.Request) (ip, userAgent string) {
	// Get real IP (handle proxy headers)
	ip = r.Header.Get("X-Real-IP")
	if ip == "" {
		ip = r.Header.Get("X-Forwarded-For")
		if ip != "" {
			// X-Forwarded-For can contain multiple IPs, take the first one
			if idx := strings.Index(ip, ","); idx > 0 {
				ip = ip[:idx]
			}
		}
	}
	if ip == "" {
		ip = r.RemoteAddr
	}

	userAgent = r.Header.Get("User-Agent")
	return ip, userAgent
}
