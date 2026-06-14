package auth

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func GetUserByToken(db *sql.DB, token string, cfg *Config) (*User, error) {
	user := &User{}
	err := db.QueryRow(`
		SELECT u.id, u.username, u.discordId, u.avatar, u.banner,
		       u.bio, u.location, u.website, u.createdAt
		FROM sessions s
		JOIN users u ON u.id = s.userId
		WHERE s.token = ?
	`, token).Scan(
		&user.ID, &user.Username, &user.DiscordID, &user.Avatar, &user.Banner,
		&user.Bio, &user.Location, &user.Website, &user.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	computeUserRoles(user, cfg)
	return user, nil
}

func GetUserByID(db *sql.DB, id string, cfg *Config) (*User, error) {
	user := &User{}
	err := db.QueryRow(`
		SELECT id, username, discordId, avatar, banner, bio, location, website, createdAt
		FROM users WHERE id = ?
	`, id).Scan(
		&user.ID, &user.Username, &user.DiscordID, &user.Avatar, &user.Banner,
		&user.Bio, &user.Location, &user.Website, &user.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	computeUserRoles(user, cfg)
	return user, nil
}

func GetUserByUsername(db *sql.DB, username string, cfg *Config) (*User, error) {
	user := &User{}
	err := db.QueryRow(`
		SELECT id, username, discordId, avatar, banner, bio, location, website, createdAt
		FROM users WHERE username = ?
	`, username).Scan(
		&user.ID, &user.Username, &user.DiscordID, &user.Avatar, &user.Banner,
		&user.Bio, &user.Location, &user.Website, &user.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	computeUserRoles(user, cfg)
	return user, nil
}

func FindOrCreateDiscordUser(db *sql.DB, discordID, discordUsername, globalName, avatarHash, bannerHash string, cfg *Config) (*User, error) {
	var avatarURL string
	if avatarHash != "" {
		avatarURL = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", discordID, avatarHash)
	}
	var bannerURL string
	if bannerHash != "" {
		bannerURL = fmt.Sprintf("https://cdn.discordapp.com/banners/%s/%s.png", discordID, bannerHash)
	}

	var userID string
	err := db.QueryRow("SELECT id FROM users WHERE discordId = ?", discordID).Scan(&userID)
	if err == nil {
		db.Exec("UPDATE users SET avatar = ?, banner = ? WHERE id = ?",
			avatarURL, bannerURL, userID)
		return GetUserByID(db, userID, cfg)
	}

	displayName := globalName
	if displayName == "" {
		displayName = discordUsername
	}

	username := strings.ToLower(strings.ReplaceAll(displayName, " ", "_"))
	var exists int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", username).Scan(&exists)
	if exists > 0 {
		username = fmt.Sprintf("%s_%s", username, discordID[:6])
	}

	id := fmt.Sprintf("user_%d", time.Now().UnixMilli())
	_, err = db.Exec(`
		INSERT INTO users (id, username, discordId, avatar, banner, createdAt)
		VALUES (?, ?, ?, ?, ?, datetime('now'))
	`, id, username, discordID, avatarURL, bannerURL)
	if err != nil {
		return nil, err
	}

	return GetUserByID(db, id, cfg)
}

func computeUserRoles(user *User, cfg *Config) {
	roles := []string{"user"}
	if user.DiscordID != nil && IsOwner(user, cfg) {
		roles = append(roles, "admin", "owner")
	}
	user.Roles = roles
	user.Permissions = &UserPermissions{
		AdminPanel:               len(roles) > 1,
		ModerateGuestbook:        len(roles) > 1,
		ModerateQuestionOfTheDay: len(roles) > 1,
		ManageShrines:            len(roles) > 1,
	}
}

func ToUserPublic(user *User) *UserPublic {
	return &UserPublic{
		ID:       user.ID,
		Username: user.Username,
		Avatar:   user.Avatar,
		Banner:   user.Banner,
		Bio:      user.Bio,
		Location: user.Location,
		Website:  user.Website,
	}
}
