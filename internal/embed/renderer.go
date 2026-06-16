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
	canvas := newModernCanvas(PreviewHeight)
	cardX, cardY, cardW, cardH := 104, 92, 992, 446
	drawPanel(canvas, cardX, cardY, cardW, cardH)
	canvas.DrawText("mirabellier.com / question of the day", cardX+44, cardY+52, 18, mutedBlue())

	if strings.TrimSpace(prompt) == "" {
		canvas.DrawTextCentered("No active question yet.", 600, 326, 44, deepBlue())
		return encodePNG(canvas)
	}

	layout := chooseTextLayout(prompt, []textCandidate{
		{26, 3, 58, 64}, {32, 4, 50, 56}, {38, 5, 42, 48},
		{46, 6, 34, 40}, {56, 7, 28, 34},
	}, 300)
	textHeight := layout.Size + max(0, len(layout.Lines)-1)*layout.LineHeight
	y := cardY + 240 - textHeight/2 + layout.Size
	for _, line := range layout.Lines {
		canvas.DrawTextCentered(line, PreviewWidth/2, y, layout.Size, deepBlue())
		y += layout.LineHeight
	}
	canvas.DrawTextCentered("Answer at mirabellier.com", 600, cardY+cardH-46, 19, mutedBlue())

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

	height := quotesHeight(len(normalized), stale, variant)
	canvas := newModernCanvas(height)

	cardX, cardY, cardW := 78, 84, 1044
	cardH := height - cardY - 50
	drawPanel(canvas, cardX, cardY, cardW, cardH)
	canvas.DrawText("Quote of the day", cardX+34, cardY+54, 34, deepBlue())
	canvas.DrawText("mirabellier.com / quotes", cardX+34, cardY+90, 18, mutedBlue())

	y := cardY + 132
	if stale {
		canvas.FillBorderedRect(cardX+34, y, cardW-68, 64, 2, color.RGBA{255, 251, 235, 255}, color.RGBA{253, 186, 116, 255})
		canvas.DrawText("Showing the last successful snapshot from "+formatTime(fetchedAt)+".", cardX+54, y+40, 18, color.RGBA{180, 83, 9, 255})
		y += 86
	}

	switch variant {
	case "empty":
		canvas.DrawText("No daily quotes are available right now.", cardX+34, y+72, 34, deepBlue())
		canvas.DrawText("The next quote snapshot will appear here once it is ready.", cardX+34, y+124, 22, slate())
	case "fallback":
		canvas.DrawText("Quote preview status", cardX+34, y+48, 30, deepBlue())
		for _, line := range wrapText(message, 62, 4) {
			canvas.DrawText(line, cardX+34, y+98, 28, slate())
			y += 34
		}
		canvas.DrawText("The page will share cleanly again after the next quote snapshot.", cardX+34, y+148, 22, slate())
	default:
		for i, entry := range normalized {
			sectionY := y
			label := entry.Category
			if i == 0 {
				label = "Featured quote"
			}
			canvas.DrawText(label, cardX+34, sectionY+28, 24, accentBlue())
			lines := wrapText("\""+entry.Quote+"\"", 76, 5)
			lineY := sectionY + 70
			for _, line := range lines {
				canvas.DrawText(line, cardX+52, lineY, 21, slate())
				lineY += 28
			}
			canvas.DrawText("-- "+entry.Author, cardX+52, lineY+16, 20, mutedBlue())
			y = lineY + 60
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
	return PreviewWidth, quotesHeight(len(normalized), stale, variant)
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
	canvas := newModernCanvas(height)

	cardX, cardY, cardW := 84, 56, 1032
	drawPanel(canvas, cardX, cardY, cardW, height-cardY*2)
	canvas.DrawText("mirabellier.com / anime", cardX+42, cardY+42, 18, mutedBlue())
	canvas.DrawText("Currently watching anime", cardX+42, cardY+96, 40, deepBlue())

	y := cardY + 160
	if preview.Stale {
		canvas.FillBorderedRect(cardX+42, y, cardW-84, 66, 2, color.RGBA{255, 251, 235, 255}, color.RGBA{253, 186, 116, 255})
		canvas.DrawText("MyAnimeList did not answer on the latest refresh.", cardX+66, y+30, 20, color.RGBA{180, 83, 9, 255})
		canvas.DrawText("Showing the last successful snapshot from "+formatTime(preview.FetchedAt)+".", cardX+66, y+54, 18, color.RGBA{146, 64, 14, 255})
		y += 88
	}

	switch variant {
	case "empty":
		canvas.FillBorderedRect(cardX+144, y+42, cardW-288, 160, 3, color.RGBA{248, 251, 255, 255}, color.RGBA{219, 234, 254, 255})
		canvas.DrawTextCentered("Nothing is marked as currently watching right now.", PreviewWidth/2, y+120, 30, deepBlue())
		canvas.DrawTextCentered("The page is ready whenever the next anime gets added.", PreviewWidth/2, y+164, 22, slate())
	case "fallback":
		msg := preview.Message
		if msg == "" {
			msg = "The MyAnimeList feed is unavailable right now."
		}
		for _, line := range wrapText(msg, 38, 2) {
			canvas.DrawText(line, cardX+42, y+18, 34, deepBlue())
			y += 38
		}
		canvas.DrawText("The page still shares cleanly.", cardX+70, y+96, 24, slate())
		drawInitialBlock(canvas, cardX+cardW-382, cardY+140, 320, 390, "anime")
	default:
		for i, item := range preview.Items {
			rowY := y + i*176
			canvas.FillBorderedRect(cardX+32, rowY, cardW-64, 146, 2, color.RGBA{248, 251, 255, 255}, color.RGBA{226, 232, 240, 255})
			if img := fetchImage(item.CoverImage); img != nil {
				canvas.DrawImage(img, cardX+42, rowY+12, 90, 126)
			} else {
				canvas.FillBorderedRect(cardX+42, rowY+12, 90, 126, 2, color.RGBA{239, 246, 255, 255}, color.RGBA{191, 219, 254, 255})
				canvas.DrawTextCentered("NO", cardX+87, rowY+60, 14, color.RGBA{96, 165, 250, 255})
				canvas.DrawTextCentered("ART", cardX+87, rowY+82, 14, color.RGBA{96, 165, 250, 255})
			}
			textX := cardX + 158
			titleLines := wrapText(item.Title, 42, 2)
			titleY := rowY + 38
			for _, line := range titleLines {
				canvas.DrawText(line, textX, titleY, 28, deepBlue())
				titleY += 32
			}
			canvas.DrawText(formatAnimeSummary(item), textX, rowY+96, 22, slate())
			canvas.DrawText(formatAnimeDetails(item), textX, rowY+126, 20, mutedBlue())
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

func quotesHeight(count int, stale bool, variant string) int {
	if variant != "list" {
		return 700
	}
	height := 260 + count*160
	if stale {
		height += 88
	}
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
		height := 56*2 + 42 + 118 + count*154 + max(0, count-1)*22 + 64
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

func formatAnimeSummary(item AnimeItem) string {
	mediaType := strings.TrimSpace(strings.ReplaceAll(item.MediaType, "_", " "))
	if mediaType == "" {
		mediaType = "Anime"
	}
	progress := fmt.Sprintf("%d watched", item.WatchedEpisodes)
	if item.TotalEpisodes > 0 {
		progress = fmt.Sprintf("%d / %d episodes", item.WatchedEpisodes, item.TotalEpisodes)
	}
	parts := []string{strings.Title(mediaType), progress}
	if item.Score > 0 {
		parts = append(parts, fmt.Sprintf("Score %d/10", item.Score))
	}
	return limit(strings.Join(parts, " - "), 74)
}

func formatAnimeDetails(item AnimeItem) string {
	season := "Season unknown"
	if item.Season != "" && item.SeasonYear > 0 {
		season = strings.Title(item.Season) + fmt.Sprintf(" %d", item.SeasonYear)
	}
	return limit("Last update "+formatTime(item.UpdatedAt)+" - "+season, 82)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
