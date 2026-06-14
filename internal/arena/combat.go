package arena

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"time"
)

type ArenaProfile struct {
	UserID              string     `json:"userId"`
	Level               int        `json:"level"`
	XP                  int        `json:"xp"`
	Coins               int        `json:"coins"`
	Wins                int        `json:"wins"`
	Losses              int        `json:"losses"`
	WinStreak           int        `json:"winStreak"`
	HP                  int        `json:"hp"`
	Power               int        `json:"power"`
	Guard               int        `json:"guard"`
	Speed               int        `json:"speed"`
	Luck                int        `json:"luck"`
	LifetimeCoinsEarned int        `json:"lifetimeCoinsEarned"`
	SelectedCard        *ArenaCard `json:"selectedCard"`
	LastCardDrawDate    *string    `json:"lastCardDrawDate"`
	DailyCardDrawCount  int        `json:"dailyCardDrawCount"`
	CatalogVersion      string     `json:"catalogVersion"`
	Effects             *ArenaEffects `json:"effects"`
	LastFightAt         *string    `json:"lastFightAt"`
	CreatedAt           *string    `json:"createdAt"`
	UpdatedAt           *string    `json:"updatedAt"`
}

// ... types same as before, no change ...

type ArenaCard struct {
	CardInstanceID string   `json:"cardInstanceId"`
	MalID          int      `json:"malId"`
	Title          string   `json:"title"`
	URL            string   `json:"url"`
	ImageURL       string   `json:"imageUrl"`
	MeanScore      *float64 `json:"meanScore"`
	Popularity     *int     `json:"popularity"`
	Favorites      *int     `json:"favorites"`
	NSFW           *string  `json:"nsfw"`
	Rarity         string   `json:"rarity"`
	IV             *IVStats `json:"iv"`
	DrawnAt        *string  `json:"drawnAt"`
}

type IVStats struct {
	Power int `json:"power"`
	Guard int `json:"guard"`
	Speed int `json:"speed"`
	Luck  int `json:"luck"`
	Total int `json:"total"`
}

type ArenaEffects struct {
	ExpBoostPct                    int     `json:"expBoostPct"`
	ExpBoostWinsRemaining          int     `json:"expBoostWinsRemaining"`
	CoinBoostPct                   int     `json:"coinBoostPct"`
	CoinBoostWinsRemaining         int     `json:"coinBoostWinsRemaining"`
	RerollKeepHigherCharges        int     `json:"rerollKeepHigherCharges"`
	StreakShieldCharges            int     `json:"streakShieldCharges"`
	UpgradeLowestRarityCharges     int     `json:"upgradeLowestRarityCharges"`
	GuaranteeSsrPlusCharges        int     `json:"guaranteeSsrPlusCharges"`
	AscensionLastPurchasedAt       *string `json:"ascensionLastPurchasedAt"`
	FightStartShieldCharges        int     `json:"fightStartShieldCharges"`
	FightStartShieldAmount         int     `json:"fightStartShieldAmount"`
	EvadeBoostPct                  int     `json:"evadeBoostPct"`
	EvadeBoostFightsRemaining      int     `json:"evadeBoostFightsRemaining"`
	FirstHitTrueDamageCharges      int     `json:"firstHitTrueDamageCharges"`
	FirstHitTrueDamageValue        int     `json:"firstHitTrueDamageValue"`
	HigherRarityDamageBonusPctCharges int  `json:"higherRarityDamageBonusPctCharges"`
	HigherRarityDamageBonusPct     int     `json:"higherRarityDamageBonusPct"`
	GateKeyCharges                 int     `json:"gateKeyCharges"`
	DoublePassiveTriggerFightsRemaining int `json:"doublePassiveTriggerFightsRemaining"`
}

type PublicProfile struct {
	UserID              string              `json:"userId"`
	Level               int                 `json:"level"`
	XP                  int                 `json:"xp"`
	XPToNext            int                 `json:"xpToNext"`
	XPProgress          float64             `json:"xpProgress"`
	Coins               int                 `json:"coins"`
	Wins                int                 `json:"wins"`
	Losses              int                 `json:"losses"`
	TotalFights         int                 `json:"totalFights"`
	WinRate             float64             `json:"winRate"`
	WinStreak           int                 `json:"winStreak"`
	Stats               *PublicStats         `json:"stats"`
	SelectedCard        *ArenaCard           `json:"selectedCard"`
	CanDrawCard         bool                `json:"canDrawCard"`
	DailyDrawLimit      int                 `json:"dailyDrawLimit"`
	DailyDrawsUsed      int                 `json:"dailyDrawsUsed"`
	DailyDrawsRemaining int                 `json:"dailyDrawsRemaining"`
	NextCardDrawAt      *string             `json:"nextCardDrawAt"`
	LastCardDrawDate    *string             `json:"lastCardDrawDate"`
	LifetimeCoinsEarned int                 `json:"lifetimeCoinsEarned"`
	Effects             *ArenaEffects       `json:"effects"`
	Equipment           map[string]any      `json:"equipment"`
	ActivePassives      []ActivePassive     `json:"activePassives"`
	MaterialInventory   map[string]int      `json:"materialInventory"`
	RecentFights        []FightSummary      `json:"recentFights"`
	LastFightAt         *string             `json:"lastFightAt"`
	CreatedAt           *string             `json:"createdAt"`
	UpdatedAt           *string             `json:"updatedAt"`
}

type PublicStats struct {
	Base      map[string]int `json:"base"`
	Equipment map[string]int `json:"equipment"`
	Total     map[string]int `json:"total"`
}

type ActivePassive struct {
	Key      string          `json:"key"`
	Trigger  string          `json:"trigger"`
	Priority int             `json:"priority"`
	When     []PassiveWhen   `json:"when,omitempty"`
	Actions  []PassiveAction `json:"actions"`
	Source   PassiveSource   `json:"source"`
}

type PassiveSource struct {
	ItemID     string `json:"itemId"`
	ItemName   string `json:"itemName"`
	Slot       string `json:"slot"`
	Tier       string `json:"tier"`
	EquippedAt string `json:"equippedAt"`
}

type FightSummary struct {
	ID             string       `json:"id"`
	OpponentUserID *string      `json:"opponentUserId"`
	Result         string       `json:"result"`
	Rounds         []FightRound `json:"rounds"`
	XPDelta        int          `json:"xpDelta"`
	CoinDelta      int          `json:"coinDelta"`
	CreatedAt      *string      `json:"createdAt"`
}

type FightRound struct {
	PlayerDamage   int    `json:"playerDamage"`
	OpponentDamage int    `json:"opponentDamage"`
	PlayerCrit     bool   `json:"playerCrit"`
	OpponentCrit   bool   `json:"opponentCrit"`
	PlayerEvaded   bool   `json:"playerEvaded"`
	OpponentEvaded bool   `json:"opponentEvaded"`
	Winner         string `json:"winner"`
}

type AttackResult struct {
	Avoided  bool `json:"avoided"`
	Critical bool `json:"critical"`
	Damage   int  `json:"damage"`
}

// --- Profile management ---

func EnsureArenaProfile(db *sql.DB, userID string) (*ArenaProfile, error) {
	profile, err := getArenaProfile(db, userID)
	if err == nil {
		if profile.CatalogVersion != CATALOG_VERSION {
			db.Exec("UPDATE arena_profiles SET catalogVersion = ?, updatedAt = datetime('now') WHERE userId = ?", CATALOG_VERSION, userID)
			profile, _ = getArenaProfile(db, userID)
		}
		return profile, nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	effectsJSON, _ := json.Marshal(defaultEffects())
	_, err = db.Exec(`
		INSERT INTO arena_profiles (userId, level, xp, coins, wins, losses, winStreak,
			hp, power, guard, speed, luck, lifetimeCoinsEarned,
			catalogVersion, effectsJson, createdAt, updatedAt)
		VALUES (?, 1, 0, 0, 0, 0, 0, 120, 12, 12, 10, 6, 0, ?, ?, ?, ?)
	`, userID, CATALOG_VERSION, string(effectsJSON), now, now)
	if err != nil {
		return nil, err
	}

	return getArenaProfile(db, userID)
}

func getArenaProfile(db *sql.DB, userID string) (*ArenaProfile, error) {
	p := &ArenaProfile{}
	var selectedCardJSON, effectsJSON sql.NullString
	err := db.QueryRow(`
		SELECT userId, level, xp, coins, wins, losses, winStreak, hp, power, guard, speed, luck,
		       lifetimeCoinsEarned, selectedCardJson, lastCardDrawDate, dailyCardDrawCount,
		       catalogVersion, effectsJson, lastFightAt, createdAt, updatedAt
		FROM arena_profiles WHERE userId = ?
	`, userID).Scan(
		&p.UserID, &p.Level, &p.XP, &p.Coins, &p.Wins, &p.Losses, &p.WinStreak,
		&p.HP, &p.Power, &p.Guard, &p.Speed, &p.Luck,
		&p.LifetimeCoinsEarned, &selectedCardJSON, &p.LastCardDrawDate, &p.DailyCardDrawCount,
		&p.CatalogVersion, &effectsJSON, &p.LastFightAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if selectedCardJSON.Valid && selectedCardJSON.String != "" {
		var card ArenaCard
		if err := json.Unmarshal([]byte(selectedCardJSON.String), &card); err == nil {
			p.SelectedCard = &card
		}
	}

	p.Effects = defaultEffects()
	if effectsJSON.Valid && effectsJSON.String != "" {
		var effects ArenaEffects
		if err := json.Unmarshal([]byte(effectsJSON.String), &effects); err == nil {
			normalizeEffects(&effects)
			p.Effects = &effects
		}
	}

	return p, nil
}

func defaultEffects() *ArenaEffects { return &ArenaEffects{} }

func normalizeEffects(e *ArenaEffects) {
	if e.ExpBoostPct < 0 { e.ExpBoostPct = 0 }
	if e.ExpBoostPct > 200 { e.ExpBoostPct = 200 }
	if e.CoinBoostPct < 0 { e.CoinBoostPct = 0 }
	if e.CoinBoostPct > 200 { e.CoinBoostPct = 200 }
	if e.EvadeBoostPct < 0 { e.EvadeBoostPct = 0 }
	if e.EvadeBoostPct > 95 { e.EvadeBoostPct = 95 }
	if e.FightStartShieldAmount < 0 { e.FightStartShieldAmount = 0 }
	if e.FightStartShieldAmount > 9999 { e.FightStartShieldAmount = 9999 }
	if e.FirstHitTrueDamageValue < 0 { e.FirstHitTrueDamageValue = 0 }
	if e.FirstHitTrueDamageValue > 9999 { e.FirstHitTrueDamageValue = 9999 }
	if e.HigherRarityDamageBonusPct < 0 { e.HigherRarityDamageBonusPct = 0 }
	if e.HigherRarityDamageBonusPct > 300 { e.HigherRarityDamageBonusPct = 300 }
}

func XPToNext(level int) int {
	if level < 1 {
		level = 1
	}
	return 80 + 40*level*level
}

func applyLevelUps(profile *ArenaProfile) int {
	leveledUp := 0
	for profile.XP >= XPToNext(profile.Level) {
		profile.XP -= XPToNext(profile.Level)
		profile.Level++
		profile.HP += LEVEL_UP_GAINS["hp"]
		profile.Power += LEVEL_UP_GAINS["power"]
		profile.Guard += LEVEL_UP_GAINS["guard"]
		profile.Speed += LEVEL_UP_GAINS["speed"]
		profile.Luck += LEVEL_UP_GAINS["luck"]
		leveledUp++
	}
	return leveledUp
}

func CalculateRoundPower(power, guard, speed, luck int, rarity string, card *ArenaCard) float64 {
	rarityPower := RarityConfigMap[rarity].PowerBonus
	malScoreBonus := malScorePowerBonus(card)
	popBonus := popularityPowerBonus(card)
	noise := float64(rand.Intn(21) - 10)
	return float64(power)*2.0 + float64(guard)*1.7 + float64(speed)*1.5 + float64(luck)*1.0 +
		float64(rarityPower) + float64(malScoreBonus) + float64(popBonus) + noise
}

func malScorePowerBonus(card *ArenaCard) int {
	if card == nil || card.MeanScore == nil { return 0 }
	bonus := int((*card.MeanScore - 6) * 4)
	if bonus < 0 { bonus = 0 }
	if bonus > 16 { bonus = 16 }
	return bonus
}

func popularityPowerBonus(card *ArenaCard) int {
	if card == nil || card.Popularity == nil { return 0 }
	bonus := int((2500 - float64(*card.Popularity)) / 250)
	if bonus < 0 { bonus = 0 }
	if bonus > 10 { bonus = 10 }
	return bonus
}

func CalculateAttackOutcome(attackerStats, defenderStats map[string]int, attackerRarity string, bonusCritPct, extraDefEvasionPct int) *AttackResult {
	evasionChance := computeEvasionChance(attackerStats, defenderStats, extraDefEvasionPct)
	if rand.Float64() < evasionChance {
		return &AttackResult{Avoided: true}
	}
	rarityPower := RarityConfigMap[attackerRarity].PowerBonus
	attackRoll := float64(getMapInt(attackerStats, "power"))*1.8 + float64(getMapInt(attackerStats, "speed"))*0.7 + float64(getMapInt(attackerStats, "luck"))*0.4 + float64(rarityPower)*0.65 + float64(rand.Intn(19)-6)
	defenseRoll := float64(getMapInt(defenderStats, "guard"))*1.6 + float64(getMapInt(defenderStats, "speed"))*0.35 + float64(getMapInt(defenderStats, "luck"))*0.25 + float64(rand.Intn(13)-4)
	damage := int(math.Max(1, attackRoll-defenseRoll*0.55))
	critChance := math.Max(0.05, math.Min(0.95, 0.05+float64(getMapInt(attackerStats, "luck"))*0.0035+float64(bonusCritPct)/100))
	critical := rand.Float64() < critChance
	if critical { damage = int(math.Max(1, math.Floor(float64(damage)*1.5))) }
	return &AttackResult{Avoided: false, Critical: critical, Damage: int(math.Max(1, float64(damage)))}
}

func computeEvasionChance(attackerStats, defenderStats map[string]int, extraDefEvasionPct int) float64 {
	chance := 0.04 + float64(getMapInt(defenderStats, "speed"))*0.002 + float64(getMapInt(defenderStats, "luck"))*0.0015 - float64(getMapInt(attackerStats, "speed"))*0.001 + float64(extraDefEvasionPct)/100
	if chance < 0.02 { chance = 0.02 }
	if chance > 0.8 { chance = 0.8 }
	return chance
}

func computeMaxHP(stats map[string]int) int {
	result := getMapInt(stats, "hp") + int(math.Floor(float64(getMapInt(stats, "guard"))*2.2)) + int(math.Floor(float64(getMapInt(stats, "power")+getMapInt(stats, "speed")+getMapInt(stats, "luck"))*0.2))
	if result < 30 { result = 30 }
	return result
}

func CalculateWinXP(opponentLevel, roundsWon, currentWinStreak int) int {
	streak := currentWinStreak
	if streak > 10 { streak = 10 }
	return 10 + int(float64(opponentLevel)*1.2) + roundsWon*2 + streak
}

func CalculateWinCoins(opponentLevel, rarityCoinReward, totalLuck int) int {
	return 18 + opponentLevel*3 + rarityCoinReward + totalLuck/5
}

func getMapInt(m map[string]int, key string) int {
	if v, ok := m[key]; ok { return v }
	return 0
}

func getCurrentRecordedDate() string { return time.Now().UTC().Format("2006-01-02") }
func randomInt(min, max int) int {
	if min >= max { return min }
	return rand.Intn(max-min+1) + min
}
func makeArenaID(prefix string) string { return fmt.Sprintf("%s-%d-%s", prefix, time.Now().UnixMilli(), randomString(6)) }
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b { b[i] = letters[rand.Intn(len(letters))] }
	return string(b)
}

type ArenaHTTPError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}
func (e *ArenaHTTPError) Error() string { return e.Message }
