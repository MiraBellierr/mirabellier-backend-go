package embed

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// RGBA is a simple canvas for rendering embed images.
type RGBA struct {
	img *image.RGBA
}

// NewRGBA creates a new canvas of the given dimensions, filled with bg.
func NewRGBA(width, height int, bg color.Color) *RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	return &RGBA{img: img}
}

// DrawImage draws an image at position.
func (c *RGBA) DrawImage(src image.Image, x, y int) {
	draw.Draw(c.img,
		image.Rect(x, y, x+src.Bounds().Dx(), y+src.Bounds().Dy()),
		src, image.Point{}, draw.Over)
}

// FillRect draws a filled rectangle.
func (c *RGBA) FillRect(x, y, w, h int, clr color.Color) {
	draw.Draw(c.img, image.Rect(x, y, x+w, y+h), &image.Uniform{clr}, image.Point{}, draw.Src)
}

// DrawText draws text using the basic font face.
func (c *RGBA) DrawText(text string, x, y int, clr color.Color) {
	face := basicfont.Face7x13
	drawer := &font.Drawer{
		Dst:  c.img,
		Src:  image.NewUniform(clr),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	drawer.DrawString(text)
}

// DrawTextCentered draws text horizontally centered at (cx, y).
func (c *RGBA) DrawTextCentered(text string, cx, y int, clr color.Color) {
	face := basicfont.Face7x13
	w := font.MeasureString(face, text).Ceil()
	c.DrawText(text, cx-w/2, y, clr)
}

// EncodePNG writes the canvas as PNG to the writer.
func (c *RGBA) EncodePNG(w io.Writer) error {
	return png.Encode(w, c.img)
}

// Bounds returns the image bounds.
func (c *RGBA) Bounds() image.Rectangle {
	return c.img.Bounds()
}

// RenderProfileEmbed generates a 1200x630 profile card PNG.
func RenderProfileEmbed(username string, avatarURL, bio *string) ([]byte, error) {
	canvas := NewRGBA(1200, 630, color.RGBA{30, 30, 40, 255})

	// Header bar
	canvas.FillRect(0, 0, 1200, 80, color.RGBA{60, 60, 120, 255})
	canvas.DrawTextCentered("mirabellier.com", 600, 50, color.White)

	// Username
	canvas.DrawTextCentered(username, 600, 300, color.White)

	// Bio
	if bio != nil && *bio != "" {
		canvas.DrawTextCentered(fmt.Sprintf("%.100s", *bio), 600, 400, color.RGBA{200, 200, 220, 255})
	}

	return encodePNG(canvas)
}

// RenderAnimeEmbed generates an anime preview card PNG.
func RenderAnimeEmbed(items []string) ([]byte, error) {
	h := 200 + len(items)*60
	if h < 400 {
		h = 400
	}
	canvas := NewRGBA(1200, h, color.RGBA{20, 20, 35, 255})

	canvas.FillRect(0, 0, 1200, 60, color.RGBA{50, 50, 100, 255})
	canvas.DrawTextCentered("Currently Watching — MyAnimeList", 600, 42, color.White)

	for i, item := range items {
		y := 90 + i*55
		canvas.DrawText(item, 40, y, color.RGBA{220, 220, 240, 255})
	}

	return encodePNG(canvas)
}

// RenderQOTDEmbed generates a 1200x630 QOTD preview card PNG.
func RenderQOTDEmbed(prompt string) ([]byte, error) {
	canvas := NewRGBA(1200, 630, color.RGBA{25, 25, 45, 255})

	canvas.FillRect(0, 0, 1200, 80, color.RGBA{80, 50, 120, 255})
	canvas.DrawTextCentered("Question of the Day", 600, 50, color.White)

	// Wrap long prompts
	if len(prompt) > 80 {
		prompt = prompt[:80] + "..."
	}
	canvas.DrawTextCentered(prompt, 600, 350, color.White)

	canvas.DrawTextCentered("Answer at mirabellier.com/question-of-the-day", 600, 500, color.RGBA{180, 180, 200, 255})

	return encodePNG(canvas)
}

// RenderQuotesEmbed generates a quotes preview card PNG.
func RenderQuotesEmbed(quotes []map[string]any) ([]byte, error) {
	canvas := NewRGBA(1200, 630, color.RGBA{20, 20, 40, 255})

	canvas.FillRect(0, 0, 1200, 80, color.RGBA{100, 70, 40, 255})
	canvas.DrawTextCentered("Quote of the Day", 600, 50, color.White)

	quoteCount := len(quotes)
	lineHeight := 100
	if quoteCount > 5 {
		quoteCount = 5
	}
	if quoteCount > 3 {
		lineHeight = 80
	}

	for i := 0; i < quoteCount; i++ {
		y := 140 + i*lineHeight
		quote, _ := quotes[i]["quote"].(string)
		if len(quote) > 100 {
			quote = quote[:100] + "..."
		}
		canvas.DrawTextCentered(fmt.Sprintf("\"%s\"", quote), 600, y, color.White)

		author, _ := quotes[i]["author"].(string)
		if author != "" {
			canvas.DrawTextCentered(fmt.Sprintf("— %s", author), 600, y+20, color.RGBA{180, 180, 200, 255})
		}
	}

	return encodePNG(canvas)
}

func encodePNG(canvas *RGBA) ([]byte, error) {
	var buf bytesBuffer
	if err := canvas.EncodePNG(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// bytesBuffer implements io.Writer using a byte slice.
type bytesBuffer struct {
	buf []byte
}

func (b *bytesBuffer) Write(p []byte) (int, error) {
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *bytesBuffer) Bytes() []byte {
	return b.buf
}
