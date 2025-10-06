package redis

import (
	"strings"
)

var (
	App     = "nftmp" // project code
	Env     = "dev"   // dev|stg|prod
	Version = "v1"    // schema version for easy bust
)

func join(parts ...string) string {
	return strings.Join(parts, ":")
}

func pfx() string {
	return join(App, Env, Version)
}

// === Helpers chuẩn hoá ===
func NormalizeAddress(addr string) string { return strings.ToLower(addr) }

// CAIP-2 nên để lowercase toàn bộ namespace; phần reference giữ nguyên nếu là số.
func NormalizeChainID(chainID string) string { return strings.ToLower(chainID) }
