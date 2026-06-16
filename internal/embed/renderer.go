package embed

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	PreviewWidth  = 1200
	PreviewHeight = 630
)

type AnimeItem struct {
	Title           string
	CoverImage      string
	MediaType       string
	WatchedEpisodes int
	TotalEpisodes   int
	Score           int
	UpdatedAt       string
	Season          string
	SeasonYear      int
}

type AnimePreview struct {
	Username  string
	FetchedAt string
	Stale     bool
	Items     []AnimeItem
	Message   string
	ErrorCode string
}

type BlogPreview struct {
	Title       string
	Description string
	Author      string
	PublishedAt string
	UpdatedAt   string
	Thumbnail   string
	Tags        []string
}

type ShrinePreview struct {
	Title       string
	Description string
	Image       string
	ImageAlt    string
	Slug        string
}

type RGBA struct {
	img *image.RGBA
}

func NewRGBA(width, height int, bg color.Color) *RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	return &RGBA{img: img}
}

func blendRGBA(dst, src color.RGBA) color.RGBA {
	if src.A == 255 {
		return src
	}
	if src.A == 0 {
		return dst
	}
	a := int(src.A)
	inv := 255 - a
	return color.RGBA{
		R: uint8((int(src.R)*a + int(dst.R)*inv) / 255),
		G: uint8((int(src.G)*a + int(dst.G)*inv) / 255),
		B: uint8((int(src.B)*a + int(dst.B)*inv) / 255),
		A: 255,
	}
}

func (c *RGBA) FillRect(x, y, w, h int, clr color.Color) {
	draw.Draw(c.img, image.Rect(x, y, x+w, y+h), &image.Uniform{clr}, image.Point{}, draw.Src)
}

func (c *RGBA) FillBorderedRect(x, y, w, h, border int, fill, stroke color.Color) {
	c.FillRect(x, y, w, h, stroke)
	c.FillRect(x+border, y+border, w-border*2, h-border*2, fill)
}

func (c *RGBA) StrokeRect(x, y, w, h, border int, stroke color.Color) {
	c.FillRect(x, y, w, border, stroke)
	c.FillRect(x, y+h-border, w, border, stroke)
	c.FillRect(x, y, border, h, stroke)
	c.FillRect(x+w-border, y, border, h, stroke)
}

func (c *RGBA) FillCircle(cx, cy, r int, clr color.RGBA) {
	r2 := r * r
	minX, maxX := max(0, cx-r), min(c.img.Bounds().Dx()-1, cx+r)
	minY, maxY := max(0, cy-r), min(c.img.Bounds().Dy()-1, cy+r)
	for y := minY; y <= maxY; y++ {
		dy := y - cy
		for x := minX; x <= maxX; x++ {
			dx := x - cx
			if dx*dx+dy*dy <= r2 {
				c.img.SetRGBA(x, y, blendRGBA(c.img.RGBAAt(x, y), clr))
			}
		}
	}
}

func (c *RGBA) DrawText(text string, x, y, size int, clr color.Color) {
	drawScaledText(c.img, text, x, y, size, clr)
}

func (c *RGBA) DrawTextCentered(text string, cx, y, size int, clr color.Color) {
	w := textWidth(text, size)
	c.DrawText(text, cx-w/2, y, size, clr)
}

func (c *RGBA) DrawTextRight(text string, right, y, size int, clr color.Color) {
	c.DrawText(text, right-textWidth(text, size), y, size, clr)
}

func (c *RGBA) DrawImage(src image.Image, x, y, w, h int) {
	if src == nil {
		return
	}
	resized := imaging.Fill(src, w, h, imaging.Center, imaging.Lanczos)
	draw.Draw(c.img, image.Rect(x, y, x+w, y+h), resized, image.Point{}, draw.Over)
}

func (c *RGBA) DrawProgressBar(x, y, w, h int, pct float64, fill, bg, textClr color.Color) {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	// Background track
	c.FillBorderedRect(x, y, w, h, 1, color.RGBA{248, 251, 255, 255}, bg)
	// Filled portion
	fillW := int(float64(w-4) * pct)
	if fillW > 0 {
		c.FillRect(x+2, y+2, fillW, h-4, fill)
	}
	pctText := fmt.Sprintf("%.0f%%", pct*100)
	pctX := x + w + 10
	c.DrawText(pctText, pctX, y+h+2, h+5, textClr)
}

func (c *RGBA) DrawBadge(x, y int, text string, size int) int {
	padX, padYT, padYB := 8, 4, 4
	w := textWidth(text, size) + padX*2
	h := padYT + size + padYB
	bg := color.RGBA{239, 246, 255, 255}
	border := color.RGBA{191, 219, 254, 255}
	textClr := color.RGBA{51, 65, 85, 255}
	c.FillBorderedRect(x, y, w, h, 1, bg, border)
	c.DrawText(text, x+padX, y+padYT+size, size, textClr)
	return x + w
}

func (c *RGBA) EncodePNG(w io.Writer) error {
	return png.Encode(w, c.img)
}

func RenderProfileEmbed(username string, avatarURL, bio *string) ([]byte, error) {
	canvas := newModernCanvas(PreviewHeight)
	cardX, cardY, cardW, cardH := 86, 82, 1028, 466
	drawPanel(canvas, cardX, cardY, cardW, cardH)
	canvas.DrawText("mirabellier.com / profile", cardX+44, cardY+52, 18, mutedBlue())

	avatarX, avatarY, avatarSize := cardX+44, cardY+118, 180
	if avatarURL != nil {
		if img := fetchImage(*avatarURL); img != nil {
			canvas.DrawImage(img, avatarX, avatarY, avatarSize, avatarSize)
		} else {
			drawInitialBlock(canvas, avatarX, avatarY, avatarSize, avatarSize, username)
		}
	} else {
		drawInitialBlock(canvas, avatarX, avatarY, avatarSize, avatarSize, username)
	}
	canvas.StrokeRect(avatarX, avatarY, avatarSize, avatarSize, 3, color.RGBA{191, 219, 254, 255})

	textX := avatarX + avatarSize + 48
	canvas.DrawText(limit(username, 34), textX, cardY+176, 52, deepBlue())
	desc := "A Mirabellier profile."
	if bio != nil && strings.TrimSpace(*bio) != "" {
		desc = *bio
	}
	y := cardY + 232
	for _, line := range wrapText(desc, 58, 4) {
		canvas.DrawText(line, textX, y, 24, slate())
		y += 32
	}
	canvas.DrawText("View profile", textX, cardY+cardH-58, 20, accentBlue())
	return encodePNG(canvas)
}

func RenderQOTDEmbed(prompt string) ([]byte, error) {
	canvas := NewRGBA(PreviewWidth, PreviewHeight, color.RGBA{248, 251, 255, 255})
	drawQuestionBackground(canvas)

	if strings.TrimSpace(prompt) == "" {
		canvas.DrawTextCentered("No active question yet.", 600, 326, 42, color.RGBA{30, 58, 138, 255})
		return encodePNG(canvas)
	}

	layout := chooseTextLayout(prompt, []textCandidate{
		{28, 3, 62, 66}, {32, 4, 56, 60}, {36, 4, 50, 54},
		{40, 5, 44, 48}, {46, 6, 38, 42}, {52, 7, 33, 37},
		{58, 8, 29, 33}, {64, 9, 25, 29},
	}, 420)
	textHeight := layout.Size + max(0, len(layout.Lines)-1)*layout.LineHeight
	y := PreviewHeight/2 - textHeight/2 + layout.Size
	for _, line := range layout.Lines {
		canvas.DrawTextCentered(line, PreviewWidth/2, y, layout.Size, color.RGBA{30, 58, 138, 255})
		y += layout.LineHeight
	}

	return encodePNG(canvas)
}

func RenderBlogEmbed(preview BlogPreview) ([]byte, error) {
	canvas := newModernCanvas(PreviewHeight)

	cardX, cardY, cardW, cardH := 76, 76, 1048, 478
	drawPanel(canvas, cardX, cardY, cardW, cardH)
	canvas.DrawText("mirabellier.com / blog", cardX+40, cardY+46, 18, mutedBlue())

	textW := 880
	if preview.Thumbnail != "" {
		textW = 650
		imgX, imgY, imgW, imgH := cardX+cardW-316, cardY+76, 254, 326
		if img := fetchImage(preview.Thumbnail); img != nil {
			canvas.DrawImage(img, imgX, imgY, imgW, imgH)
			canvas.StrokeRect(imgX, imgY, imgW, imgH, 3, color.RGBA{191, 219, 254, 255})
		} else {
			drawInitialBlock(canvas, imgX, imgY, imgW, imgH, "blog")
		}
	}

	title := strings.TrimSpace(preview.Title)
	if title == "" {
		title = "Untitled blog post"
	}
	titleChars := 34
	if textW > 700 {
		titleChars = 46
	}
	titleLines := wrapText(title, titleChars, 3)
	y := cardY + 118
	for _, line := range titleLines {
		canvas.DrawText(line, cardX+40, y, 38, deepBlue())
		y += 44
	}

	desc := strings.TrimSpace(preview.Description)
	if desc == "" {
		desc = "A post from Mirabellier."
	}
	descChars := 56
	if textW > 700 {
		descChars = 72
	}
	for _, line := range wrapText(desc, descChars, 4) {
		canvas.DrawText(line, cardX+42, y+22, 22, slate())
		y += 28
	}

	meta := "By " + strings.TrimSpace(preview.Author)
	if strings.TrimSpace(preview.Author) == "" {
		meta = "By Mirabellier"
	}
	if t := formatDate(preview.PublishedAt); t != "" {
		meta += " - " + t
	}
	canvas.DrawText(meta, cardX+42, cardY+cardH-74, 20, accentBlue())

	if len(preview.Tags) > 0 {
		tagLine := "# " + strings.Join(preview.Tags, "  # ")
		canvas.DrawText(limit(tagLine, 80), cardX+42, cardY+cardH-36, 18, mutedSlate())
	}

	return encodePNG(canvas)
}

func RenderShrineEmbed(preview ShrinePreview) ([]byte, error) {
	canvas := newModernCanvas(PreviewHeight)
	cardX, cardY, cardW, cardH := 76, 76, 1048, 478
	drawPanel(canvas, cardX, cardY, cardW, cardH)
	canvas.DrawText("mirabellier.com / shrine", cardX+40, cardY+46, 18, mutedBlue())

	imageX, imageY, imageW, imageH := cardX+cardW-350, cardY+82, 288, 316
	if preview.Image != "" {
		if img := fetchImage(preview.Image); img != nil {
			canvas.DrawImage(img, imageX, imageY, imageW, imageH)
			canvas.StrokeRect(imageX, imageY, imageW, imageH, 3, color.RGBA{191, 219, 254, 255})
		} else {
			drawInitialBlock(canvas, imageX, imageY, imageW, imageH, preview.Title)
		}
	} else {
		drawInitialBlock(canvas, imageX, imageY, imageW, imageH, preview.Title)
	}

	title := strings.TrimSpace(preview.Title)
	if title == "" {
		title = "Character Shrine"
	}
	y := cardY + 138
	for _, line := range wrapText(title, 34, 3) {
		canvas.DrawText(line, cardX+42, y, 42, deepBlue())
		y += 48
	}
	desc := strings.TrimSpace(preview.Description)
	if desc == "" {
		desc = "A character shrine on Mirabellier."
	}
	for _, line := range wrapText(desc, 52, 4) {
		canvas.DrawText(line, cardX+44, y+20, 22, slate())
		y += 30
	}
	canvas.DrawText("Open shrine", cardX+44, cardY+cardH-56, 20, accentBlue())
	return encodePNG(canvas)
}

func RenderQuotesEmbed(quotes []map[string]any, stale bool, fetchedAt, message string) ([]byte, int, error) {
	normalized := normalizeQuotes(quotes)
	variant := "list"
	if len(normalized) == 0 {
		if strings.TrimSpace(message) != "" {
			variant = "fallback"
		} else {
			variant = "empty"
		}
	}

	layouts := buildQuoteLayouts(normalized)
	height := quotesHeightFromLayouts(layouts, stale, variant)
	canvas := newQuoteCanvas(height)

	cardX, cardY, cardW := 78, 84, 1044
	cardH := height - cardY - 50
	canvas.FillRect(cardX+10, cardY+12, cardW, cardH, color.RGBA{191, 219, 254, 255})
	canvas.FillBorderedRect(cardX, cardY, cardW, cardH, 3, color.RGBA{255, 255, 255, 255}, color.RGBA{147, 197, 253, 255})
	drawFlowerAccent(canvas, cardX+cardW-158, cardY+26)
	canvas.DrawText("Quote of the day", cardX+34, cardY+48, 34, color.RGBA{29, 78, 216, 255})
	canvas.DrawText("("+formatTime(fetchedAt)+")", cardX+34, cardY+78, 18, color.RGBA{96, 165, 250, 255})

	y := cardY + 118
	if stale {
		canvas.FillBorderedRect(cardX+34, y-8, cardW-68, 62, 2, color.RGBA{255, 251, 235, 255}, color.RGBA{245, 158, 11, 255})
		canvas.DrawText("Showing the last successful quote snapshot.", cardX+56, y+18, 18, color.RGBA{180, 83, 9, 255})
		canvas.DrawText("Updated: "+formatTime(fetchedAt), cardX+56, y+42, 16, color.RGBA{180, 83, 9, 255})
		y += 84
	}

	switch variant {
	case "empty":
		canvas.DrawText("No daily quotes are available right now.", cardX+34, cardY+184, 36, color.RGBA{29, 78, 216, 255})
		canvas.DrawText("Check back soon for a new quote snapshot.", cardX+34, cardY+238, 22, color.RGBA{51, 65, 85, 255})
	case "fallback":
		canvas.DrawText("Quote preview status", cardX+34, cardY+184, 34, color.RGBA{29, 78, 216, 255})
		lineY := cardY + 236
		for _, line := range wrapText(message, 62, 4) {
			canvas.DrawText(line, cardX+34, lineY, 26, color.RGBA{51, 65, 85, 255})
			lineY += 34
		}
		canvas.DrawText("The page still has a clean Discord preview.", cardX+34, lineY+40, 20, color.RGBA{37, 99, 235, 255})
	default:
		for i, layout := range layouts {
			sectionY := y
			label := layout.Entry.Category
			labelSize, quoteSize, lineHeight := 20, 18, 22
			if i == 0 {
				label = "Featured quote"
				labelSize, quoteSize, lineHeight = 24, 21, 28
			}
			canvas.DrawText(label, cardX+34, sectionY+28, labelSize, color.RGBA{29, 78, 216, 255})
			lineY := sectionY + 68
			for _, line := range layout.Lines {
				canvas.DrawText(line, cardX+52, lineY, quoteSize, color.RGBA{15, 23, 42, 255})
				lineY += lineHeight
			}
			canvas.DrawText("-- "+layout.Entry.Author, cardX+52, lineY+16, 20, color.RGBA{37, 99, 235, 255})
			y += layout.Height + 18
		}
	}

	png, err := encodePNG(canvas)
	return png, height, err
}

func QuotesPreviewDimensions(quotes []map[string]any, stale bool, message string) (int, int) {
	normalized := normalizeQuotes(quotes)
	variant := "list"
	if len(normalized) == 0 {
		if strings.TrimSpace(message) != "" {
			variant = "fallback"
		} else {
			variant = "empty"
		}
	}
	return PreviewWidth, quotesHeightFromLayouts(buildQuoteLayouts(normalized), stale, variant)
}

func RenderAnimeEmbed(preview AnimePreview) ([]byte, int, error) {
	variant := "list"
	if len(preview.Items) == 0 {
		if strings.TrimSpace(preview.Message) != "" || strings.TrimSpace(preview.ErrorCode) != "" {
			variant = "fallback"
		} else {
			variant = "empty"
		}
	}

	height := animeHeight(len(preview.Items), preview.Stale, variant)
	canvas := newAnimeCanvas(height, variant)

	cardX, cardY, cardW := 84, 56, 1032
	cardH := height - cardY*2
	canvas.FillRect(cardX+10, cardY+12, cardW, cardH, color.RGBA{191, 219, 254, 255})
	canvas.FillBorderedRect(cardX, cardY, cardW, cardH, 7, color.RGBA{255, 255, 255, 255}, color.RGBA{96, 165, 250, 255})
	canvas.DrawText("mirabellier.com / anime", cardX+42, cardY+34, 18, color.RGBA{37, 99, 235, 255})
	canvas.DrawText("my currently watching anime !!!", cardX+42, cardY+90, 40, color.RGBA{30, 64, 175, 255})

	y := cardY + 42 + 102
	if preview.Stale {
		canvas.FillBorderedRect(cardX+42, y-8, cardW-84, 66, 2, color.RGBA{255, 251, 235, 255}, color.RGBA{245, 158, 11, 255})
		canvas.DrawText("MyAnimeList did not answer on the latest refresh.", cardX+66, y+18, 20, color.RGBA{180, 83, 9, 255})
		canvas.DrawText("Showing the last successful snapshot from "+formatTime(preview.FetchedAt)+".", cardX+66, y+44, 18, color.RGBA{146, 64, 14, 255})
		y += 88
	}

	switch variant {
	case "empty":
		panelX, panelY, panelW, panelH := cardX+144, cardY+190, cardW-288, 160
		canvas.FillBorderedRect(panelX, panelY, panelW, panelH, 3, color.RGBA{255, 255, 255, 245}, color.RGBA{219, 234, 254, 255})
		canvas.DrawTextCentered("Nothing is marked as currently watching right now.", PreviewWidth/2, panelY+72, 30, color.RGBA{29, 78, 216, 255})
		canvas.DrawTextCentered("The list will update when a new anime gets added.", PreviewWidth/2, panelY+116, 22, color.RGBA{51, 65, 85, 255})
	case "fallback":
		msg := preview.Message
		if msg == "" {
			msg = "The MyAnimeList feed is unavailable right now."
		}
		textY := y + 24
		for _, line := range wrapText(msg, 36, 3) {
			canvas.DrawText(line, cardX+56, textY, 34, color.RGBA{29, 78, 216, 255})
			textY += 40
		}
		canvas.DrawText("The page still has a clean Discord preview.", cardX+56, textY+84, 22, color.RGBA{51, 65, 85, 255})
		drawAnimePosterFallback(canvas, cardX+cardW-352, cardY+164, 260, 340)
	default:
		const rowH = 116
		const rowGap = 10
		const posterW = 76
		const posterH = 106
		contentW := cardW - 84
		for i, item := range preview.Items {
			rowY := y + i*(rowH+rowGap)
			canvas.FillBorderedRect(cardX+32, rowY, contentW+20, rowH, 2, color.RGBA{248, 251, 255, 255}, color.RGBA{219, 234, 254, 255})
			// Poster image
			coverX, coverY := cardX+48, rowY+4
			if img := fetchImage(item.CoverImage); img != nil {
				canvas.DrawImage(img, coverX, coverY, posterW, posterH)
			} else {
				canvas.FillBorderedRect(coverX, coverY, posterW, posterH, 2, color.RGBA{239, 246, 255, 255}, color.RGBA{191, 219, 254, 255})
				canvas.DrawTextCentered("NO", coverX+posterW/2, coverY+posterH/2-6, 14, color.RGBA{96, 165, 250, 255})
				canvas.DrawTextCentered("ART", coverX+posterW/2, coverY+posterH/2+16, 14, color.RGBA{96, 165, 250, 255})
			}

			textX := coverX + posterW + 16

			// Number on the right
			numStr := fmt.Sprintf("#%02d", i+1)
			canvas.DrawTextRight(numStr, cardX+cardW-40, rowY+26, 16, mutedBlue())

			// Title (up to 2 lines)
			titleY := rowY + 26
			for _, line := range wrapText(item.Title, 36, 2) {
				canvas.DrawText(line, textX, titleY, 24, deepBlue())
				titleY += 28
			}

			// Badge row
			badgeY := rowY + 60
			badgeX := textX
			mediaType, progressStr, thirdBadge := animeBadgeTexts(item)
			badgeX = canvas.DrawBadge(badgeX, badgeY, mediaType, 15)
			if progressStr != "" {
				badgeX = canvas.DrawBadge(badgeX+6, badgeY, progressStr, 15)
			}
			if thirdBadge != "" {
				canvas.DrawBadge(badgeX+6, badgeY, thirdBadge, 15)
			}

			// Progress bar
			barY := rowY + 82
			barW := 340
			barH := 8
			pct, pctLabel := animeProgress(item)
			if pctLabel == "caught up" {
				canvas.DrawProgressBar(textX, barY, barW, barH, 1.0, accentBlue(), color.RGBA{219, 234, 254, 255}, accentBlue())
				canvas.DrawText("caught up", textX+barW+12, barY+barH+2, barH+5, accentBlue())
			} else {
				canvas.DrawProgressBar(textX, barY, barW, barH, pct, accentBlue(), color.RGBA{219, 234, 254, 255}, slate())
			}

			// Updated date
			canvas.DrawText(formatShortDate(item.UpdatedAt), textX, rowY+106, 13, mutedSlate())
		}
	}

	png, err := encodePNG(canvas)
	return png, height, err
}

func AnimePreviewDimensions(preview AnimePreview) (int, int) {
	variant := "list"
	if len(preview.Items) == 0 {
		if strings.TrimSpace(preview.Message) != "" || strings.TrimSpace(preview.ErrorCode) != "" {
			variant = "fallback"
		} else {
			variant = "empty"
		}
	}
	return PreviewWidth, animeHeight(len(preview.Items), preview.Stale, variant)
}

func newModernCanvas(height int) *RGBA {
	canvas := NewRGBA(PreviewWidth, height, color.RGBA{248, 251, 255, 255})
	canvas.FillRect(0, 0, PreviewWidth, height, color.RGBA{248, 251, 255, 255})
	canvas.FillRect(0, 0, PreviewWidth, 118, color.RGBA{239, 246, 255, 255})
	canvas.FillRect(0, height-104, PreviewWidth, 104, color.RGBA{241, 245, 249, 255})
	canvas.FillRect(0, 118, PreviewWidth, 3, color.RGBA{219, 234, 254, 255})
	canvas.FillRect(0, height-106, PreviewWidth, 2, color.RGBA{226, 232, 240, 255})
	return canvas
}

func drawPanel(canvas *RGBA, x, y, w, h int) {
	canvas.FillRect(x+8, y+10, w, h, color.RGBA{219, 234, 254, 80})
	canvas.FillBorderedRect(x, y, w, h, 3, color.RGBA{255, 255, 255, 250}, color.RGBA{191, 219, 254, 255})
	canvas.FillRect(x+3, y+3, w-6, 8, color.RGBA{239, 246, 255, 255})
}

func drawInitialBlock(canvas *RGBA, x, y, w, h int, label string) {
	canvas.FillBorderedRect(x, y, w, h, 3, color.RGBA{239, 246, 255, 255}, color.RGBA{191, 219, 254, 255})
	initial := "M"
	trimmed := strings.TrimSpace(label)
	if trimmed != "" {
		initial = strings.ToUpper(string([]rune(trimmed)[0]))
	}
	canvas.DrawTextCentered(initial, x+w/2, y+h/2+24, 58, deepBlue())
}

func deepBlue() color.Color {
	return color.RGBA{30, 64, 175, 255}
}

func accentBlue() color.Color {
	return color.RGBA{37, 99, 235, 255}
}

func mutedBlue() color.Color {
	return color.RGBA{96, 165, 250, 255}
}

func slate() color.Color {
	return color.RGBA{51, 65, 85, 255}
}

func mutedSlate() color.Color {
	return color.RGBA{100, 116, 139, 255}
}

func drawSoftBackground(canvas *RGBA) {
	canvas.FillRect(0, 0, PreviewWidth, PreviewHeight, color.RGBA{248, 251, 255, 255})
	canvas.FillRect(0, 0, PreviewWidth, PreviewHeight/2, color.RGBA{232, 243, 255, 255})
	canvas.FillRect(48, 36, 220, 180, color.RGBA{252, 207, 232, 80})
	canvas.FillRect(930, 24, 220, 180, color.RGBA{147, 197, 253, 80})
	canvas.FillRect(900, 440, 280, 160, color.RGBA{147, 197, 253, 55})
}

func drawQuestionBackground(canvas *RGBA) {
	for y := 0; y < PreviewHeight; y++ {
		t := float64(y) / float64(PreviewHeight-1)
		r := uint8(248*(1-t) + 232*t)
		g := uint8(251*(1-t) + 243*t)
		b := uint8(255*(1-t) + 255*t)
		canvas.FillRect(0, y, PreviewWidth, 1, color.RGBA{r, g, b, 255})
	}
}

func newQuoteCanvas(height int) *RGBA {
	canvas := NewRGBA(PreviewWidth, height, color.RGBA{234, 244, 255, 255})
	canvas.FillRect(0, 0, PreviewWidth, height, color.RGBA{234, 244, 255, 255})
	canvas.FillRect(0, 0, PreviewWidth, height/2, color.RGBA{248, 251, 255, 255})
	canvas.FillRect(0, height/2, PreviewWidth, height-height/2, color.RGBA{219, 234, 254, 255})
	return canvas
}

func drawFlowerAccent(canvas *RGBA, x, y int) {
	petal := color.RGBA{252, 207, 232, 255}
	canvas.FillCircle(x+28, y+20, 18, petal)
	canvas.FillCircle(x+58, y+20, 18, petal)
	canvas.FillCircle(x+28, y+50, 18, petal)
	canvas.FillCircle(x+58, y+50, 18, petal)
	canvas.FillCircle(x+43, y+35, 13, color.RGBA{147, 197, 253, 255})
	canvas.FillRect(x+82, y+34, 46, 3, color.RGBA{147, 197, 253, 255})
}

func newAnimeCanvas(height int, variant string) *RGBA {
	canvas := NewRGBA(PreviewWidth, height, color.RGBA{248, 251, 255, 255})
	for y := 0; y < height; y++ {
		t := float64(y) / float64(max(1, height-1))
		var r, g, b float64
		if t < 0.52 {
			local := t / 0.52
			r = 248*(1-local) + 239*local
			g = 251*(1-local) + 246*local
			b = 255
		} else {
			local := (t - 0.52) / 0.48
			r = 239*(1-local) + 219*local
			g = 246*(1-local) + 234*local
			b = 255*(1-local) + 254*local
		}
		canvas.FillRect(0, y, PreviewWidth, 1, color.RGBA{uint8(r), uint8(g), uint8(b), 255})
	}
	if variant == "empty" {
		canvas.FillCircle(200, 170, 150, color.RGBA{191, 219, 254, 115})
		canvas.FillCircle(PreviewWidth-130, height-95, 175, color.RGBA{147, 197, 253, 72})
	} else {
		canvas.FillCircle(170, 150, 140, color.RGBA{191, 219, 254, 115})
		canvas.FillCircle(PreviewWidth-130, height-110, 170, color.RGBA{147, 197, 253, 72})
		canvas.FillCircle(PreviewWidth-230, 130, 88, color.RGBA{219, 234, 254, 178})
	}
	return canvas
}

func drawAnimePosterFallback(canvas *RGBA, x, y, w, h int) {
	canvas.FillBorderedRect(x, y, w, h, 4, color.RGBA{239, 246, 255, 255}, color.RGBA{147, 197, 253, 255})
	canvas.FillRect(x+24, y+24, w-48, h-118, color.RGBA{219, 234, 254, 255})
	canvas.DrawTextCentered("MAL", x+w/2, y+h/2-20, 42, color.RGBA{29, 78, 216, 255})
	canvas.DrawTextCentered("preview", x+w/2, y+h/2+28, 24, color.RGBA{59, 130, 246, 255})
}

func drawBlueBackground(canvas *RGBA, h int) {
	canvas.FillRect(0, 0, PreviewWidth, h, color.RGBA{248, 251, 255, 255})
	canvas.FillRect(0, h/2, PreviewWidth, h/2, color.RGBA{219, 234, 254, 255})
	canvas.FillRect(40, 60, 280, 190, color.RGBA{191, 219, 254, 95})
	canvas.FillRect(940, h-250, 230, 190, color.RGBA{147, 197, 253, 75})
}

type textCandidate struct {
	MaxChars   int
	MaxLines   int
	Size       int
	LineHeight int
}

type textLayout struct {
	Lines      []string
	Size       int
	LineHeight int
}

func chooseTextLayout(text string, candidates []textCandidate, maxHeight int) textLayout {
	for _, candidate := range candidates {
		lines := wrapText(text, candidate.MaxChars, candidate.MaxLines)
		height := candidate.Size + max(0, len(lines)-1)*candidate.LineHeight
		if height <= maxHeight {
			return textLayout{Lines: lines, Size: candidate.Size, LineHeight: candidate.LineHeight}
		}
	}
	last := candidates[len(candidates)-1]
	return textLayout{Lines: wrapText(text, last.MaxChars, last.MaxLines), Size: last.Size, LineHeight: last.LineHeight}
}

type quoteEntry struct {
	Category string
	Quote    string
	Author   string
}

type quoteLayout struct {
	Entry  quoteEntry
	Lines  []string
	Height int
}

func normalizeQuotes(values []map[string]any) []quoteEntry {
	entries := make([]quoteEntry, 0, len(values))
	for _, value := range values {
		quote, _ := value["quote"].(string)
		if strings.TrimSpace(quote) == "" {
			continue
		}
		category, _ := value["category"].(string)
		author, _ := value["author"].(string)
		if category == "" {
			category = "Quote"
		}
		if author == "" {
			author = "Unknown"
		}
		entries = append(entries, quoteEntry{Category: category, Quote: quote, Author: author})
	}
	return entries
}

func buildQuoteLayouts(entries []quoteEntry) []quoteLayout {
	layouts := make([]quoteLayout, 0, len(entries))
	for i, entry := range entries {
		maxChars, maxLines, lineHeight := 74, 0, 22
		if i == 0 {
			lineHeight = 28
		}
		lines := wrapText("\""+entry.Quote+"\"", maxChars, maxLines)
		authorY := 68 + max(0, len(lines)-1)*lineHeight + 38
		if i > 0 {
			authorY = 68 + max(0, len(lines)-1)*lineHeight + 34
		}
		layouts = append(layouts, quoteLayout{
			Entry:  entry,
			Lines:  lines,
			Height: authorY + 22,
		})
	}
	return layouts
}

func quotesHeightFromLayouts(layouts []quoteLayout, stale bool, variant string) int {
	if variant != "list" {
		return 700
	}
	contentHeight := 0
	for i, layout := range layouts {
		if i > 0 {
			contentHeight += 18
		}
		contentHeight += layout.Height
	}
	cardHeight := 118 + contentHeight + 40
	if stale {
		cardHeight += 84
	}
	height := 84 + cardHeight + 60
	if height < 760 {
		height = 760
	}
	return height
}

func animeHeight(count int, stale bool, variant string) int {
	switch variant {
	case "fallback":
		return 720
	case "empty":
		if stale {
			return 648
		}
		return 560
	default:
		height := 56*2 + 42 + 118 + count*116 + max(0, count-1)*10 + 64
		if stale {
			height += 88
		}
		if height < 720 {
			height = 720
		}
		return height
	}
}

func wrapText(value string, maxChars, maxLines int) []string {
	text := strings.Join(strings.Fields(value), " ")
	if text == "" {
		return []string{""}
	}
	words := strings.Split(text, " ")
	lines := []string{}
	current := ""
	for _, word := range words {
		candidate := word
		if current != "" {
			candidate = current + " " + word
		}
		if len(candidate) <= maxChars {
			current = candidate
			continue
		}
		if current != "" {
			lines = append(lines, current)
			current = word
		} else {
			lines = append(lines, limit(word, maxChars))
		}
		if maxLines > 0 && len(lines) == maxLines {
			lines[len(lines)-1] = limit(lines[len(lines)-1], maxChars)
			return lines
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
		lines[len(lines)-1] = limit(lines[len(lines)-1], maxChars)
	}
	return lines
}

func drawScaledText(dst *image.RGBA, text string, x, baseline, size int, clr color.Color) {
	if text == "" {
		return
	}
	scale := max(1, size/13)
	face := basicfont.Face7x13
	w := font.MeasureString(face, text).Ceil()
	tmp := image.NewRGBA(image.Rect(0, 0, w+4, 18))
	drawer := &font.Drawer{
		Dst:  tmp,
		Src:  image.NewUniform(clr),
		Face: face,
		Dot:  fixed.P(2, 13),
	}
	drawer.DrawString(text)
	scaled := imaging.Resize(tmp, tmp.Bounds().Dx()*scale, tmp.Bounds().Dy()*scale, imaging.NearestNeighbor)
	draw.Draw(dst, image.Rect(x, baseline-size, x+scaled.Bounds().Dx(), baseline-size+scaled.Bounds().Dy()), scaled, image.Point{}, draw.Over)
}

func textWidth(text string, size int) int {
	scale := max(1, size/13)
	return font.MeasureString(basicfont.Face7x13, text).Ceil() * scale
}

func encodePNG(canvas *RGBA) ([]byte, error) {
	var buf bytes.Buffer
	if err := canvas.EncodePNG(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func limit(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxLen {
		return value
	}
	if maxLen <= 3 {
		return value[:maxLen]
	}
	return strings.TrimSpace(value[:maxLen-3]) + "..."
}

func formatTime(value string) string {
	if value == "" {
		return "unknown time"
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return "unknown time"
	}
	return t.UTC().Format("Jan 2, 2006 15:04 UTC")
}

func formatDate(value string) string {
	if value == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return ""
	}
	return t.UTC().Format("Jan 2, 2006")
}

func fetchImage(url string) image.Image {
	if strings.TrimSpace(url) == "" {
		return nil
	}
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")
	req.Header.Set("User-Agent", "Mirabellier/1.0 (+https://mirabellier.com)")
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil
	}
	img, _, err := image.Decode(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil
	}
	return img
}

func animeBadgeTexts(item AnimeItem) (mediaType, progress, third string) {
	mediaType = strings.TrimSpace(strings.ReplaceAll(item.MediaType, "_", " "))
	if mediaType == "" {
		mediaType = "Anime"
	}
	mediaType = strings.Title(mediaType)

	if item.TotalEpisodes > 0 {
		progress = fmt.Sprintf("%d / %d eps", item.WatchedEpisodes, item.TotalEpisodes)
	} else {
		progress = fmt.Sprintf("%d watched", item.WatchedEpisodes)
	}

	if item.Season != "" && item.SeasonYear > 0 {
		third = strings.Title(item.Season) + " " + strconv.Itoa(item.SeasonYear)
	} else if item.Score > 0 {
		third = fmt.Sprintf("Score %d", item.Score)
	}
	return
}

func animeProgress(item AnimeItem) (pct float64, label string) {
	if item.TotalEpisodes > 0 {
		if item.WatchedEpisodes >= item.TotalEpisodes {
			return 1.0, "caught up"
		}
		pct := float64(item.WatchedEpisodes) / float64(item.TotalEpisodes)
		return pct, fmt.Sprintf("%.0f%%", pct*100)
	}
	return 1.0, fmt.Sprintf("%d watched", item.WatchedEpisodes)
}

func formatShortDate(value string) string {
	if value == "" {
		return "Updated recently"
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return "Updated recently"
	}
	return "Updated " + t.UTC().Format("Jan 2, 2006")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
