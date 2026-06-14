package images

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
)

type OptimizeResult struct {
	OriginalName string
	JPEGName     string
	WebPName     string
}

// SaveAndOptimize saves uploaded image data and creates optimized JPEG + WebP versions.
// Mirrors the Node.js Sharp pipeline: resize to max 2000x2000, output JPEG (q85) + WebP (q85).
func SaveAndOptimize(data []byte, originalFilename, imagesDir string) (*OptimizeResult, error) {
	ext := strings.ToLower(filepath.Ext(originalFilename))

	base := fmt.Sprintf("mirabellier-image-%d-%d", time.Now().UnixMilli(), time.Now().Nanosecond()%10000)

	// Save original
	origPath := filepath.Join(imagesDir, base+ext)
	if err := os.WriteFile(origPath, data, 0644); err != nil {
		return nil, fmt.Errorf("save original: %w", err)
	}

	// Decode
	img, err := imaging.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	// Resize if larger than 2000x2000
	bounds := img.Bounds()
	if bounds.Dx() > 2000 || bounds.Dy() > 2000 {
		img = imaging.Fit(img, 2000, 2000, imaging.Lanczos)
	}

	// Save JPEG
	jpgPath := filepath.Join(imagesDir, base+".jpg")
	if err := imaging.Save(img, jpgPath, imaging.JPEGQuality(85)); err != nil {
		return nil, fmt.Errorf("save jpeg: %w", err)
	}

	// Save WebP
	webpPath := filepath.Join(imagesDir, base+".webp")
	if err := saveAsWebP(img, webpPath); err != nil {
		// WebP is optional
		webpPath = ""
	}

	return &OptimizeResult{
		OriginalName: base + ext,
		JPEGName:     base + ".jpg",
		WebPName:     base + ".webp",
	}, nil
}

// saveAsWebP attempts to encode as WebP. Falls back to cwebp CLI if available.
func saveAsWebP(img image.Image, path string) error {
	// Try cwebp CLI (most reliable on Linux)
	tmpPNG := path + ".tmp.png"
	if err := imaging.Save(img, tmpPNG, imaging.PNGCompressionLevel(0)); err != nil {
		return err
	}
	defer os.Remove(tmpPNG)

	cmd := exec.Command("cwebp", "-q", "85", tmpPNG, "-o", path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cwebp failed: %w", err)
	}
	return nil
}

// ConvertToWebP converts an image file to WebP format.
func ConvertToWebP(inputPath, outputPath string) error {
	img, err := imaging.Open(inputPath)
	if err != nil {
		return err
	}
	return saveAsWebP(img, outputPath)
}

// OptimizeFromFile resizes and converts an existing file.
func OptimizeFromFile(inputPath, imagesDir string) (*OptimizeResult, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, err
	}
	return SaveAndOptimize(data, filepath.Base(inputPath), imagesDir)
}

// DecodeToJPEG decodes any image format and saves as JPEG at the given quality.
func DecodeToJPEG(data []byte, outputPath string, quality int) error {
	img, err := imaging.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return jpeg.Encode(f, img, &jpeg.Options{Quality: quality})
}
