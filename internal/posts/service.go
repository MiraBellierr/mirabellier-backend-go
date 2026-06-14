package posts

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

const maxTagsList = 10
const maxTagLen = 20
const maxCommentLen = 2000

func buildNestedComments(flat []Comment) []Comment {
	byID := make(map[string]*Comment)
	var roots []Comment
	for i := range flat {
		byID[flat[i].ID] = &flat[i]
	}
	for i := range flat {
		if flat[i].ParentID != nil {
			if parent, ok := byID[*flat[i].ParentID]; ok {
				parent.Children = append(parent.Children, flat[i])
				continue
			}
		}
		roots = append(roots, flat[i])
	}
	return roots
}

func sanitizeTags(tags []string) []string {
	var result []string
	seen := make(map[string]bool)
	for _, tag := range tags {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if len(tag) == 0 || len(tag) > maxTagLen || seen[tag] {
			continue
		}
		valid := true
		for _, c := range tag {
			if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		if len(result) >= maxTagsList {
			break
		}
		seen[tag] = true
		result = append(result, tag)
	}
	return result
}

func ListPosts(db *sql.DB) ([]Post, error) {
	rows, err := db.Query(`
		SELECT p.id, p.title, p.content, p.userId, p.author, p.shortDescription, p.thumbnail,
		       p.tags, p.likes, p.comments, p.createdAt, p.updatedAt,
		       u.username, u.avatar
		FROM posts p
		LEFT JOIN users u ON u.id = p.userId
		ORDER BY p.createdAt DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		var tagsJSON, likesJSON, commentsJSON sql.NullString
		var authorName, authorAvatar sql.NullString
		var author sql.NullString
		err := rows.Scan(&p.ID, &p.Title, &p.ContentRaw, &p.UserID, &author,
			&p.ShortDescription, &p.Thumbnail,
			&tagsJSON, &likesJSON, &commentsJSON,
			&p.CreatedAt, &p.UpdatedAt,
			&authorName, &authorAvatar)
		if err != nil {
			return nil, err
		}
		if author.Valid {
			p.Author = author.String
		}
		// Node.js: row.userId ? row.authorName || row.author || "Unknown" : row.author || "Unknown"
		if p.UserID != nil && authorName.Valid {
			p.Author = authorName.String
		}
		if p.Author == "" {
			p.Author = "Unknown"
		}
		if authorName.Valid {
			p.AuthorName = &authorName.String
		}
		if authorAvatar.Valid {
			p.AuthorAvatar = &authorAvatar.String
		}

		p.Content = json.RawMessage(p.ContentRaw)
		if tagsJSON.Valid {
			json.Unmarshal([]byte(tagsJSON.String), &p.Tags)
		}
		if likesJSON.Valid {
			json.Unmarshal([]byte(likesJSON.String), &p.Likes)
		}
		if commentsJSON.Valid {
			p.Comments = parseAndEnrichComments([]byte(commentsJSON.String), db)
		}

		posts = append(posts, p)
	}

	return posts, nil
}

func GetPost(db *sql.DB, id string) (*Post, error) {
	p := &Post{}
	var tagsJSON, likesJSON, commentsJSON sql.NullString
	var authorName, authorAvatar sql.NullString
	var author sql.NullString
	err := db.QueryRow(`
		SELECT p.id, p.title, p.content, p.userId, p.author, p.shortDescription, p.thumbnail,
		       p.tags, p.likes, p.comments, p.createdAt, p.updatedAt,
		       u.username, u.avatar
		FROM posts p
		LEFT JOIN users u ON u.id = p.userId
		WHERE p.id = ?
	`, id).Scan(&p.ID, &p.Title, &p.ContentRaw, &p.UserID, &author,
		&p.ShortDescription, &p.Thumbnail,
		&tagsJSON, &likesJSON, &commentsJSON,
		&p.CreatedAt, &p.UpdatedAt,
		&authorName, &authorAvatar)
	if err != nil {
		return nil, err
	}
	if author.Valid {
		p.Author = author.String
	}
	// Node.js: row.userId ? row.authorName || row.author || "Unknown" : row.author || "Unknown"
	if p.UserID != nil && authorName.Valid {
		p.Author = authorName.String
	}
	if p.Author == "" {
		p.Author = "Unknown"
	}
	if authorName.Valid {
		p.AuthorName = &authorName.String
	}
	if authorAvatar.Valid {
		p.AuthorAvatar = &authorAvatar.String
	}

	p.Content = json.RawMessage(p.ContentRaw)
	if tagsJSON.Valid {
		json.Unmarshal([]byte(tagsJSON.String), &p.Tags)
	}
	if likesJSON.Valid {
		json.Unmarshal([]byte(likesJSON.String), &p.Likes)
	}
	if commentsJSON.Valid {
		p.Comments = parseAndEnrichComments([]byte(commentsJSON.String), db)
	}

	return p, nil
}

func parseAndEnrichComments(raw []byte, db *sql.DB) []Comment {
	var flat []Comment
	json.Unmarshal(raw, &flat)

	for i, c := range flat {
		if c.UserID != nil && c.User == nil {
			user := lookupCommentUser(db, *c.UserID)
			if user != nil {
				flat[i].User = user
			}
		}
	}

	return buildNestedComments(flat)
}

func lookupCommentUser(db *sql.DB, userID string) *CommentUser {
	var username string
	var avatar sql.NullString
	err := db.QueryRow("SELECT username, avatar FROM users WHERE id = ?", userID).Scan(&username, &avatar)
	if err != nil {
		return nil
	}
	u := &CommentUser{ID: userID, Username: username}
	if avatar.Valid {
		u.Avatar = &avatar.String
	}
	return u
}

func CreatePost(db *sql.DB, input *CreatePostInput, postID string) (*Post, error) {
	// Node.js: title = req.body.title || req.body.name || "Untitled"
	title := input.Title
	if title == "" {
		title = input.Name
	}
	if title == "" {
		title = "Untitled"
	}

	// Node.js: content = req.body.content || req.body.body || {}
	var content interface{}
	if len(input.Content) > 0 {
		json.Unmarshal(input.Content, &content)
	} else if len(input.Body) > 0 {
		json.Unmarshal(input.Body, &content)
	}
	if content == nil {
		content = map[string]interface{}{}
	}
	contentJSON, _ := json.Marshal(content)

	sanitizedTags := sanitizeTags(input.Tags)
	tagsJSON, _ := json.Marshal(sanitizedTags)
	emptyArr := "[]"

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`
		INSERT INTO posts (id, title, content, userId, author, shortDescription, thumbnail, tags, likes, comments, createdAt, updatedAt)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, postID, title, string(contentJSON), input.UserID, input.Author,
		input.ShortDescription, input.Thumbnail, string(tagsJSON), emptyArr, emptyArr, now, now)
	if err != nil {
		return nil, err
	}

	return GetPost(db, postID)
}

func UpdatePost(db *sql.DB, id string, input *UpdatePostInput, userID string) (*Post, error) {
	var ownerID sql.NullString
	err := db.QueryRow("SELECT userId FROM posts WHERE id = ?", id).Scan(&ownerID)
	if err != nil {
		return nil, fmt.Errorf("post not found")
	}
	if !ownerID.Valid || ownerID.String != userID {
		return nil, fmt.Errorf("not authorized")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if input.Title != nil {
		db.Exec("UPDATE posts SET title = ?, updatedAt = ? WHERE id = ?", *input.Title, now, id)
	}
	if input.Content != nil {
		contentJSON, _ := json.Marshal(*input.Content)
		// Content is a json.RawMessage that may need unquoting if it was a JSON string
		db.Exec("UPDATE posts SET content = ?, updatedAt = ? WHERE id = ?", string(contentJSON), now, id)
	}
	if input.ShortDescription != nil {
		db.Exec("UPDATE posts SET shortDescription = ?, updatedAt = ? WHERE id = ?", *input.ShortDescription, now, id)
	}
	if input.Thumbnail != nil {
		db.Exec("UPDATE posts SET thumbnail = ?, updatedAt = ? WHERE id = ?", *input.Thumbnail, now, id)
	}
	if input.Tags != nil {
		tagsJSON, _ := json.Marshal(sanitizeTags(input.Tags))
		db.Exec("UPDATE posts SET tags = ?, updatedAt = ? WHERE id = ?", string(tagsJSON), now, id)
	}

	return GetPost(db, id)
}

func DeletePost(db *sql.DB, id string, userID string) error {
	var ownerID sql.NullString
	err := db.QueryRow("SELECT userId FROM posts WHERE id = ?", id).Scan(&ownerID)
	if err != nil {
		return fmt.Errorf("post not found")
	}
	if !ownerID.Valid || ownerID.String != userID {
		return fmt.Errorf("not authorized")
	}
	_, err = db.Exec("DELETE FROM posts WHERE id = ?", id)
	return err
}

func ToggleLike(db *sql.DB, postID string, identityType, identityKey, action string) (*LikeResponse, error) {
	var likesJSON sql.NullString
	db.QueryRow("SELECT likes FROM posts WHERE id = ?", postID).Scan(&likesJSON)

	var likes []string
	if likesJSON.Valid {
		json.Unmarshal([]byte(likesJSON.String), &likes)
	}
	if likes == nil {
		likes = []string{}
	}

	fullKey := identityKey

	if action == "like" {
		found := false
		for _, l := range likes {
			if l == fullKey {
				found = true
				break
			}
		}
		if !found {
			likes = append(likes, fullKey)
		}
	} else {
		var filtered []string
		for _, l := range likes {
			if l != fullKey {
				filtered = append(filtered, l)
			}
		}
		likes = filtered
	}

	newJSON, _ := json.Marshal(likes)
	db.Exec("UPDATE posts SET likes = ?, updatedAt = datetime('now') WHERE id = ?", string(newJSON), postID)

	liked := false
	for _, l := range likes {
		if l == fullKey {
			liked = true
			break
		}
	}

	return &LikeResponse{Likes: likes, Liked: liked}, nil
}

func AddComment(db *sql.DB, postID string, input *CreateCommentInput, userID *string, username *string, avatar *string) (*Comment, error) {
	if len(input.Text) > maxCommentLen {
		return nil, fmt.Errorf("comment too long (max %d chars)", maxCommentLen)
	}

	var commentsJSON sql.NullString
	db.QueryRow("SELECT comments FROM posts WHERE id = ?", postID).Scan(&commentsJSON)

	var comments []Comment
	if commentsJSON.Valid {
		json.Unmarshal([]byte(commentsJSON.String), &comments)
	}
	if comments == nil {
		comments = []Comment{}
	}

	commentID := fmt.Sprintf("comment_%d_%d", time.Now().UnixMilli(), rand.Intn(10000))
	now := time.Now().UTC().Format(time.RFC3339)

	comment := Comment{
		ID:        commentID,
		Text:      input.Text,
		ParentID:  input.ParentID,
		UserID:    userID,
		CreatedAt: now,
	}
	if userID != nil && username != nil {
		comment.User = &CommentUser{
			ID:       *userID,
			Username: *username,
			Avatar:   avatar,
		}
	}

	comments = append(comments, comment)

	newJSON, _ := json.Marshal(comments)
	db.Exec("UPDATE posts SET comments = ?, updatedAt = datetime('now') WHERE id = ?", string(newJSON), postID)

	return &comment, nil
}

func ListTags(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT tags FROM posts")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := make(map[string]bool)
	for rows.Next() {
		var tagsJSON sql.NullString
		rows.Scan(&tagsJSON)
		if tagsJSON.Valid {
			var tags []string
			json.Unmarshal([]byte(tagsJSON.String), &tags)
			for _, t := range tags {
				seen[t] = true
			}
		}
	}

	var result []string
	for t := range seen {
		result = append(result, t)
	}
	return result, nil
}
