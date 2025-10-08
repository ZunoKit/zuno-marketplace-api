package utils

import (
	"fmt"
	"mime"
	"path/filepath"
	"strings"
)

// AllowedMediaTypes contains the supported file types and their MIME types
var AllowedMediaTypes = map[string][]string{
	"image": {
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/gif",
		"image/webp",
		"image/svg+xml",
		"image/bmp",
		"image/tiff",
	},
	"video": {
		"video/mp4",
		"video/mpeg",
		"video/quicktime",
		"video/webm",
		"video/x-msvideo", // AVI
		"video/x-ms-wmv",  // WMV
	},
	"audio": {
		"audio/mpeg", // MP3
		"audio/wav",
		"audio/wave",
		"audio/x-wav",
		"audio/ogg",
		"audio/webm",
		"audio/aac",
		"audio/mp4", // M4A
		"audio/x-m4a",
	},
	"document": {
		"application/pdf",
		"application/json",
		"text/plain",
		"text/markdown",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",   // DOCX
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",         // XLSX
		"application/vnd.openxmlformats-officedocument.presentationml.presentation", // PPTX
	},
	"3d": {
		"model/gltf-binary",     // GLB
		"model/gltf+json",       // GLTF
		"application/x-blender", // Blender
		"model/obj",             // OBJ
		"model/fbx",             // FBX
	},
}

// AllowedExtensions maps file extensions to MIME types
var AllowedExtensions = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
	".bmp":  "image/bmp",
	".tiff": "image/tiff",
	".tif":  "image/tiff",

	".mp4":  "video/mp4",
	".mpeg": "video/mpeg",
	".mpg":  "video/mpeg",
	".mov":  "video/quicktime",
	".webm": "video/webm",
	".avi":  "video/x-msvideo",
	".wmv":  "video/x-ms-wmv",

	".mp3": "audio/mpeg",
	".wav": "audio/wav",
	".ogg": "audio/ogg",
	".aac": "audio/aac",
	".m4a": "audio/mp4",

	".pdf":  "application/pdf",
	".json": "application/json",
	".txt":  "text/plain",
	".md":   "text/markdown",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",

	".glb":   "model/gltf-binary",
	".gltf":  "model/gltf+json",
	".blend": "application/x-blender",
	".obj":   "model/obj",
	".fbx":   "model/fbx",
}

// MaxFileSizes defines maximum file size per media type (in bytes)
var MaxFileSizes = map[string]int64{
	"image":    100 * 1024 * 1024, // 100MB
	"video":    500 * 1024 * 1024, // 500MB
	"audio":    50 * 1024 * 1024,  // 50MB
	"document": 50 * 1024 * 1024,  // 50MB
	"3d":       200 * 1024 * 1024, // 200MB
	"default":  100 * 1024 * 1024, // 100MB default
}

// ValidateFileType validates if a file type is allowed
func ValidateFileType(filename string, mimeType string) error {
	// First, try to validate by MIME type
	if mimeType != "" && mimeType != "application/octet-stream" {
		if isAllowedMimeType(mimeType) {
			return nil
		}
	}

	// Fall back to extension-based validation
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return fmt.Errorf("file has no extension and unknown MIME type")
	}

	if _, ok := AllowedExtensions[ext]; !ok {
		return fmt.Errorf("file type %s is not allowed", ext)
	}

	return nil
}

// ValidateFileSize validates if file size is within limits
func ValidateFileSize(size int64, mediaType string) error {
	maxSize, exists := MaxFileSizes[mediaType]
	if !exists {
		maxSize = MaxFileSizes["default"]
	}

	if size > maxSize {
		return fmt.Errorf("file size %d bytes exceeds maximum allowed size of %d bytes for %s",
			size, maxSize, mediaType)
	}

	if size <= 0 {
		return fmt.Errorf("invalid file size: %d bytes", size)
	}

	return nil
}

// GetMediaType returns the media type category for a MIME type
func GetMediaType(mimeType string) string {
	mimeType = strings.ToLower(mimeType)

	for category, types := range AllowedMediaTypes {
		for _, t := range types {
			if t == mimeType {
				return category
			}
		}
	}

	// Try to determine from MIME type prefix
	if strings.HasPrefix(mimeType, "image/") {
		return "image"
	} else if strings.HasPrefix(mimeType, "video/") {
		return "video"
	} else if strings.HasPrefix(mimeType, "audio/") {
		return "audio"
	} else if strings.HasPrefix(mimeType, "model/") {
		return "3d"
	} else if strings.HasPrefix(mimeType, "text/") || strings.HasPrefix(mimeType, "application/") {
		return "document"
	}

	return "other"
}

// GetMimeTypeFromExtension returns MIME type for a file extension
func GetMimeTypeFromExtension(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if mimeType, ok := AllowedExtensions[ext]; ok {
		return mimeType
	}

	// Try using Go's built-in mime package
	mimeType := mime.TypeByExtension(ext)
	if mimeType != "" {
		// Remove charset and other parameters
		if idx := strings.IndexByte(mimeType, ';'); idx != -1 {
			mimeType = mimeType[:idx]
		}
		return strings.TrimSpace(mimeType)
	}

	return "application/octet-stream"
}

// isAllowedMimeType checks if a MIME type is in the allowed list
func isAllowedMimeType(mimeType string) bool {
	mimeType = strings.ToLower(mimeType)

	// Remove charset and other parameters
	if idx := strings.IndexByte(mimeType, ';'); idx != -1 {
		mimeType = mimeType[:idx]
	}
	mimeType = strings.TrimSpace(mimeType)

	for _, types := range AllowedMediaTypes {
		for _, t := range types {
			if t == mimeType {
				return true
			}
		}
	}

	return false
}

// SanitizeFilename removes potentially dangerous characters from a filename
func SanitizeFilename(filename string) string {
	// Remove path separators
	filename = filepath.Base(filename)

	// Replace spaces with underscores
	filename = strings.ReplaceAll(filename, " ", "_")

	// Remove or replace potentially problematic characters
	replacer := strings.NewReplacer(
		"..", "_",
		"/", "_",
		"\\", "_",
		"<", "_",
		">", "_",
		":", "_",
		"\"", "_",
		"|", "_",
		"?", "_",
		"*", "_",
		"\x00", "_", // null character
	)

	filename = replacer.Replace(filename)

	// Ensure the filename is not empty
	if filename == "" || filename == "." {
		filename = "file"
	}

	// Limit filename length
	const maxLength = 255
	if len(filename) > maxLength {
		ext := filepath.Ext(filename)
		base := strings.TrimSuffix(filename, ext)
		if len(base) > maxLength-len(ext) {
			base = base[:maxLength-len(ext)]
		}
		filename = base + ext
	}

	return filename
}

// ValidateImageDimensions validates image dimensions
func ValidateImageDimensions(width, height uint32) error {
	const maxDimension = 10000 // Maximum 10k pixels per side
	const minDimension = 1     // Minimum 1 pixel

	if width < minDimension || height < minDimension {
		return fmt.Errorf("image dimensions too small: %dx%d", width, height)
	}

	if width > maxDimension || height > maxDimension {
		return fmt.Errorf("image dimensions too large: %dx%d (max %d)", width, height, maxDimension)
	}

	return nil
}

// IsImageMimeType checks if a MIME type represents an image
func IsImageMimeType(mimeType string) bool {
	return GetMediaType(mimeType) == "image"
}

// IsVideoMimeType checks if a MIME type represents a video
func IsVideoMimeType(mimeType string) bool {
	return GetMediaType(mimeType) == "video"
}

// IsAudioMimeType checks if a MIME type represents audio
func IsAudioMimeType(mimeType string) bool {
	return GetMediaType(mimeType) == "audio"
}
