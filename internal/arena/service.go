package arena

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"time"
)

func GetArenaProfilePayload(db *sql.DB, userID string) (*PublicProfile, error) {
	profile, err := EnsureArenaProfile(db, userID)
	if err != nil {
		return nil, err
	}
	return buildPublicProfile(db, profile)
}

func DrawDailyCard(db *sql.DB, userID string) (*ArenaCard, *PublicProfile, error) {
	profile, err := EnsureArenaProfile(db, userID)
	if err != nil {
		return nil, nil, err
	}

	today := getCurrentRecordedDate()
	drawsUsedToday := 0
	if profile.LastCardDrawDate != nil && *profile.LastCardDrawDate == today {
		drawsUsedToday = profile.DailyCardDrawCount
	}

	if drawsUsedToday >= DAILY_CARD_DRAW_LIMIT {
		return nil, nil, &ArenaHTTPError{
			Status: 409, Code: "ARENA_DAILY_DRAW_LIMIT",
			Message: fmt.Sprintf("You can only draw %d cards per day.", DAILY_CARD_DRAW_LIMIT),
			Details: map[string]any{"nextDrawAt": getNextCardDrawAt(today)},
		}
	}

	malCard, err := DrawArenaCard(db)
	if err != nil {
		return nil, nil, &ArenaHTTPError{Status: 503, Code: "MAL_POOL_EMPTY", Message: "Arena card pool is currently unavailable. Please try again in a moment."}
	}

	rarity := RarityFromFavorites(malCard.Favorites)
	card := createDrawnCard(malCard, rarity)

	// Node.js: always sets the new card as selected and inserts into collection
	nextDrawCount := 1
	if profile.LastCardDrawDate != nil && *profile.LastCardDrawDate == today {
		nextDrawCount = drawsUsedToday + 1
	}
	profile.SelectedCard = card
	profile.LastCardDrawDate = &today
	profile.DailyCardDrawCount = nextDrawCount

	saveProfileState(db, profile)
	insertCollectionCard(db, userID, card)

	pubProfile, _ := buildPublicProfile(db, profile)
	return card, pubProfile, nil
}

func getNextCardDrawAt(lastDrawDate string) string {
	t, _ := time.Parse("2006-01-02", lastDrawDate)
	next := t.Add(24 * time.Hour)
	return next.Format("2006-01-02") + "T00:00:00.000Z"
}

func RunFight(db *sql.DB, userID string) (map[string]any, error) {
	profile, err := EnsureArenaProfile(db, userID)
	if err != nil {
		return nil, err
	}

	if profile.SelectedCard == nil {
		return nil, &ArenaHTTPError{Status: 409, Code: "ARENA_CARD_REQUIRED", Message: "Draw a card to start."}
	}

	// Cooldown check
	if profile.LastFightAt != nil && *profile.LastFightAt != "" {
		t, err := time.Parse(time.RFC3339, *profile.LastFightAt)
		if err == nil {
			elapsed := time.Since(t)
			if elapsed < time.Duration(FIGHT_COOLDOWN_MS)*time.Millisecond {
				return nil, &ArenaHTTPError{
					Status: 429, Code: "ARENA_FIGHT_COOLDOWN",
					Message: "Fight cooldown active. Please wait a few seconds before fighting again.",
					Details: map[string]any{"retryAfterMs": FIGHT_COOLDOWN_MS - int(elapsed.Milliseconds())},
				}
			}
		}
	}

	// Select opponent
	opponentProfile, err := selectOpponentForFight(db, userID)
	if err != nil {
		return nil, &ArenaHTTPError{Status: 503, Code: "MAL_POOL_EMPTY", Message: "Arena card pool is currently unavailable. Please try again in a moment."}
	}

	// Simulate fight
	result := simulateFightSimple(profile, opponentProfile)

	now := time.Now().UTC().Format(time.RFC3339)
	playerWon := result.playerWon

	var xpDelta, coinDelta int
	var materialDrops []string
	materialDropCount := 1
	if playerWon {
		materialDropCount = 2
	}

	if playerWon {
		rarityCoinReward := RarityConfigMap[result.playerRarity].CoinReward
		xpDelta = CalculateWinXP(opponentProfile.Level, result.roundsWon, profile.WinStreak)
		coinDelta = CalculateWinCoins(opponentProfile.Level, rarityCoinReward, profile.Luck)

		// Consume win boosts
		if profile.Effects != nil {
			if profile.Effects.ExpBoostWinsRemaining > 0 && profile.Effects.ExpBoostPct > 0 {
				xpDelta = int(float64(xpDelta) * (1 + float64(profile.Effects.ExpBoostPct)/100))
				profile.Effects.ExpBoostWinsRemaining--
				if profile.Effects.ExpBoostWinsRemaining == 0 {
					profile.Effects.ExpBoostPct = 0
				}
			}
			if profile.Effects.CoinBoostWinsRemaining > 0 && profile.Effects.CoinBoostPct > 0 {
				coinDelta = int(float64(coinDelta) * (1 + float64(profile.Effects.CoinBoostPct)/100))
				profile.Effects.CoinBoostWinsRemaining--
				if profile.Effects.CoinBoostWinsRemaining == 0 {
					profile.Effects.CoinBoostPct = 0
				}
			}
		}

		profile.XP += xpDelta
		profile.Coins += coinDelta
		profile.Wins++
		profile.WinStreak++
		profile.LifetimeCoinsEarned += coinDelta
	} else {
		profile.Losses++
		if profile.Effects != nil && profile.Effects.StreakShieldCharges > 0 {
			profile.Effects.StreakShieldCharges--
		} else {
			profile.WinStreak = 0
		}
	}

	// Material drops
	dropTier := selectMaterialDropTier(profile.Level, opponentProfile.Level)
	mats := MATERIAL_IDS_BY_TIER[dropTier]
	for i := 0; i < materialDropCount; i++ {
		if len(mats) > 0 {
			itemID := mats[randomInt(0, len(mats)-1)]
			addItemToInventory(db, userID, itemID, 1)
			materialDrops = append(materialDrops, itemID)
		}
	}

	applyLevelUps(profile)
	profile.LastFightAt = &now
	saveProfileState(db, profile)

	// Save fight record
	fightID := makeArenaID("fight")
	var opponentID *string
	if !opponentProfile.IsNPC {
		opponentID = &opponentProfile.UserID
	}
	fightResult := "loss"
	if playerWon {
		fightResult = "win"
	}
	roundsJSON, _ := json.Marshal(result.rounds)
	db.Exec(`INSERT INTO arena_fights (id, userId, opponentUserId, result, roundsJson, xpDelta, coinDelta, createdAt)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		fightID, userID, opponentID, fightResult, string(roundsJSON), xpDelta, coinDelta, now)

	pubProfile, _ := buildPublicProfile(db, profile)

	// Build fight response matching Node.js top-level shape
	resp := map[string]any{
		"result": fightResult,
		"opponent": map[string]any{
			"userId":      opponentProfile.UserID,
			"displayName": opponentProfile.DisplayName,
			"isNpc":       opponentProfile.IsNPC,
			"level":       opponentProfile.Level,
			"stats": map[string]int{
				"hp": opponentProfile.HP, "power": opponentProfile.Power,
				"guard": opponentProfile.Guard, "speed": opponentProfile.Speed, "luck": opponentProfile.Luck,
			},
			"selectedCard": opponentProfile.SelectedCard,
		},
		"battle": map[string]any{
			"rounds":  result.rounds,
			"maxHp":   map[string]int{"player": result.playerMaxHP, "opponent": result.opponentMaxHP},
			"finalHp": map[string]int{"player": result.playerFinalHP, "opponent": result.opponentFinalHP},
			"console": result.console,
		},
		"rounds": result.rounds,
		"score":  map[string]int{"player": 1, "opponent": 0},
		"rewards": map[string]any{
			"xp":             xpDelta,
			"coins":          coinDelta,
			"levelsGained":   0,
			"materialDrops":  materialDrops,
		},
		"profile": pubProfile,
	}
	if !playerWon {
		resp["score"] = map[string]int{"player": 0, "opponent": 1}
	}

	return resp, nil
}

type opponentInfo struct {
	UserID      string
	Level       int
	HP          int
	Power       int
	Guard       int
	Speed       int
	Luck        int
	SelectedCard *ArenaCard
	IsNPC       bool
	DisplayName string
}

func selectOpponentForFight(db *sql.DB, userID string) (*opponentInfo, error) {
	// Try random real player
	var oppUserID string
	var level, hp, power, guard, speed, luck int
	var cardJSON sql.NullString
	err := db.QueryRow(`
		SELECT p.userId, p.level, p.hp, p.power, p.guard, p.speed, p.luck, p.selectedCardJson
		FROM arena_profiles p
		JOIN users u ON u.id = p.userId
		WHERE p.userId <> ? AND p.selectedCardJson IS NOT NULL
		ORDER BY RANDOM() LIMIT 1
	`, userID).Scan(&oppUserID, &level, &hp, &power, &guard, &speed, &luck, &cardJSON)
	if err == nil && cardJSON.Valid && cardJSON.String != "" {
		var card ArenaCard
		json.Unmarshal([]byte(cardJSON.String), &card)

		var username string
		db.QueryRow("SELECT username FROM users WHERE id = ?", oppUserID).Scan(&username)

		return &opponentInfo{
			UserID: oppUserID, Level: level, HP: hp, Power: power, Guard: guard,
			Speed: speed, Luck: luck, SelectedCard: &card,
			IsNPC: false, DisplayName: username,
		}, nil
	}

	// Fallback: NPC Training Slime
	return &opponentInfo{
		UserID:      "npc:training-slime",
		Level:       1,
		HP:          95,
		Power:       10,
		Guard:       10,
		Speed:       8,
		Luck:        5,
		SelectedCard: &ArenaCard{Title: "Training Slime", Rarity: "C"},
		IsNPC:       true,
		DisplayName: "Training Slime",
	}, nil
}

type fightResult struct {
	playerWon      bool
	roundsWon      int
	rounds         []map[string]any
	playerMaxHP    int
	opponentMaxHP  int
	playerFinalHP  int
	opponentFinalHP int
	playerRarity   string
	opponentRarity string
	console        []map[string]any
}

func simulateFightSimple(player *ArenaProfile, opponent *opponentInfo) *fightResult {
	playerStats := map[string]int{
		"hp": player.HP, "power": player.Power, "guard": player.Guard,
		"speed": player.Speed, "luck": player.Luck,
	}
	oppStats := map[string]int{
		"hp": opponent.HP, "power": opponent.Power, "guard": opponent.Guard,
		"speed": opponent.Speed, "luck": opponent.Luck,
	}

	playerMaxHP := computeMaxHP(playerStats)
	opponentMaxHP := computeMaxHP(oppStats)
	pHP := playerMaxHP
	oHP := opponentMaxHP

	playerRarity := "C"
	if player.SelectedCard != nil {
		playerRarity = player.SelectedCard.Rarity
	}
	oppRarity := "C"
	if opponent.SelectedCard != nil {
		oppRarity = opponent.SelectedCard.Rarity
	}

	var rounds []map[string]any
	var console []map[string]any
	playerWins := 0
	oppWins := 0

	pushConsole := func(msg string) {
		console = append(console, map[string]any{
			"line": msg, "playerHp": pHP, "opponentHp": oHP,
		})
	}

	maxTurns := 60
	for turn := 1; turn <= maxTurns; turn++ {
		if pHP <= 0 || oHP <= 0 {
			break
		}

		// Determine who attacks first based on speed
		pSpeed := playerStats["speed"] + randomInt(0, 8)
		oSpeed := oppStats["speed"] + randomInt(0, 8)

		if pSpeed >= oSpeed {
			// Player attacks first
			pResult := attack("player", playerStats, oppStats, playerRarity, 0)
			handleAttack(pResult, playerStats, oppStats, &pHP, &oHP, turn, "player", &playerWins, &oppWins, &rounds, pushConsole)

			if oHP <= 0 {
				break
			}

			oResult := attack("opponent", oppStats, playerStats, oppRarity, 0)
			handleAttack(oResult, oppStats, playerStats, &oHP, &pHP, turn, "opponent", &oppWins, &playerWins, &rounds, pushConsole)
		} else {
			// Opponent attacks first
			oResult := attack("opponent", oppStats, playerStats, oppRarity, 0)
			handleAttack(oResult, oppStats, playerStats, &oHP, &pHP, turn, "opponent", &oppWins, &playerWins, &rounds, pushConsole)

			if pHP <= 0 {
				break
			}

			pResult := attack("player", playerStats, oppStats, playerRarity, 0)
			handleAttack(pResult, playerStats, oppStats, &pHP, &oHP, turn, "player", &playerWins, &oppWins, &rounds, pushConsole)
		}
	}

	playerWon := false
	if oHP <= 0 && pHP <= 0 {
		playerWon = playerStats["power"] > oppStats["power"] || (playerStats["power"] == oppStats["power"] && playerStats["speed"] >= oppStats["speed"])
	} else if oHP <= 0 {
		playerWon = true
	}

	if playerWon {
		pHP = max(1, pHP)
		oHP = 0
	} else {
		oHP = max(1, oHP)
		pHP = 0
	}

	roundsWon := max(1, min(3, 4-(len(rounds)/6)))

	return &fightResult{
		playerWon:      playerWon,
		roundsWon:      roundsWon,
		rounds:         rounds,
		playerMaxHP:    playerMaxHP,
		opponentMaxHP:  opponentMaxHP,
		playerFinalHP:  pHP,
		opponentFinalHP: oHP,
		playerRarity:   playerRarity,
		opponentRarity: oppRarity,
		console:        console,
	}
}

func attack(side string, attacker, defender map[string]int, rarity string, turn int) *AttackResult {
	return CalculateAttackOutcome(attacker, defender, rarity, 0, 0)
}

func handleAttack(r *AttackResult, attacker, defender map[string]int, attHP, defHP *int, turn int, side string, attWins, defWins *int, rounds *[]map[string]any, pushConsole func(string)) {
	displayName := side

	if r.Avoided {
		pushConsole(displayName + " avoided the attack")
		*rounds = append(*rounds, map[string]any{
			"turn": turn, "attacker": side, "attackerName": displayName,
			"avoided": true, "critical": false, "damage": 0,
			"playerHp": *attHP, "opponentHp": *defHP,
		})
		return
	}

	damage := r.Damage
	if damage > *defHP {
		damage = *defHP
	}
	*defHP -= damage

	if damage > 0 {
		pushConsole(displayName + " dealt " + itoa(damage) + " damage")
	}
	if r.Critical {
		pushConsole(displayName + " landed a critical hit")
	}

	*rounds = append(*rounds, map[string]any{
		"turn": turn, "attacker": side, "attackerName": displayName,
		"avoided": false, "critical": r.Critical, "damage": damage,
		"playerHp": *attHP, "opponentHp": *defHP,
	})

	// Round winner by damage
	if side == "player" {
		(*attWins)++
	} else {
		(*defWins)++
	}
}

func itoa(i int) string { return fmt.Sprintf("%d", i) }
func min(a, b int) int { if a < b { return a }; return b }

// --- Shop, collection, leaderboard, helpers ---

func GetArenaShopPayload(db *sql.DB, userID string) (map[string]any, error) {
	profile, err := EnsureArenaProfile(db, userID)
	if err != nil {
		return nil, err
	}
	inventory := getInventoryMap(db, userID)
	equippedRows, _ := db.Query("SELECT slot, itemId FROM arena_equipment WHERE userId = ?", userID)
	equippedBySlot := map[string]string{}
	if equippedRows != nil {
		defer equippedRows.Close()
		for equippedRows.Next() {
			var slot, itemID string
			equippedRows.Scan(&slot, &itemID)
			equippedBySlot[slot] = itemID
		}
	}

	// Build shop items grouped by tier (matching Node.js buildShopCatalog)
	type catalogItem struct {
		ShopItem
		OwnedQuantity  int     `json:"ownedQuantity"`
		IsOwned        bool    `json:"isOwned"`
		IsEquipped     bool    `json:"isEquipped"`
		Unlocked       bool    `json:"unlocked"`
		CanBuy         bool    `json:"canBuy"`
		CanCraft       bool    `json:"canCraft"`
		CooldownEndsAt *string `json:"cooldownEndsAt"`
	}

	tierMap := make(map[string][]catalogItem)
	for _, item := range ShopItems {
		ownedQty := 0
		if qty, ok := inventory[item.ID]; ok {
			ownedQty = qty
		}
		isOwned := ownedQty > 0
		unlocked := profile.Level >= item.UnlockLevel
		isEquipped := item.Type == "gear" && item.Slot != "" && equippedBySlot[item.Slot] == item.ID

		var recipe *ShopRecipe
		if item.RecipeID != "" {
			if r, ok := ShopRecipesByID[item.RecipeID]; ok {
				recipe = &r
			}
		}

		canBuy := item.Acquisition == "buy" && unlocked && profile.Coins >= item.Price

		canCraft := false
		if item.Acquisition == "craft" && recipe != nil && unlocked {
			hasCoins := profile.Coins >= recipe.CoinCost
			hasInputs := true
			for _, inp := range recipe.Inputs {
				if (inventory[inp.ItemID]) < inp.Quantity {
					hasInputs = false
					break
				}
			}
			gearOwned := item.Type == "gear" && ownedQty > 0
			canCraft = hasCoins && hasInputs && !gearOwned
		}

		var cooldownEndsAt *string
		if item.Type == "consumable" && item.ConsumableEffect != nil && item.ConsumableEffect.Kind == "ascension" &&
			profile.Effects != nil && profile.Effects.AscensionLastPurchasedAt != nil {
			t, err := time.Parse(time.RFC3339, *profile.Effects.AscensionLastPurchasedAt)
			if err == nil {
				cooldownMs := int64(item.ConsumableEffect.CooldownDays)
				if cooldownMs <= 0 {
					cooldownMs = 7
				}
				cooldownMs *= 24 * 60 * 60 * 1000
				cooldownEnds := t.UnixMilli() + cooldownMs
				if cooldownEnds > time.Now().UnixMilli() {
					ends := time.UnixMilli(cooldownEnds).UTC().Format(time.RFC3339)
					cooldownEndsAt = &ends
				}
			}
		}
		if cooldownEndsAt != nil {
			canBuy = false
			canCraft = false
		}

		ci := catalogItem{
			ShopItem:       item,
			OwnedQuantity:  ownedQty,
			IsOwned:        isOwned,
			IsEquipped:     isEquipped,
			Unlocked:       unlocked,
			CanBuy:         canBuy,
			CanCraft:       canCraft,
			CooldownEndsAt: cooldownEndsAt,
		}
		tierMap[item.Tier] = append(tierMap[item.Tier], ci)
	}

	// Group by tier order
	type tierGroup struct {
		Tier  string        `json:"tier"`
		Items []catalogItem `json:"items"`
	}
	var shop []tierGroup
	for _, tier := range ShopTiers {
		shop = append(shop, tierGroup{Tier: tier, Items: tierMap[tier]})
	}

	// Build recipes (unchanged from before)
	type recipeEntry struct {
		ShopRecipe
		Output   map[string]any   `json:"output"`
		Inputs   []map[string]any `json:"inputs"`
		Unlocked bool             `json:"unlocked"`
		CanCraft bool             `json:"canCraft"`
	}
	var recipes []recipeEntry
	for _, r := range ShopRecipes {
		outputItem := ShopItemsByID[r.Output.ItemID]
		unlocked := profile.Level >= r.UnlockLevel
		hasCoins := profile.Coins >= r.CoinCost
		var inputs []map[string]any
		hasInputs := true
		for _, inp := range r.Inputs {
			owned := 0
			if qty, ok := inventory[inp.ItemID]; ok {
				owned = qty
			}
			inpItem := ShopItemsByID[inp.ItemID]
			inputs = append(inputs, map[string]any{
				"itemId": inp.ItemID, "itemName": inpItem.Name, "required": inp.Quantity, "owned": owned,
			})
			if owned < inp.Quantity {
				hasInputs = false
			}
		}
		outputOwnedQty := 0
		if qty, ok := inventory[r.Output.ItemID]; ok {
			outputOwnedQty = qty
		}
		blockedByGear := outputItem.Type == "gear" && outputOwnedQty > 0

		recipes = append(recipes, recipeEntry{
			ShopRecipe: r,
			Output:     map[string]any{"itemId": r.Output.ItemID, "itemName": outputItem.Name, "quantity": r.Output.Quantity},
			Inputs:     inputs,
			Unlocked:   unlocked,
			CanCraft:   unlocked && hasCoins && hasInputs && !blockedByGear,
		})
	}

	// Equipped
	equipped := map[string]string{}
	for slot, itemID := range equippedBySlot {
		equipped[slot] = itemID
	}

	fullProfile, _ := GetArenaProfilePayload(db, userID)

	return map[string]any{
		"catalogVersion": CATALOG_VERSION,
		"profile":        fullProfile,
		"shop":           shop,
		"recipes":        recipes,
		"equipped":       equipped,
	}, nil
}

func BuyShopItem(db *sql.DB, userID, itemID string) (map[string]any, error) {
	item, ok := ShopItemsByID[itemID]
	if !ok {
		return nil, &ArenaHTTPError{Status: 404, Code: "ARENA_ITEM_NOT_FOUND", Message: "Item not found."}
	}
	profile, err := EnsureArenaProfile(db, userID)
	if err != nil {
		return nil, err
	}
	if profile.Level < item.UnlockLevel {
		return nil, &ArenaHTTPError{Status: 403, Code: "ARENA_ITEM_LOCKED", Message: "Level too low for this item."}
	}
	if item.Acquisition != "buy" {
		return nil, &ArenaHTTPError{Status: 400, Code: "ARENA_ITEM_CRAFT_ONLY", Message: "This item is crafted, not bought directly."}
	}
	if profile.Coins < item.Price {
		return nil, &ArenaHTTPError{Status: 400, Code: "ARENA_NOT_ENOUGH_COINS", Message: "Not enough coins."}
	}
	inventory := getInventoryMap(db, userID)
	if item.Type == "gear" && inventory[item.ID] > 0 {
		return nil, &ArenaHTTPError{Status: 409, Code: "ARENA_GEAR_ALREADY_OWNED", Message: "Gear is already owned. Equip your current copy instead."}
	}
	profile.Coins -= item.Price
	addItemToInventory(db, userID, item.ID, 1)
	if item.Type == "gear" {
		db.Exec("INSERT OR REPLACE INTO arena_equipment (userId, slot, itemId, equippedAt) VALUES (?, ?, ?, datetime('now'))", userID, item.Slot, item.ID)
	}
	db.Exec(`UPDATE arena_profiles SET coins=?, hp=?, power=?, guard=?, speed=?, luck=?, updatedAt=datetime('now') WHERE userId=?`,
		profile.Coins, profile.HP, profile.Power, profile.Guard, profile.Speed, profile.Luck, userID)
	shop, _ := GetArenaShopPayload(db, userID)
	return map[string]any{"purchasedItemId": itemID, "appliedInstantly": false, "shop": shop}, nil
}

func CraftShopRecipe(db *sql.DB, userID, recipeID string, quantity int) (map[string]any, error) {
	recipe, ok := ShopRecipesByID[recipeID]
	if !ok {
		return nil, &ArenaHTTPError{Status: 404, Code: "ARENA_RECIPE_NOT_FOUND", Message: "Recipe not found."}
	}
	if quantity < 1 {
		quantity = 1
	}
	if quantity > 20 {
		quantity = 20
	}
	outputItem := ShopItemsByID[recipe.Output.ItemID]
	craftCount := quantity
	if outputItem.Type == "gear" {
		craftCount = 1
	}
	profile, err := EnsureArenaProfile(db, userID)
	if err != nil {
		return nil, err
	}
	if profile.Level < recipe.UnlockLevel {
		return nil, &ArenaHTTPError{Status: 403, Code: "ARENA_RECIPE_LOCKED", Message: "Level too low for this recipe."}
	}
	totalCoinCost := recipe.CoinCost * craftCount
	if profile.Coins < totalCoinCost {
		return nil, &ArenaHTTPError{Status: 400, Code: "ARENA_NOT_ENOUGH_COINS", Message: "Not enough coins."}
	}
	inventory := getInventoryMap(db, userID)
	for _, input := range recipe.Inputs {
		if inventory[input.ItemID] < input.Quantity*craftCount {
			return nil, &ArenaHTTPError{Status: 400, Code: "ARENA_RECIPE_MATERIALS_MISSING", Message: "Missing crafting materials."}
		}
	}
	if outputItem.Type == "gear" && inventory[outputItem.ID] > 0 {
		return nil, &ArenaHTTPError{Status: 409, Code: "ARENA_GEAR_ALREADY_OWNED", Message: "Gear already owned."}
	}
	for _, input := range recipe.Inputs {
		deductItemFromInventory(db, userID, input.ItemID, input.Quantity*craftCount)
	}
	addItemToInventory(db, userID, recipe.Output.ItemID, craftCount)
	if outputItem.Type == "gear" && outputItem.Slot != "" {
		db.Exec("INSERT OR REPLACE INTO arena_equipment (userId, slot, itemId, equippedAt) VALUES (?, ?, ?, datetime('now'))", userID, outputItem.Slot, outputItem.ID)
	}
	profile.Coins -= totalCoinCost
	db.Exec("UPDATE arena_profiles SET coins=?, updatedAt=datetime('now') WHERE userId=?", profile.Coins, userID)
	shop, _ := GetArenaShopPayload(db, userID)
	return map[string]any{"recipe": recipe, "outputItem": outputItem, "quantityCrafted": craftCount, "shop": shop}, nil
}

func UseConsumable(db *sql.DB, userID, itemID string) (map[string]any, error) {
	profile, err := EnsureArenaProfile(db, userID)
	if err != nil {
		return nil, err
	}
	item, ok := ShopItemsByID[itemID]
	if !ok || item.Type != "consumable" {
		return nil, &ArenaHTTPError{Status: 400, Code: "ARENA_SHOP_INVALID", Message: "Not a consumable"}
	}
	inventory := getInventoryMap(db, userID)
	if inventory[itemID] <= 0 {
		return nil, &ArenaHTTPError{Status: 402, Code: "ARENA_MISSING_ITEM", Message: "Item not owned"}
	}
	deductItemFromInventory(db, userID, itemID, 1)

	if item.ConsumableEffect == nil {
		return map[string]any{"ok": true}, nil
	}
	if profile.Effects == nil {
		profile.Effects = defaultEffects()
	}
	effect := item.ConsumableEffect
	switch effect.Kind {
	case "exp_boost":
		profile.Effects.ExpBoostPct = max(profile.Effects.ExpBoostPct, effect.Pct)
		profile.Effects.ExpBoostWinsRemaining += effect.Wins
	case "coin_boost":
		profile.Effects.CoinBoostPct = max(profile.Effects.CoinBoostPct, effect.Pct)
		profile.Effects.CoinBoostWinsRemaining += effect.Wins
	case "shield_fight_start":
		profile.Effects.FightStartShieldAmount = max(profile.Effects.FightStartShieldAmount, effect.Amount)
		profile.Effects.FightStartShieldCharges += effect.Charges
	case "evade_next_fight":
		profile.Effects.EvadeBoostPct = max(profile.Effects.EvadeBoostPct, effect.Pct)
		profile.Effects.EvadeBoostFightsRemaining += effect.Fights
	case "first_hit_true_damage":
		profile.Effects.FirstHitTrueDamageValue = max(profile.Effects.FirstHitTrueDamageValue, effect.Value)
		profile.Effects.FirstHitTrueDamageCharges += effect.Charges
	case "bonus_vs_higher_rarity":
		profile.Effects.HigherRarityDamageBonusPct = max(profile.Effects.HigherRarityDamageBonusPct, effect.Pct)
		profile.Effects.HigherRarityDamageBonusPctCharges += effect.Charges
	case "reroll_keep_higher":
		profile.Effects.RerollKeepHigherCharges += effect.Charges
	case "streak_shield":
		profile.Effects.StreakShieldCharges += effect.Charges
	case "upgrade_lowest_rarity":
		profile.Effects.UpgradeLowestRarityCharges += effect.Charges
	case "guarantee_ssr_plus":
		profile.Effects.GuaranteeSsrPlusCharges += effect.Charges
	case "cooldown_bypass":
		profile.Effects.GateKeyCharges += effect.Charges
	case "ascension":
		profile.HP += 1
		profile.Power += 1
		profile.Guard += 1
		profile.Speed += 1
		profile.Luck += 1
		now := time.Now().UTC().Format(time.RFC3339)
		profile.Effects.AscensionLastPurchasedAt = &now
	case "double_passive_trigger":
		profile.Effects.DoublePassiveTriggerFightsRemaining += effect.Fights
	case "restore_consumable_charge":
	default:
		return nil, &ArenaHTTPError{Status: 400, Message: "Unsupported consumable effect."}
	}
	db.Exec(`UPDATE arena_profiles SET hp=?, power=?, guard=?, speed=?, luck=?, effectsJson=?, updatedAt=datetime('now') WHERE userId=?`,
		profile.HP, profile.Power, profile.Guard, profile.Speed, profile.Luck, mustMarshal(profile.Effects), userID)
	shop, _ := GetArenaShopPayload(db, userID)
	return map[string]any{"activatedItemId": itemID, "effects": profile.Effects, "shop": shop}, nil
}

func mustMarshal(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func GetLeaderboard(db *sql.DB, metric string, limit int) (map[string]any, error) {
	if limit < 1 || limit > 50 {
		limit = 20
	}
	normalizedMetric := "level"
	switch metric {
	case "win_rate":
		normalizedMetric = "win_rate"
	case "rich":
		normalizedMetric = "rich"
	}

	var orderBy string
	switch normalizedMetric {
	case "level":
		orderBy = "p.level DESC, CAST(p.xp AS REAL) / CAST((80 + 40 * p.level * p.level) AS REAL) DESC, p.wins DESC, p.updatedAt ASC"
	case "win_rate":
		orderBy = "CAST(p.wins AS REAL) / NULLIF(CAST((p.wins + p.losses) AS REAL), 0) DESC, p.level DESC, p.updatedAt ASC"
	case "rich":
		orderBy = "p.coins DESC, p.lifetimeCoinsEarned DESC, p.level DESC, p.updatedAt ASC"
	}

	rows, err := db.Query(`SELECT p.userId, u.username, u.avatar, p.level, p.xp, p.coins,
		p.wins, p.losses, p.winStreak, p.lifetimeCoinsEarned, p.updatedAt,
		(p.wins + p.losses) AS totalFights,
		CASE WHEN (p.wins + p.losses) > 0 THEN CAST(p.wins AS REAL) / CAST((p.wins + p.losses) AS REAL) ELSE 0 END AS winRate,
		CASE WHEN (80 + 40 * p.level * p.level) > 0 THEN CAST(p.xp AS REAL) / CAST((80 + 40 * p.level * p.level) AS REAL) ELSE 0 END AS xpProgress
		FROM arena_profiles p JOIN users u ON u.id = p.userId ORDER BY `+orderBy+` LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []map[string]any
	rank := 0
	for rows.Next() {
		var userID, username string
		var avatar sql.NullString
		var level, xp, coins, wins, losses, winStreak int
		var lifetimeCoinsEarned int
		var updatedAt sql.NullString
		var totalFights int
		var winRate, xpProgress float64
		rows.Scan(&userID, &username, &avatar, &level, &xp, &coins, &wins, &losses, &winStreak,
			&lifetimeCoinsEarned, &updatedAt, &totalFights, &winRate, &xpProgress)
		rank++

		userObj := map[string]any{
			"id":       userID,
			"username": username,
		}
		if avatar.Valid {
			userObj["avatar"] = avatar.String
		} else {
			userObj["avatar"] = nil
		}

		entry := map[string]any{
			"rank":                rank,
			"user":                userObj,
			"level":               level,
			"xp":                  xp,
			"xpProgress":          math.Round(xpProgress*1000) / 1000,
			"xpToNext":            XPToNext(level),
			"coins":               coins,
			"wins":                wins,
			"losses":              losses,
			"totalFights":         totalFights,
			"winRate":             math.Round(winRate*1000) / 1000,
			"winStreak":           winStreak,
			"lifetimeCoinsEarned": lifetimeCoinsEarned,
		}
		if updatedAt.Valid {
			entry["updatedAt"] = updatedAt.String
		}
		entries = append(entries, entry)
	}
	if entries == nil {
		entries = []map[string]any{}
	}

	return map[string]any{"metric": normalizedMetric, "limit": limit, "entries": entries}, nil
}

func GetCollectionCards(db *sql.DB, userID string, limit int) ([]ArenaCard, error) {
	if limit < 1 || limit > 500 {
		limit = 200
	}
	rows, err := db.Query("SELECT cardJson FROM arena_card_collection WHERE userId = ? ORDER BY createdAt DESC LIMIT ?", userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cards []ArenaCard
	for rows.Next() {
		var cardJSON string
		rows.Scan(&cardJSON)
		var card ArenaCard
		if json.Unmarshal([]byte(cardJSON), &card) == nil {
			cards = append(cards, card)
		}
	}
	if cards == nil {
		cards = []ArenaCard{}
	}
	return cards, nil
}

func SelectCollectionCard(db *sql.DB, userID, cardInstanceID string) (*ArenaCard, *PublicProfile, error) {
	profile, err := EnsureArenaProfile(db, userID)
	if err != nil {
		return nil, nil, err
	}
	var cardJSON string
	err = db.QueryRow("SELECT cardJson FROM arena_card_collection WHERE userId = ? AND cardInstanceId = ?", userID, cardInstanceID).Scan(&cardJSON)
	if err != nil {
		return nil, nil, &ArenaHTTPError{Status: 404, Code: "ARENA_COLLECTION_CARD_NOT_FOUND", Message: "Card not found in your collection."}
	}
	var card ArenaCard
	if json.Unmarshal([]byte(cardJSON), &card) != nil {
		return nil, nil, &ArenaHTTPError{Status: 409, Code: "ARENA_COLLECTION_CARD_INVALID", Message: "Stored card data is invalid."}
	}
	profile.SelectedCard = &card
	saveProfileState(db, profile)
	pubProfile, _ := buildPublicProfile(db, profile)
	return &card, pubProfile, nil
}

func buildPublicProfile(db *sql.DB, profile *ArenaProfile) (*PublicProfile, error) {
	totalFights := profile.Wins + profile.Losses
	nextXP := XPToNext(profile.Level)
	xpProgress := 0.0
	if nextXP > 0 {
		xpProgress = float64(profile.XP) / float64(nextXP)
	}
	winRate := 0.0
	if totalFights > 0 {
		winRate = float64(profile.Wins) / float64(totalFights)
	}
	dailyDrawsUsed := profile.DailyCardDrawCount
	if profile.LastCardDrawDate != nil && *profile.LastCardDrawDate != getCurrentRecordedDate() {
		dailyDrawsUsed = 0
	}
	canDraw := dailyDrawsUsed < DAILY_CARD_DRAW_LIMIT
	inventory := getInventoryMap(db, profile.UserID)
	materialInv := map[string]int{}
	for itemID, qty := range inventory {
		if item, ok := ShopItemsByID[itemID]; ok && item.Type == "material" {
			materialInv[itemID] = qty
		}
	}
	rows, err := db.Query("SELECT slot, itemId, equippedAt FROM arena_equipment WHERE userId = ?", profile.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	equipment := map[string]any{}
	equipStats := map[string]int{"hp": 0, "power": 0, "guard": 0, "speed": 0, "luck": 0}
	for rows.Next() {
		var slot, itemID, equippedAt string
		rows.Scan(&slot, &itemID, &equippedAt)
		if item, ok := ShopItemsByID[itemID]; ok && item.Type == "gear" {
			equipment[slot] = map[string]any{
				"itemId": item.ID, "name": item.Name, "slot": item.Slot,
				"tier": item.Tier, "stats": item.Stats, "equippedAt": equippedAt,
			}
			for k, v := range item.Stats {
				equipStats[k] += v
			}
		}
	}
	return &PublicProfile{
		UserID: profile.UserID, Level: profile.Level, XP: profile.XP,
		XPToNext: nextXP, XPProgress: math.Round(xpProgress*100) / 100,
		Coins: profile.Coins, Wins: profile.Wins, Losses: profile.Losses,
		TotalFights: totalFights, WinRate: math.Round(winRate*100) / 100,
		WinStreak: profile.WinStreak,
		Stats: &PublicStats{
			Base:      map[string]int{"hp": profile.HP, "power": profile.Power, "guard": profile.Guard, "speed": profile.Speed, "luck": profile.Luck},
			Equipment: equipStats,
			Total:     map[string]int{"hp": profile.HP + equipStats["hp"], "power": profile.Power + equipStats["power"], "guard": profile.Guard + equipStats["guard"], "speed": profile.Speed + equipStats["speed"], "luck": profile.Luck + equipStats["luck"]},
		},
		SelectedCard: profile.SelectedCard, CanDrawCard: canDraw,
		DailyDrawLimit: DAILY_CARD_DRAW_LIMIT, DailyDrawsUsed: dailyDrawsUsed,
		DailyDrawsRemaining: DAILY_CARD_DRAW_LIMIT - dailyDrawsUsed,
		LifetimeCoinsEarned: profile.LifetimeCoinsEarned,
		Effects: profile.Effects, Equipment: equipment, ActivePassives: []ActivePassive{},
		MaterialInventory: materialInv, LastFightAt: profile.LastFightAt,
		CreatedAt: profile.CreatedAt, UpdatedAt: profile.UpdatedAt,
	}, nil
}

func saveProfileState(db *sql.DB, profile *ArenaProfile) {
	cardJSON, _ := json.Marshal(profile.SelectedCard)
	effectsJSON, _ := json.Marshal(profile.Effects)
	now := time.Now().UTC().Format(time.RFC3339)
	db.Exec(`UPDATE arena_profiles SET
		level=?, xp=?, coins=?, wins=?, losses=?, winStreak=?,
		hp=?, power=?, guard=?, speed=?, luck=?, lifetimeCoinsEarned=?,
		selectedCardJson=?, lastCardDrawDate=?, dailyCardDrawCount=?,
		effectsJson=?, lastFightAt=?, updatedAt=?
		WHERE userId=?`,
		profile.Level, profile.XP, profile.Coins, profile.Wins, profile.Losses, profile.WinStreak,
		profile.HP, profile.Power, profile.Guard, profile.Speed, profile.Luck, profile.LifetimeCoinsEarned,
		string(cardJSON), profile.LastCardDrawDate, profile.DailyCardDrawCount,
		string(effectsJSON), profile.LastFightAt, now, profile.UserID)
}

func createDrawnCard(malCard *MALCharacterCard, rarity string) *ArenaCard {
	now := time.Now().UTC().Format(time.RFC3339)
	iv := &IVStats{
		Power: randomInt(CARD_IV_MIN, CARD_IV_MAX),
		Guard: randomInt(CARD_IV_MIN, CARD_IV_MAX),
		Speed: randomInt(CARD_IV_MIN, CARD_IV_MAX),
		Luck:  randomInt(CARD_IV_MIN, CARD_IV_MAX),
	}
	iv.Total = iv.Power + iv.Guard + iv.Speed + iv.Luck
	return &ArenaCard{
		CardInstanceID: makeArenaID("card"), MalID: malCard.MalID,
		Title: malCard.Title, URL: malCard.URL, ImageURL: malCard.ImageURL,
		MeanScore: malCard.MeanScore, Popularity: malCard.Popularity,
		Favorites: malCard.Favorites, NSFW: &malCard.NSFW,
		Rarity: rarity, IV: iv, DrawnAt: &now,
	}
}

func insertCollectionCard(db *sql.DB, userID string, card *ArenaCard) {
	now := time.Now().UTC().Format(time.RFC3339)
	cardJSON, _ := json.Marshal(card)
	db.Exec(`INSERT OR IGNORE INTO arena_card_collection (id, userId, cardInstanceId, cardJson, createdAt, updatedAt)
		VALUES (?, ?, ?, ?, ?, ?)`, makeArenaID("collect"), userID, card.CardInstanceID, string(cardJSON), now, now)
}

func getInventoryMap(db *sql.DB, userID string) map[string]int {
	rows, _ := db.Query("SELECT itemId, quantity FROM arena_inventory WHERE userId = ?", userID)
	if rows == nil {
		return map[string]int{}
	}
	defer rows.Close()
	inv := map[string]int{}
	for rows.Next() {
		var itemID string
		var qty int
		rows.Scan(&itemID, &qty)
		if qty > 0 {
			inv[itemID] = qty
		}
	}
	return inv
}

func addItemToInventory(db *sql.DB, userID, itemID string, qty int) {
	now := time.Now().UTC().Format(time.RFC3339)
	db.Exec(`INSERT INTO arena_inventory (id, userId, itemId, quantity, createdAt, updatedAt)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(userId, itemId) DO UPDATE SET quantity=quantity+?, updatedAt=?`,
		makeArenaID("inv"), userID, itemID, qty, now, now, qty, now)
}

func deductItemFromInventory(db *sql.DB, userID, itemID string, qty int) {
	db.Exec("UPDATE arena_inventory SET quantity=MAX(0, quantity-?), updatedAt=datetime('now') WHERE userId=? AND itemId=?", qty, userID, itemID)
}

func getShopProfile(profile *ArenaProfile) map[string]any {
	return map[string]any{"userId": profile.UserID, "level": profile.Level, "coins": profile.Coins}
}

func canAffordRecipe(profile *ArenaProfile, inventory map[string]int, recipe ShopRecipe) bool {
	if profile.Coins < recipe.CoinCost {
		return false
	}
	for _, input := range recipe.Inputs {
		if inventory[input.ItemID] < input.Quantity {
			return false
		}
	}
	return true
}

func selectMaterialDropTier(playerLevel, opponentLevel int) int {
	pt := tierIndexForLevel(playerLevel)
	ot := tierIndexForLevel(opponentLevel)
	capped := pt
	if ot < capped {
		capped = ot
	}
	if capped < 0 {
		capped = 0
	}
	total := 0
	for i := 0; i <= capped; i++ {
		total += i + 1
	}
	if total <= 0 {
		return 0
	}
	roll := rand.Intn(total)
	for i := 0; i <= capped; i++ {
		roll -= (i + 1)
		if roll < 0 {
			return i
		}
	}
	return capped
}

func tierIndexForLevel(level int) int {
	idx := 0
	for i, t := range TierConfig {
		if level >= t.UnlockLevel {
			idx = i
		}
	}
	return idx
}

var MATERIAL_IDS_BY_TIER [][]string

func init() {
	MATERIAL_IDS_BY_TIER = make([][]string, len(TierConfig))
	for i, t := range TierConfig {
		ids := make([]string, len(t.Materials))
		for j, m := range t.Materials {
			ids[j] = m.ID
		}
		MATERIAL_IDS_BY_TIER[i] = ids
	}
}
