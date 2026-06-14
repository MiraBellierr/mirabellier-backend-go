package guestbook

import "time"

type GuestbookEntry struct {
	ID        string     `json:"id"`
	Author    string     `json:"author"`
	Message   string     `json:"message"`
	Website   *string    `json:"website,omitempty"`
	Mood      string     `json:"mood"`
	X         int        `json:"x"`
	Y         int        `json:"y"`
	CreatedAt string     `json:"createdAt"`
	User      *EntryUser `json:"user,omitempty"`
}

type EntryUser struct {
	ID       string  `json:"id,omitempty"`
	Username string  `json:"username,omitempty"`
	Avatar   *string `json:"avatar,omitempty"`
}

type CreateEntryInput struct {
	Name    string  `json:"name" binding:"required"`
	Message string  `json:"message" binding:"required"`
	Website *string `json:"website,omitempty"`
	Mood    string  `json:"mood,omitempty"`
	X       int     `json:"x,omitempty"`
	Y       int     `json:"y,omitempty"`
}

type UpdatePositionInput struct {
	X int `json:"x" binding:"required"`
	Y int `json:"y" binding:"required"`
}

var validMoods = map[string]bool{
	"sparkly": true,
	"cozy":    true,
	"sleepy":  true,
	"sunny":   true,
	"chaotic": true,
}

// Board dimensions (3x scale of 1199x678 image)
const BoardLogicalWidth = 3597
const BoardLogicalHeight = 2034
const BoardPadding = 48
const BoardColumns = 6

func calculateFallbackPosition(index int) (int, int) {
	colW := (BoardLogicalWidth - BoardPadding*2) / BoardColumns
	row := index / BoardColumns
	col := index % BoardColumns
	x := BoardPadding + col*colW + colW/2
	y := BoardPadding + row*300 + 150 // 300px row spacing
	return x, y
}

func generateEntryID() string {
	return "gb_" + time.Now().UTC().Format("20060102150405") + "_" + randomHex(4)
}

func randomHex(n int) string {
	const letters = "abcdef0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}
