package auth

// User matches the frontend's User type.
type User struct {
	ID        string           `json:"id" db:"id"`
	Username  string           `json:"username" db:"username"`
	DiscordID *string          `json:"discordId,omitempty" db:"discord_id"`
	Avatar    *string          `json:"avatar,omitempty" db:"avatar"`
	Banner    *string          `json:"banner,omitempty" db:"banner"`
	Bio       *string          `json:"bio,omitempty" db:"bio"`
	Location  *string          `json:"location,omitempty" db:"location"`
	Website   *string          `json:"website,omitempty" db:"website"`
	CreatedAt string           `json:"createdAt" db:"createdAt"`
	UpdatedAt *string          `json:"updatedAt,omitempty" db:"updatedAt"`

	Roles       []string         `json:"roles,omitempty"`
	Permissions *UserPermissions `json:"permissions,omitempty"`
}

type UserPermissions struct {
	AdminPanel               bool `json:"adminPanel"`
	ModerateGuestbook        bool `json:"moderateGuestbook"`
	ModerateQuestionOfTheDay bool `json:"moderateQuestionOfTheDay"`
	ManageShrines            bool `json:"manageShrines"`
}

type UserPublic struct {
	ID       string  `json:"id"`
	Username string  `json:"username"`
	Avatar   *string `json:"avatar,omitempty"`
	Banner   *string `json:"banner,omitempty"`
	Bio      *string `json:"bio,omitempty"`
	Location *string `json:"location,omitempty"`
	Website  *string `json:"website,omitempty"`
}

type UserStats struct {
	PostsCount    int          `json:"postsCount"`
	LikesCount    int          `json:"likesCount"`
	CommentsCount int          `json:"commentsCount"`
	RecentPosts   []PostSummary `json:"recentPosts"`
}

type PostSummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"createdAt"`
}

type Session struct {
	Token     string `json:"token" db:"token"`
	UserID    string `json:"userId" db:"user_id"`
	CreatedAt string `json:"createdAt" db:"created_at"`
}
