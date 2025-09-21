package service

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewProcessorService(t *testing.T) {
	processor := NewProcessorService(4096, 4096)
	assert.NotNil(t, processor)
}

func TestProcessorService_DetectFormat(t *testing.T) {
	processor := NewProcessorService(4096, 4096)

	t.Run("detect_jpeg", func(t *testing.T) {
		// Create a proper JPEG with sufficient data (minimum 512 bytes)
		jpegData := make([]byte, 512)
		jpegData[0] = 0xFF
		jpegData[1] = 0xD8
		jpegData[2] = 0xFF
		jpegData[3] = 0xE0
		copy(jpegData[4:], []byte{0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01})
		jpegData[510] = 0xFF
		jpegData[511] = 0xD9

		format, err := processor.DetectFormat(jpegData)
		assert.NoError(t, err)
		assert.Equal(t, "image/jpeg", format)
	})

	t.Run("detect_png", func(t *testing.T) {
		// Create a proper PNG with sufficient data (minimum 512 bytes)
		pngData := make([]byte, 512)
		copy(pngData[0:], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})  // PNG signature
		copy(pngData[8:], []byte{0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52})  // IHDR chunk
		copy(pngData[16:], []byte{0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01}) // 1x1 image
		copy(pngData[508:], []byte{0x49, 0x45, 0x4E, 0x44})                        // IEND chunk

		format, err := processor.DetectFormat(pngData)
		assert.NoError(t, err)
		assert.Equal(t, "image/png", format)
	})

	t.Run("invalid_format", func(t *testing.T) {
		invalidData := []byte("not an image")

		format, err := processor.DetectFormat(invalidData)
		assert.Error(t, err)
		assert.Empty(t, format)
	})

	t.Run("empty_data", func(t *testing.T) {
		format, err := processor.DetectFormat([]byte{})
		assert.Error(t, err)
		assert.Empty(t, format)
	})
}

func TestProcessorService_GetDimensions(t *testing.T) {
	processor := NewProcessorService(4096, 4096)

	t.Run("get_jpeg_dimensions", func(t *testing.T) {
		// Create a simple test image
		img := image.NewRGBA(image.Rect(0, 0, 100, 50))
		var buf bytes.Buffer
		err := jpeg.Encode(&buf, img, nil)
		assert.NoError(t, err)

		width, height, err := processor.GetDimensions(buf.Bytes())
		assert.NoError(t, err)
		assert.Equal(t, 100, width)
		assert.Equal(t, 50, height)
	})

	t.Run("get_png_dimensions", func(t *testing.T) {
		// Create a simple test image
		img := image.NewRGBA(image.Rect(0, 0, 200, 100))
		var buf bytes.Buffer
		err := png.Encode(&buf, img)
		assert.NoError(t, err)

		width, height, err := processor.GetDimensions(buf.Bytes())
		assert.NoError(t, err)
		assert.Equal(t, 200, width)
		assert.Equal(t, 100, height)
	})

	t.Run("invalid_image_data", func(t *testing.T) {
		invalidData := []byte("not an image")

		width, height, err := processor.GetDimensions(invalidData)
		assert.Error(t, err)
		assert.Equal(t, 0, width)
		assert.Equal(t, 0, height)
	})
}

func TestProcessorService_ValidateImage(t *testing.T) {
	processor := NewProcessorService(4096, 4096)

	t.Run("valid_image_size", func(t *testing.T) {
		// Create a small test image
		img := image.NewRGBA(image.Rect(0, 0, 10, 10))
		var buf bytes.Buffer
		err := jpeg.Encode(&buf, img, nil)
		assert.NoError(t, err)

		err = processor.ValidateImage(buf.Bytes(), 10*1024*1024) // 10MB limit
		assert.NoError(t, err)
	})

	t.Run("image_too_large", func(t *testing.T) {
		data := make([]byte, 1024)                // 1KB of data
		err := processor.ValidateImage(data, 512) // 512 byte limit
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "image size")
	})

	t.Run("invalid_image_format", func(t *testing.T) {
		invalidData := []byte("not an image")
		err := processor.ValidateImage(invalidData, 10*1024*1024)
		assert.Error(t, err)
	})

	t.Run("empty_data", func(t *testing.T) {
		err := processor.ValidateImage([]byte{}, 10*1024*1024)
		assert.Error(t, err)
	})
}

func TestProcessorService_ProcessImage(t *testing.T) {
	processor := NewProcessorService(4096, 4096)

	t.Run("resize_jpeg", func(t *testing.T) {
		// Create a test image
		img := image.NewRGBA(image.Rect(0, 0, 100, 100))
		var buf bytes.Buffer
		err := jpeg.Encode(&buf, img, nil)
		assert.NoError(t, err)

		config := ResizeConfig{
			Width:           50,
			Height:          50,
			Quality:         85,
			Format:          "jpeg",
			Mode:            ResizeModeSmartFit,
			BackgroundColor: "#FFFFFF",
		}

		processedData, err := processor.ProcessImage(buf.Bytes(), config)
		assert.NoError(t, err)
		assert.NotEmpty(t, processedData)

		// Verify processed image dimensions
		width, height, err := processor.GetDimensions(processedData)
		assert.NoError(t, err)
		assert.Equal(t, 50, width)
		assert.Equal(t, 50, height)
	})

	t.Run("resize_png", func(t *testing.T) {
		// Create a test image
		img := image.NewRGBA(image.Rect(0, 0, 200, 100))
		var buf bytes.Buffer
		err := png.Encode(&buf, img)
		assert.NoError(t, err)

		config := ResizeConfig{
			Width:           100,
			Height:          50,
			Format:          "png",
			Mode:            ResizeModeCrop,
			BackgroundColor: "#FFFFFF",
		}

		processedData, err := processor.ProcessImage(buf.Bytes(), config)
		assert.NoError(t, err)
		assert.NotEmpty(t, processedData)

		// Verify processed image dimensions
		width, height, err := processor.GetDimensions(processedData)
		assert.NoError(t, err)
		assert.Equal(t, 100, width)
		assert.Equal(t, 50, height)
	})

	t.Run("invalid_input_data", func(t *testing.T) {
		config := ResizeConfig{
			Width:           100,
			Height:          100,
			Format:          "jpeg",
			BackgroundColor: "#FFFFFF",
		}

		_, err := processor.ProcessImage([]byte("not an image"), config)
		assert.Error(t, err)
	})

	t.Run("unsupported_format", func(t *testing.T) {
		// Create a test image
		img := image.NewRGBA(image.Rect(0, 0, 100, 100))
		var buf bytes.Buffer
		err := jpeg.Encode(&buf, img, nil)
		assert.NoError(t, err)

		config := ResizeConfig{
			Width:           50,
			Height:          50,
			Format:          "unsupported",
			BackgroundColor: "#FFFFFF",
		}

		_, err = processor.ProcessImage(buf.Bytes(), config)
		assert.Error(t, err)
	})

	t.Run("stretch_mode", func(t *testing.T) {
		// Create a test image
		img := image.NewRGBA(image.Rect(0, 0, 100, 50))
		var buf bytes.Buffer
		err := jpeg.Encode(&buf, img, nil)
		assert.NoError(t, err)

		config := ResizeConfig{
			Width:           200,
			Height:          200,
			Format:          "jpeg",
			Mode:            ResizeModeStretch,
			BackgroundColor: "#FFFFFF",
		}

		processedData, err := processor.ProcessImage(buf.Bytes(), config)
		assert.NoError(t, err)
		assert.NotEmpty(t, processedData)

		// Verify processed image dimensions
		width, height, err := processor.GetDimensions(processedData)
		assert.NoError(t, err)
		assert.Equal(t, 200, width)
		assert.Equal(t, 200, height)
	})
}

func TestProcessorService_DetectFormat_Additional(t *testing.T) {
	processor := NewProcessorService(4096, 4096)

	t.Run("detect_gif", func(t *testing.T) {
		// Create proper GIF with sufficient data (minimum 512 bytes)
		gifData := make([]byte, 512)
		copy(gifData[0:], []byte{0x47, 0x49, 0x46, 0x38, 0x37, 0x61, 0x01, 0x00, 0x01, 0x00, 0x80, 0x00})
		format, err := processor.DetectFormat(gifData)
		assert.NoError(t, err)
		assert.Equal(t, "image/gif", format)
	})

	t.Run("detect_webp", func(t *testing.T) {
		// Create proper WebP with sufficient data (minimum 512 bytes)
		webpData := make([]byte, 512)
		copy(webpData[0:], []byte{
			0x52, 0x49, 0x46, 0x46, // RIFF
			0x1A, 0x00, 0x00, 0x00, // file size
			0x57, 0x45, 0x42, 0x50, // WEBP
			0x56, 0x50, 0x38, 0x4C, // VP8L
			0x0E, 0x00, 0x00, 0x00, // chunk size
			0x2F, 0x00, 0x00, 0x00, 0x00, 0x88, 0x88, 0x08,
		})
		format, err := processor.DetectFormat(webpData)
		assert.NoError(t, err)
		assert.Equal(t, "image/webp", format)
	})

	t.Run("riff_but_not_webp", func(t *testing.T) {
		// RIFF header but not WebP (AVI format) with sufficient data
		riffData := make([]byte, 512)
		copy(riffData[0:], []byte{
			0x52, 0x49, 0x46, 0x46, // RIFF
			0x26, 0x00, 0x00, 0x00, // file size
			0x41, 0x56, 0x49, 0x20, // AVI format
		})
		format, err := processor.DetectFormat(riffData)
		assert.Error(t, err)
		assert.Empty(t, format)
	})
}

func TestProcessorService_ProcessImage_AdditionalFormats(t *testing.T) {
	processor := NewProcessorService(4096, 4096)

	t.Run("process_gif_format", func(t *testing.T) {
		// Create a test image
		img := image.NewRGBA(image.Rect(0, 0, 100, 100))
		var buf bytes.Buffer
		err := jpeg.Encode(&buf, img, nil)
		assert.NoError(t, err)

		config := ResizeConfig{
			Width:           50,
			Height:          50,
			Quality:         85,
			Format:          "gif",
			Mode:            ResizeModeSmartFit,
			BackgroundColor: "#FFFFFF",
		}

		processedData, err := processor.ProcessImage(buf.Bytes(), config)
		assert.NoError(t, err)
		assert.NotEmpty(t, processedData)

		// Verify the format is GIF
		format, err := processor.DetectFormat(processedData)
		assert.NoError(t, err)
		assert.Equal(t, "image/gif", format)
	})

	t.Run("process_webp_format", func(t *testing.T) {
		// Create a test image
		img := image.NewRGBA(image.Rect(0, 0, 100, 100))
		var buf bytes.Buffer
		err := jpeg.Encode(&buf, img, nil)
		assert.NoError(t, err)

		config := ResizeConfig{
			Width:           50,
			Height:          50,
			Quality:         85,
			Format:          "webp",
			Mode:            ResizeModeSmartFit,
			BackgroundColor: "#FFFFFF",
		}

		processedData, err := processor.ProcessImage(buf.Bytes(), config)
		assert.NoError(t, err)
		assert.NotEmpty(t, processedData)

		// WebP falls back to JPEG currently
		format, err := processor.DetectFormat(processedData)
		assert.NoError(t, err)
		assert.Equal(t, "image/jpeg", format)
	})

	t.Run("process_large_dimensions", func(t *testing.T) {
		// Create a test image
		img := image.NewRGBA(image.Rect(0, 0, 100, 100))
		var buf bytes.Buffer
		err := jpeg.Encode(&buf, img, nil)
		assert.NoError(t, err)

		config := ResizeConfig{
			Width:           5000, // Exceeds max dimensions
			Height:          5000,
			Quality:         85,
			Format:          "jpeg",
			Mode:            ResizeModeSmartFit,
			BackgroundColor: "#FFFFFF",
		}

		_, err = processor.ProcessImage(buf.Bytes(), config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "exceed maximum")
	})

	t.Run("process_zero_dimensions", func(t *testing.T) {
		// Create a test image
		img := image.NewRGBA(image.Rect(0, 0, 100, 100))
		var buf bytes.Buffer
		err := jpeg.Encode(&buf, img, nil)
		assert.NoError(t, err)

		config := ResizeConfig{
			Width:           0,
			Height:          0,
			Quality:         85,
			Format:          "jpeg",
			Mode:            ResizeModeSmartFit,
			BackgroundColor: "#FFFFFF",
		}

		_, err = processor.ProcessImage(buf.Bytes(), config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
	})
}
