package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
)

// DeviceFingerprint represents a unique device fingerprint
type DeviceFingerprint struct {
	IP             string
	UserAgent      string
	AcceptHeader   string
	Language       string
	Encoding       string
	DNT            string
	Platform       string // From User-Agent parsing
	ScreenInfo     string // From client-side JS if available
	TimezoneOffset string // From client-side JS if available
	Hash           string // SHA256 hash of combined attributes
}

// FingerprintConfig defines configuration for fingerprinting
type FingerprintConfig struct {
	RequireExactMatch  bool // Require exact fingerprint match
	AllowIPChange      bool // Allow IP changes (for mobile users)
	AllowMinorUAChange bool // Allow minor User-Agent changes (browser updates)
	IPTrustProxies     bool // Trust X-Forwarded-For header
	MaxIPDistance      int  // Maximum allowed IP distance (for subnets)
	StrictMode         bool // Strict mode for high-security environments
}

// DefaultConfig returns default fingerprint configuration
func DefaultConfig() *FingerprintConfig {
	return &FingerprintConfig{
		RequireExactMatch:  false,
		AllowIPChange:      true,
		AllowMinorUAChange: true,
		IPTrustProxies:     true,
		MaxIPDistance:      24, // Same /24 subnet
		StrictMode:         false,
	}
}

// Generator generates device fingerprints
type Generator struct {
	config *FingerprintConfig
}

// NewGenerator creates a new fingerprint generator
func NewGenerator(config *FingerprintConfig) *Generator {
	if config == nil {
		config = DefaultConfig()
	}
	return &Generator{config: config}
}

// Generate creates a fingerprint from HTTP request
func (g *Generator) Generate(r *http.Request) *DeviceFingerprint {
	fp := &DeviceFingerprint{
		IP:           g.extractIP(r),
		UserAgent:    r.Header.Get("User-Agent"),
		AcceptHeader: r.Header.Get("Accept"),
		Language:     r.Header.Get("Accept-Language"),
		Encoding:     r.Header.Get("Accept-Encoding"),
		DNT:          r.Header.Get("DNT"),
	}

	// Parse platform from User-Agent
	fp.Platform = g.extractPlatform(fp.UserAgent)

	// Get client-side data if available (from headers set by frontend)
	fp.ScreenInfo = r.Header.Get("X-Screen-Info")
	fp.TimezoneOffset = r.Header.Get("X-Timezone-Offset")

	// Generate hash
	fp.Hash = g.generateHash(fp)

	return fp
}

// extractIP extracts the real IP address
func (g *Generator) extractIP(r *http.Request) string {
	// Check X-Forwarded-For if we trust proxies
	if g.config.IPTrustProxies {
		xff := r.Header.Get("X-Forwarded-For")
		if xff != "" {
			// Get first IP in the chain
			ips := strings.Split(xff, ",")
			if len(ips) > 0 {
				ip := strings.TrimSpace(ips[0])
				if net.ParseIP(ip) != nil {
					return ip
				}
			}
		}

		// Check X-Real-IP
		xri := r.Header.Get("X-Real-IP")
		if xri != "" && net.ParseIP(xri) != nil {
			return xri
		}
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// extractPlatform extracts platform info from User-Agent
func (g *Generator) extractPlatform(ua string) string {
	ua = strings.ToLower(ua)

	switch {
	case strings.Contains(ua, "windows"):
		return "windows"
	case strings.Contains(ua, "macintosh") || strings.Contains(ua, "mac os"):
		return "macos"
	case strings.Contains(ua, "linux"):
		return "linux"
	case strings.Contains(ua, "android"):
		return "android"
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad"):
		return "ios"
	default:
		return "unknown"
	}
}

// generateHash generates a unique hash for the fingerprint
func (g *Generator) generateHash(fp *DeviceFingerprint) string {
	// Combine relevant fields for hashing
	var parts []string

	if !g.config.AllowIPChange {
		parts = append(parts, fp.IP)
	} else if g.config.MaxIPDistance > 0 {
		// Use subnet instead of exact IP
		parts = append(parts, g.getSubnet(fp.IP, g.config.MaxIPDistance))
	}

	if g.config.AllowMinorUAChange {
		// Use only major browser version
		parts = append(parts, g.getMajorBrowserVersion(fp.UserAgent))
	} else {
		parts = append(parts, fp.UserAgent)
	}

	// Always include these
	parts = append(parts,
		fp.AcceptHeader,
		fp.Language,
		fp.Platform,
	)

	// Include optional fields if in strict mode
	if g.config.StrictMode {
		parts = append(parts,
			fp.Encoding,
			fp.DNT,
			fp.ScreenInfo,
			fp.TimezoneOffset,
		)
	}

	// Generate SHA256 hash
	combined := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:])
}

// getSubnet returns the subnet for an IP
func (g *Generator) getSubnet(ip string, bits int) string {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return ip
	}

	// Create subnet mask
	if parsedIP.To4() != nil {
		// IPv4
		mask := net.CIDRMask(bits, 32)
		network := parsedIP.Mask(mask)
		return network.String()
	} else {
		// IPv6
		mask := net.CIDRMask(bits, 128)
		network := parsedIP.Mask(mask)
		return network.String()
	}
}

// getMajorBrowserVersion extracts major browser version from User-Agent
func (g *Generator) getMajorBrowserVersion(ua string) string {
	ua = strings.ToLower(ua)

	// Common browser patterns
	patterns := []struct {
		name    string
		pattern string
	}{
		{"chrome", "chrome/"},
		{"firefox", "firefox/"},
		{"safari", "version/"},
		{"edge", "edg/"},
		{"opera", "opr/"},
	}

	for _, p := range patterns {
		if idx := strings.Index(ua, p.pattern); idx != -1 {
			// Extract version number
			start := idx + len(p.pattern)
			end := strings.IndexAny(ua[start:], " ;)")
			if end == -1 {
				end = len(ua[start:])
			}
			version := ua[start : start+end]

			// Get major version only
			if dotIdx := strings.Index(version, "."); dotIdx != -1 {
				version = version[:dotIdx]
			}

			return p.name + "/" + version
		}
	}

	return "unknown"
}

// Validator validates fingerprints
type Validator struct {
	config *FingerprintConfig
}

// NewValidator creates a new fingerprint validator
func NewValidator(config *FingerprintConfig) *Validator {
	if config == nil {
		config = DefaultConfig()
	}
	return &Validator{config: config}
}

// Validate checks if two fingerprints match according to config
func (v *Validator) Validate(stored, current *DeviceFingerprint) (bool, string) {
	// Exact match mode
	if v.config.RequireExactMatch {
		if stored.Hash != current.Hash {
			return false, "fingerprint hash mismatch"
		}
		return true, ""
	}

	// Check IP if required
	if !v.config.AllowIPChange {
		if stored.IP != current.IP {
			// Check if in same subnet
			if v.config.MaxIPDistance > 0 {
				storedSubnet := v.getSubnet(stored.IP, v.config.MaxIPDistance)
				currentSubnet := v.getSubnet(current.IP, v.config.MaxIPDistance)
				if storedSubnet != currentSubnet {
					return false, fmt.Sprintf("IP changed from different subnet: %s -> %s", stored.IP, current.IP)
				}
			} else {
				return false, fmt.Sprintf("IP changed: %s -> %s", stored.IP, current.IP)
			}
		}
	}

	// Check User-Agent
	if !v.config.AllowMinorUAChange {
		if stored.UserAgent != current.UserAgent {
			return false, "user-agent changed"
		}
	} else {
		// Allow minor version changes
		if v.getMajorBrowserVersion(stored.UserAgent) != v.getMajorBrowserVersion(current.UserAgent) {
			return false, "major browser change detected"
		}
	}

	// Check platform (should rarely change)
	if stored.Platform != current.Platform {
		return false, fmt.Sprintf("platform changed: %s -> %s", stored.Platform, current.Platform)
	}

	// In strict mode, check additional fields
	if v.config.StrictMode {
		if stored.AcceptHeader != current.AcceptHeader {
			return false, "accept header changed"
		}
		if stored.Language != current.Language {
			return false, "language changed"
		}
		if stored.ScreenInfo != current.ScreenInfo && stored.ScreenInfo != "" {
			return false, "screen info changed"
		}
	}

	return true, ""
}

// getSubnet returns the subnet for an IP (reused from Generator)
func (v *Validator) getSubnet(ip string, bits int) string {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return ip
	}

	if parsedIP.To4() != nil {
		mask := net.CIDRMask(bits, 32)
		network := parsedIP.Mask(mask)
		return network.String()
	} else {
		mask := net.CIDRMask(bits, 128)
		network := parsedIP.Mask(mask)
		return network.String()
	}
}

// getMajorBrowserVersion (reused from Generator)
func (v *Validator) getMajorBrowserVersion(ua string) string {
	ua = strings.ToLower(ua)

	patterns := []struct {
		name    string
		pattern string
	}{
		{"chrome", "chrome/"},
		{"firefox", "firefox/"},
		{"safari", "version/"},
		{"edge", "edg/"},
		{"opera", "opr/"},
	}

	for _, p := range patterns {
		if idx := strings.Index(ua, p.pattern); idx != -1 {
			start := idx + len(p.pattern)
			end := strings.IndexAny(ua[start:], " ;)")
			if end == -1 {
				end = len(ua[start:])
			}
			version := ua[start : start+end]

			if dotIdx := strings.Index(version, "."); dotIdx != -1 {
				version = version[:dotIdx]
			}

			return p.name + "/" + version
		}
	}

	return "unknown"
}

// CalculateSimilarity calculates similarity score between two fingerprints
func (v *Validator) CalculateSimilarity(fp1, fp2 *DeviceFingerprint) float64 {
	score := 0.0
	total := 0.0

	// IP similarity (30% weight)
	total += 30.0
	if fp1.IP == fp2.IP {
		score += 30.0
	} else if v.getSubnet(fp1.IP, 24) == v.getSubnet(fp2.IP, 24) {
		score += 15.0 // Same /24 subnet
	}

	// User-Agent similarity (25% weight)
	total += 25.0
	if fp1.UserAgent == fp2.UserAgent {
		score += 25.0
	} else if v.getMajorBrowserVersion(fp1.UserAgent) == v.getMajorBrowserVersion(fp2.UserAgent) {
		score += 15.0 // Same browser, different version
	}

	// Platform (20% weight)
	total += 20.0
	if fp1.Platform == fp2.Platform {
		score += 20.0
	}

	// Headers (15% weight)
	total += 15.0
	if fp1.AcceptHeader == fp2.AcceptHeader {
		score += 5.0
	}
	if fp1.Language == fp2.Language {
		score += 5.0
	}
	if fp1.Encoding == fp2.Encoding {
		score += 5.0
	}

	// Additional info (10% weight)
	total += 10.0
	if fp1.ScreenInfo == fp2.ScreenInfo && fp1.ScreenInfo != "" {
		score += 5.0
	}
	if fp1.TimezoneOffset == fp2.TimezoneOffset && fp1.TimezoneOffset != "" {
		score += 5.0
	}

	return (score / total) * 100.0
}
