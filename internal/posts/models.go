package posts

import "encoding/json"

// Post matches the frontend's Post type exactly.
type Post struct {
	ID               string          `json:"id"`
	Title            string          `json:"title"`
	ContentRaw       string          `json:"-"`
	UserID           *string         `json:"userId,omitempty"`
	Author           string          `json:"author"`
	AuthorName       *string         `json:"authorName,omitempty"`
	AuthorAvatar     *string         `json:"authorAvatar,omitempty"`
	CreatedAt        string          `json:"createdAt"`
	UpdatedAt        *string         `json:"updatedAt,omitempty"`
	Content          json.RawMessage `json:"content"`
	ShortDescription *string         `json:"shortDescription,omitempty"`
	Thumbnail        *string         `json:"thumbnail,omitempty"`
	Tags             []string        `json:"tags,omitempty"`
	Likes            []string        `json:"likes,omitempty"`
	Comments         []Comment       `json:"comments,omitempty"`
}

type Comment struct {
	ID        string       `json:"id"`
	Text      string       `json:"text"`
	ParentID  *string      `json:"parentId,omitempty"`
	UserID    *string      `json:"userId,omitempty"`
	CreatedAt string       `json:"createdAt"`
	User      *CommentUser `json:"user,omitempty"`
	Children  []Comment    `json:"children,omitempty"`
}

type CommentUser struct {
	ID       string  `json:"id"`
	Username string  `json:"username"`
	Avatar   *string `json:"avatar,omitempty"`
}

type CreatePostInput struct {
	Title            string          `json:"title"`
	Name             string          `json:"name,omitempty"`
	Content          json.RawMessage `json:"content"`
	Body             json.RawMessage `json:"body,omitempty"`
	UserID           *string         `json:"userId,omitempty"`
	Author           string          `json:"author"`
	ShortDescription *string         `json:"shortDescription,omitempty"`
	Thumbnail        *string         `json:"thumbnail,omitempty"`
	Tags             []string        `json:"tags,omitempty"`
}

type UpdatePostInput struct {
	Title            *string          `json:"title,omitempty"`
	Content          *json.RawMessage `json:"content,omitempty"`
	ShortDescription *string          `json:"shortDescription,omitempty"`
	Thumbnail        *string          `json:"thumbnail,omitempty"`
	Tags             []string         `json:"tags,omitempty"`
}

type CreateCommentInput struct {
	Text     string  `json:"text" binding:"required"`
	ParentID *string `json:"parentId,omitempty"`
}

type LikeAction struct {
	Action string `json:"action"` // "like" or "unlike"
}

type LikeResponse struct {
	Likes []string `json:"likes,omitempty"`
	Liked bool     `json:"liked,omitempty"`
}
