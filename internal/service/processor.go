package service

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"

	"resizr/pkg/logger"

	"github.com/disintegration/imaging"
	"github.com/icza/gox/imagex/colorx"
	"go.uber.org/zap"
	"golang.org/x/image/webp"
)

// ProcessorServiceImpl implements the ProcessorService interface
type ProcessorServiceImpl struct {
	maxWidth  int // Maximum allowed image width
	maxHeight int // Maximum allowed image height
}

// NewProcessorService creates a new image processor service
func NewProcessorService(maxWidth, maxHeight int) ProcessorService {
	if maxWidth <= 0 {
		maxWidth = 4096 // Default maximum width
	}
	if maxHeight <= 0 {
		maxHeight = 4096 // Default maximum height
	}

	return &ProcessorServiceImpl{
		maxWidth:  maxWidth,
		maxHeight: maxHeight,
	}
}

// DetectFormat detects image format from data
func (p *ProcessorServiceImpl) DetectFormat(data []byte) (string, error) {
	if len(data) < 512 {
		return "", fmt.Errorf("insufficient data for format detection")
	}

	// Use http.DetectContentType for initial detection
	contentType := http.DetectContentType(data)

	// Validate it's an image and supported format
	switch contentType {
	case "image/jpeg":
		return "image/jpeg", nil
	case "image/png":
		return "image/png", nil
	case "image/gif":
		return "image/gif", nil
	case "image/webp":
		return "image/webp", nil
	default:
		// Try more specific detection
		return p.detectFormatByHeader(data)
	}
}

// detectFormatByHeader detects format by examining file headers
func (p *ProcessorServiceImpl) detectFormatByHeader(data []byte) (string, error) {
	if len(data) < 12 {
		return "", fmt.Errorf("file too small for format detection")
	}

	// JPEG: FF D8 FF
	if bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) {
		return "image/jpeg", nil
	}

	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return "image/png", nil
	}

	// GIF: 47 49 46 38 (GIF8)
	if bytes.HasPrefix(data, []byte{0x47, 0x49, 0x46, 0x38}) {
		return "image/gif", nil
	}

	// WebP: Check for RIFF and WEBP
	if bytes.HasPrefix(data, []byte{0x52, 0x49, 0x46, 0x46}) && len(data) >= 12 {
		if bytes.Equal(data[8:12], []byte{0x57, 0x45, 0x42, 0x50}) {
			return "image/webp", nil
		}
	}

	return "", fmt.Errorf("unsupported image format")
}

// GetDimensions extracts image dimensions
func (p *ProcessorServiceImpl) GetDimensions(data []byte) (width, height int, err error) {
	// Decode image to get dimensions
	img, _, err := p.decodeImage(data)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to decode image for dimensions: %w", err)
	}

	return p.getImageDimensions(img)
}

// ProcessImage resizes image to specified resolution
func (p *ProcessorServiceImpl) ProcessImage(data []byte, config ResizeConfig) ([]byte, error) {
	logger.Debug("Processing image",
		zap.Int("target_width", config.Width),
		zap.Int("target_height", config.Height),
		zap.String("mode", string(config.Mode)),
		zap.Int("quality", config.Quality),
		zap.String("background_color", config.BackgroundColor))

	// Decode original image
	srcImage, format, err := p.decodeImage(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode source image: %w", err)
	}

	// Validate target dimensions
	if config.Width <= 0 || config.Height <= 0 {
		return nil, fmt.Errorf("invalid target dimensions: %dx%d", config.Width, config.Height)
	}

	if config.Width > p.maxWidth || config.Height > p.maxHeight {
		return nil, fmt.Errorf("target dimensions %dx%d exceed maximum allowed %dx%d",
			config.Width, config.Height, p.maxWidth, p.maxHeight)
	}

	// Validate target canvas background
	backgroundColor, err := colorx.ParseHexColor(config.BackgroundColor)
	if err != nil {
		return nil, fmt.Errorf("failed to parse background color HEX: %w", err)
	}

	// Apply resize based on mode
	var resizedImage image.Image

	switch config.Mode {
	case ResizeModeSmartFit:
		resizedImage = p.smartFitResize(srcImage, config.Width, config.Height, backgroundColor)
	case ResizeModeCrop:
		resizedImage = p.cropResize(srcImage, config.Width, config.Height)
	case ResizeModeStretch:
		resizedImage = imaging.Resize(srcImage, config.Width, config.Height, imaging.Lanczos)
	default:
		// Default to smart fit
		resizedImage = p.smartFitResize(srcImage, config.Width, config.Height, backgroundColor)
	}

	// Encode the processed image
	processedData, err := p.encodeImage(resizedImage, format, config.Quality)
	if err != nil {
		return nil, fmt.Errorf("failed to encode processed image: %w", err)
	}

	logger.Debug("Image processing completed",
		zap.Int("original_size", len(data)),
		zap.Int("processed_size", len(processedData)),
		zap.String("format", format))

	return processedData, nil
}

// ConvertImage converts image to another format
func (p *ProcessorServiceImpl) ConvertImage(data []byte, config ConvertConfig) ([]byte, error) {
	logger.Debug("Converting image",
		zap.String("target_format", config.Format),
		zap.Int("target_quality", config.Quality),
	)

	// Decode original image
	srcImage, format, err := p.decodeImage(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode source image: %w", err)
	}

	if format != config.Format {
		return nil, fmt.Errorf("expected different from %s format", format)
	}

	// Validate image dimensions
	width, height, err := p.getImageDimensions(srcImage)
	if err != nil {
		return nil, fmt.Errorf("invalid image dimensions: %w", err)
	}

	// Encode to another format
	convertedData, err := p.encodeImage(srcImage, config.Format, config.Quality)
	if err != nil {
		return nil, fmt.Errorf("failed to encode processed image: %w", err)
	}

	logger.Debug("Image conversion successed",
		zap.String("format", config.Format),
		zap.Int("width", width),
		zap.Int("height", height),
	)

	return convertedData, nil
}

// ValidateImage checks if image data is valid
func (p *ProcessorServiceImpl) ValidateImage(data []byte, maxSize int64) error {
	// Check file size
	if int64(len(data)) > maxSize {
		return fmt.Errorf("image size %d bytes exceeds maximum allowed %d bytes",
			len(data), maxSize)
	}

	if len(data) == 0 {
		return fmt.Errorf("empty image data")
	}

	// Validate format
	format, err := p.DetectFormat(data)
	if err != nil {
		return fmt.Errorf("invalid image format: %w", err)
	}

	// Validate dimensions
	width, height, err := p.GetDimensions(data)
	if err != nil {
		return fmt.Errorf("invalid image dimensions: %w", err)
	}

	logger.Debug("Image validation passed",
		zap.String("format", format),
		zap.Int("width", width),
		zap.Int("height", height),
		zap.Int("size", len(data)))

	return nil
}

// Helper methods

// decodeImage decodes image data into image.Image
func (p *ProcessorServiceImpl) decodeImage(data []byte) (image.Image, string, error) {
	reader := bytes.NewReader(data)

	// Try to decode as different formats
	img, format, err := image.Decode(reader)
	if err != nil {
		// Try WebP specifically (not in standard library)
		reader.Seek(0, 0)
		if webpImg, webpErr := webp.Decode(reader); webpErr == nil {
			return webpImg, "webp", nil
		}
		return nil, "", err
	}

	return img, format, nil
}

// encodeImage encodes image.Image to bytes
func (p *ProcessorServiceImpl) encodeImage(img image.Image, format string, quality int) ([]byte, error) {
	var buf bytes.Buffer

	switch format {
	case "jpeg":
		options := &jpeg.Options{Quality: quality}
		if err := jpeg.Encode(&buf, img, options); err != nil {
			return nil, err
		}
	case "png":
		encoder := &png.Encoder{CompressionLevel: png.DefaultCompression}
		if err := encoder.Encode(&buf, img); err != nil {
			return nil, err
		}
	case "gif":
		options := &gif.Options{NumColors: 256}
		if err := gif.Encode(&buf, img, options); err != nil {
			return nil, err
		}
	case "webp":
		// For WebP, we'll fall back to JPEG for now
		// (WebP encoding requires additional libraries)
		options := &jpeg.Options{Quality: quality}
		if err := jpeg.Encode(&buf, img, options); err != nil {
			return nil, err
		}
	default:
		// Default to JPEG
		options := &jpeg.Options{Quality: quality}
		if err := jpeg.Encode(&buf, img, options); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

// smartFitResize implements smart fit algorithm
func (p *ProcessorServiceImpl) smartFitResize(src image.Image, targetWidth, targetHeight int, backgroundColor color.Color) image.Image {
	srcBounds := src.Bounds()
	srcWidth := srcBounds.Dx()
	srcHeight := srcBounds.Dy()

	// Calculate aspect ratios
	srcAspect := float64(srcWidth) / float64(srcHeight)
	targetAspect := float64(targetWidth) / float64(targetHeight)

	var resizedWidth, resizedHeight int

	// Determine resize dimensions to fit inside target while maintaining aspect ratio
	if srcAspect > targetAspect {
		// Source is wider - fit by width
		resizedWidth = targetWidth
		resizedHeight = int(float64(targetWidth) / srcAspect)
	} else {
		// Source is taller - fit by height
		resizedHeight = targetHeight
		resizedWidth = int(float64(targetHeight) * srcAspect)
	}

	// Resize the image maintaining aspect ratio
	resized := imaging.Resize(src, resizedWidth, resizedHeight, imaging.Lanczos)

	// Create target canvas and center the resized image
	canvas := imaging.New(targetWidth, targetHeight, backgroundColor)

	// Calculate position to center the image
	x := (targetWidth - resizedWidth) / 2
	y := (targetHeight - resizedHeight) / 2

	// Paste the resized image onto the canvas
	result := imaging.Paste(canvas, resized, image.Pt(x, y))

	return result
}

// cropResize implements crop resize algorithm
func (p *ProcessorServiceImpl) cropResize(src image.Image, targetWidth, targetHeight int) image.Image {
	srcBounds := src.Bounds()
	srcWidth := srcBounds.Dx()
	srcHeight := srcBounds.Dy()

	// Calculate aspect ratios
	srcAspect := float64(srcWidth) / float64(srcHeight)
	targetAspect := float64(targetWidth) / float64(targetHeight)

	var resizedWidth, resizedHeight int

	// Determine resize dimensions to fill target area
	if srcAspect > targetAspect {
		// Source is wider - fit by height, crop width
		resizedHeight = targetHeight
		resizedWidth = int(float64(targetHeight) * srcAspect)
	} else {
		// Source is taller - fit by width, crop height
		resizedWidth = targetWidth
		resizedHeight = int(float64(targetWidth) / srcAspect)
	}

	// Resize the image
	resized := imaging.Resize(src, resizedWidth, resizedHeight, imaging.Lanczos)

	// Crop to target size from center
	cropped := imaging.CropCenter(resized, targetWidth, targetHeight)

	return cropped
}

// getIageDimensions returns dimensions of decoded image
func (p *ProcessorServiceImpl) getImageDimensions(img image.Image) (width, height int, err error) {
	bounds := img.Bounds()
	width = bounds.Dx()
	height = bounds.Dy()

	// Validate dimensions
	if width <= 0 || height <= 0 {
		return 0, 0, fmt.Errorf("invalid image dimensions: %dx%d", width, height)
	}

	if width > p.maxWidth || height > p.maxHeight {
		return 0, 0, fmt.Errorf("image dimensions %dx%d exceed maximum allowed %dx%d",
			width, height, p.maxWidth, p.maxHeight)
	}

	return width, height, nil
}

// GetSupportedFormats returns list of supported image formats
func (p *ProcessorServiceImpl) GetSupportedFormats() []string {
	return []string{"image/jpeg", "image/png", "image/gif", "image/webp"}
}

// GetMaxDimension returns maximum allowed image dimension
func (p *ProcessorServiceImpl) GetMaxDimension() int {
	// Return the larger of the two as the overall max dimension
	if p.maxWidth >= p.maxHeight {
		return p.maxWidth
	}
	return p.maxHeight
}
