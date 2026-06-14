package arena

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mirabellier/mirabellier-backend-go/internal/auth"
)

func RegisterRoutes(r *gin.RouterGroup, db *sql.DB) {
	h := &handler{db: db}

	protected := r.Group("/arena")
	protected.Use(auth.Require())
	{
		protected.GET("/profile", h.getProfile)
		protected.GET("/collection", h.getCollection)
		protected.POST("/collection/select-card", h.selectCard)
		protected.POST("/fight", h.runFight)
		protected.POST("/draw-card", h.drawCard)
		protected.GET("/shop", h.getShop)
		protected.POST("/shop/buy", h.buyItem)
		protected.POST("/shop/use-consumable", h.useConsumable)
		protected.POST("/shop/craft", h.craftRecipe)
	}

	r.GET("/arena/leaderboard", h.getLeaderboard)
}

type handler struct {
	db *sql.DB
}

func (h *handler) getProfile(c *gin.Context) {
	user := auth.GetUser(c)
	profile, err := GetArenaProfilePayload(h.db, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load arena profile"})
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (h *handler) getCollection(c *gin.Context) {
	user := auth.GetUser(c)
	limitStr := c.DefaultQuery("limit", "200")
	limit, _ := strconv.Atoi(limitStr)
	if limit < 1 || limit > 500 {
		limit = 200
	}

	profile, err := GetArenaProfilePayload(h.db, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load profile"})
		return
	}

	cards, err := GetCollectionCards(h.db, user.ID, limit)
	if err != nil {
		cards = []ArenaCard{}
	}

	c.JSON(http.StatusOK, gin.H{
		"profile": profile,
		"cards":   cards,
		"limit":   limit,
	})
}

func (h *handler) selectCard(c *gin.Context) {
	user := auth.GetUser(c)

	var input struct {
		CardInstanceID string `json:"cardInstanceId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cardInstanceId is required"})
		return
	}

	card, profile, err := SelectCollectionCard(h.db, user.ID, input.CardInstanceID)
	if err != nil {
		if ae, ok := err.(*ArenaHTTPError); ok {
			c.JSON(ae.Status, gin.H{"error": ae.Message, "code": ae.Code})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"selectedCard": card,
		"profile":      profile,
	})
}

func (h *handler) runFight(c *gin.Context) {
	user := auth.GetUser(c)

	result, err := RunFight(h.db, user.ID)
	if err != nil {
		log.Printf("[arena] fight error for %s: %v", user.ID, err)
		if ae, ok := err.(*ArenaHTTPError); ok {
			c.JSON(ae.Status, gin.H{"error": ae.Message, "code": ae.Code})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *handler) drawCard(c *gin.Context) {
	user := auth.GetUser(c)

	card, profile, err := DrawDailyCard(h.db, user.ID)
	if err != nil {
		if ae, ok := err.(*ArenaHTTPError); ok {
			c.JSON(ae.Status, gin.H{"error": ae.Message, "code": ae.Code})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"card":    card,
		"profile": profile,
	})
}

func (h *handler) getShop(c *gin.Context) {
	user := auth.GetUser(c)

	shop, err := GetArenaShopPayload(h.db, user.ID)
	if err != nil {
		log.Printf("[arena] shop error for %s: %v", user.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load shop"})
		return
	}

	c.JSON(http.StatusOK, shop)
}

func (h *handler) buyItem(c *gin.Context) {
	user := auth.GetUser(c)

	var input struct {
		ItemID string `json:"itemId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "itemId is required"})
		return
	}

	result, err := BuyShopItem(h.db, user.ID, input.ItemID)
	if err != nil {
		log.Printf("[arena] buy error for %s, item %s: %v", user.ID, input.ItemID, err)
		if ae, ok := err.(*ArenaHTTPError); ok {
			c.JSON(ae.Status, gin.H{"error": ae.Message, "code": ae.Code})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *handler) useConsumable(c *gin.Context) {
	user := auth.GetUser(c)

	var input struct {
		ItemID string `json:"itemId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "itemId is required"})
		return
	}

	result, err := UseConsumable(h.db, user.ID, input.ItemID)
	if err != nil {
		log.Printf("[arena] useConsumable error for %s, item %s: %v", user.ID, input.ItemID, err)
		if ae, ok := err.(*ArenaHTTPError); ok {
			c.JSON(ae.Status, gin.H{"error": ae.Message, "code": ae.Code})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *handler) craftRecipe(c *gin.Context) {
	user := auth.GetUser(c)

	var input struct {
		RecipeID string `json:"recipeId" binding:"required"`
		Quantity int    `json:"quantity,omitempty"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "recipeId is required"})
		return
	}
	if input.Quantity < 1 {
		input.Quantity = 1
	}

	result, err := CraftShopRecipe(h.db, user.ID, input.RecipeID, input.Quantity)
	if err != nil {
		log.Printf("[arena] craft error for %s, recipe %s: %v", user.ID, input.RecipeID, err)
		if ae, ok := err.(*ArenaHTTPError); ok {
			c.JSON(ae.Status, gin.H{"error": ae.Message, "code": ae.Code})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *handler) getLeaderboard(c *gin.Context) {
	metric := c.DefaultQuery("metric", "level")
	limitStr := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitStr)

	entries, err := GetLeaderboard(h.db, metric, limit)
	if err != nil {
		log.Printf("[arena] leaderboard error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load leaderboard"})
		return
	}

	c.JSON(http.StatusOK, entries)
}
