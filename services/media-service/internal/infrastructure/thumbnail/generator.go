package thumbnail

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/quangdang46/NFT-Marketplace/services/media-service/internal/domain"
)

// ThumbnailSize represents a thumbnail size configuration
type ThumbnailSize struct {
	Name   string
	Width  int
	Height int
	Mode   imaging.ResampleFilter
}

// DefaultThumbnailSizes defines standard thumbnail sizes
var DefaultThumbnailSizes = []ThumbnailSize{
	{Name: "small", Width: 150, Height: 150, Mode: imaging.Lanczos},
	{Name: "medium", Width: 300, Height: 300, Mode: imaging.Lanczos},
	{Name: "large", Width: 600, Height: 600, Mode: imaging.Lanczos},
	{Name: "preview", Width: 1200, Height: 1200, Mode: imaging.Lanczos},
}

// Generator handles thumbnail generation for images
type Generator struct {
	sizes   []ThumbnailSize
	quality int
}

// NewGenerator creates a new thumbnail generator
func NewGenerator() *Generator {
	return &Generator{
		sizes:   DefaultThumbnailSizes,
		quality: 85, // JPEG quality
	}
}

// NewGeneratorWithSizes creates a generator with custom sizes
func NewGeneratorWithSizes(sizes []ThumbnailSize, quality int) *Generator {
	return &Generator{
		sizes:   sizes,
		quality: quality,
	}
}

// GenerateThumbnails generates multiple thumbnail sizes from an image
func (g *Generator) GenerateThumbnails(ctx context.Context, r io.Reader, mimeType string) ([]domain.ThumbnailResult, error) {
	// Decode the original image
	img, format, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Get original dimensions
	bounds := img.Bounds()
	originalWidth := bounds.Dx()
	originalHeight := bounds.Dy()

	results := make([]domain.ThumbnailResult, 0, len(g.sizes))

	for _, size := range g.sizes {
		// Skip if thumbnail would be larger than original
		if size.Width > originalWidth && size.Height > originalHeight {
			continue
		}

		// Generate thumbnail
		thumb := imaging.Fit(img, size.Width, size.Height, size.Mode)

		// Encode thumbnail
		buf := new(bytes.Buffer)
		if err := g.encodeImage(thumb, buf, format, mimeType); err != nil {
			return nil, fmt.Errorf("failed to encode thumbnail %s: %w", size.Name, err)
		}

		// Get thumbnail dimensions
		thumbBounds := thumb.Bounds()

		results = append(results, domain.ThumbnailResult{
			Name:   size.Name,
			Width:  uint32(thumbBounds.Dx()),
			Height: uint32(thumbBounds.Dy()),
			Format: format,
			Data:   buf.Bytes(),
			Size:   int64(buf.Len()),
		})
	}

	return results, nil
}

// GenerateSingleThumbnail generates a single thumbnail of specified size
func (g *Generator) GenerateSingleThumbnail(ctx context.Context, r io.Reader, width, height int, mimeType string) (*domain.ThumbnailResult, error) {
	// Decode the original image
	img, format, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Generate thumbnail
	thumb := imaging.Fit(img, width, height, imaging.Lanczos)

	// Encode thumbnail
	buf := new(bytes.Buffer)
	if err := g.encodeImage(thumb, buf, format, mimeType); err != nil {
		return nil, fmt.Errorf("failed to encode thumbnail: %w", err)
	}

	// Get thumbnail dimensions
	thumbBounds := thumb.Bounds()

	return &domain.ThumbnailResult{
		Name:   fmt.Sprintf("%dx%d", width, height),
		Width:  uint32(thumbBounds.Dx()),
		Height: uint32(thumbBounds.Dy()),
		Format: format,
		Data:   buf.Bytes(),
		Size:   int64(buf.Len()),
	}, nil
}

// GenerateSquareThumbnail generates a square thumbnail by center-cropping
func (g *Generator) GenerateSquareThumbnail(ctx context.Context, r io.Reader, size int, mimeType string) (*domain.ThumbnailResult, error) {
	// Decode the original image
	img, format, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Center crop to square
	thumb := imaging.Fill(img, size, size, imaging.Center, imaging.Lanczos)

	// Encode thumbnail
	buf := new(bytes.Buffer)
	if err := g.encodeImage(thumb, buf, format, mimeType); err != nil {
		return nil, fmt.Errorf("failed to encode thumbnail: %w", err)
	}

	return &domain.ThumbnailResult{
		Name:   fmt.Sprintf("square_%d", size),
		Width:  uint32(size),
		Height: uint32(size),
		Format: format,
		Data:   buf.Bytes(),
		Size:   int64(buf.Len()),
	}, nil
}

// encodeImage encodes an image to the specified format
func (g *Generator) encodeImage(img image.Image, w io.Writer, format string, mimeType string) error {
	// Normalize format string
	format = strings.ToLower(format)

	// Override format based on MIME type if needed
	if mimeType != "" {
		switch mimeType {
		case "image/jpeg", "image/jpg":
			format = "jpeg"
		case "image/png":
			format = "png"
		case "image/gif":
			format = "gif"
		case "image/webp":
			format = "webp"
		}
	}

	switch format {
	case "jpeg", "jpg":
		return jpeg.Encode(w, img, &jpeg.Options{Quality: g.quality})
	case "png":
		return png.Encode(w, img)
	case "gif":
		return gif.Encode(w, img, nil)
	case "webp":
		// WebP encoding requires additional library
		// For now, fallback to JPEG
		return jpeg.Encode(w, img, &jpeg.Options{Quality: g.quality})
	default:
		// Default to JPEG for unknown formats
		return jpeg.Encode(w, img, &jpeg.Options{Quality: g.quality})
	}
}

// OptimizeImage optimizes an image without changing dimensions
func (g *Generator) OptimizeImage(ctx context.Context, r io.Reader, mimeType string) (*domain.ThumbnailResult, error) {
	// Decode the original image
	img, format, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Re-encode with optimization
	buf := new(bytes.Buffer)
	if err := g.encodeImage(img, buf, format, mimeType); err != nil {
		return nil, fmt.Errorf("failed to encode optimized image: %w", err)
	}

	bounds := img.Bounds()

	return &domain.ThumbnailResult{
		Name:   "optimized",
		Width:  uint32(bounds.Dx()),
		Height: uint32(bounds.Dy()),
		Format: format,
		Data:   buf.Bytes(),
		Size:   int64(buf.Len()),
	}, nil
}

// GetImageDimensions returns the dimensions of an image without fully decoding it
func GetImageDimensions(r io.Reader) (width, height uint32, format string, err error) {
	config, format, err := image.DecodeConfig(r)
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to decode image config: %w", err)
	}

	return uint32(config.Width), uint32(config.Height), format, nil
}

// IsAnimatedGIF checks if a GIF is animated
func IsAnimatedGIF(r io.Reader) (bool, error) {
	g, err := gif.DecodeAll(r)
	if err != nil {
		return false, fmt.Errorf("failed to decode GIF: %w", err)
	}

	return len(g.Image) > 1, nil
}

// ExtractGIFFrame extracts a specific frame from an animated GIF
func ExtractGIFFrame(r io.Reader, frameIndex int) (image.Image, error) {
	g, err := gif.DecodeAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to decode GIF: %w", err)
	}

	if frameIndex >= len(g.Image) {
		frameIndex = 0 // Default to first frame if index is out of bounds
	}

	return g.Image[frameIndex], nil
}

// GenerateVideoThumbnail generates a thumbnail from a video file
// This is a placeholder - actual implementation would use ffmpeg or similar
func (g *Generator) GenerateVideoThumbnail(ctx context.Context, videoPath string, timestamp float64) (*domain.ThumbnailResult, error) {
	// This would typically use ffmpeg to extract a frame at the specified timestamp
	// Example command: ffmpeg -i video.mp4 -ss 00:00:05 -vframes 1 thumbnail.jpg

	return nil, fmt.Errorf("video thumbnail generation not yet implemented")
}

// GeneratePDFThumbnail generates a thumbnail from a PDF file
// This is a placeholder - actual implementation would use a PDF rendering library
func (g *Generator) GeneratePDFThumbnail(ctx context.Context, pdfPath string, pageNumber int) (*domain.ThumbnailResult, error) {
	// This would typically use a PDF library to render a specific page as an image

	return nil, fmt.Errorf("PDF thumbnail generation not yet implemented")
}
