package arena

import (
	"fmt"
	"strconv"
	"strings"
)

var RarityOrder = []string{"C", "R", "SR", "SSR", "UR"}

type RarityConfig struct {
	ID          string
	Weight      int
	PowerBonus  int
	CoinReward  int
}

var RarityConfigMap = map[string]RarityConfig{
	"C":   {ID: "C", Weight: 60, PowerBonus: 0, CoinReward: 0},
	"R":   {ID: "R", Weight: 25, PowerBonus: 3, CoinReward: 3},
	"SR":  {ID: "SR", Weight: 10, PowerBonus: 7, CoinReward: 7},
	"SSR": {ID: "SSR", Weight: 4, PowerBonus: 12, CoinReward: 12},
	"UR":  {ID: "UR", Weight: 1, PowerBonus: 18, CoinReward: 18},
}

type Sprite struct {
	Sheet string `json:"sheet"`
	Row   int    `json:"row"`
	Col   int    `json:"col"`
	Size  int    `json:"size"`
}

type PassiveAction struct {
	Type                string `json:"type"`
	Value               int    `json:"value,omitempty"`
	ChancePct           int    `json:"chancePct,omitempty"`
	MaxTriggersPerFight int    `json:"maxTriggersPerFight,omitempty"`
	Turns               int    `json:"turns,omitempty"`
	Target              string `json:"target,omitempty"`
}

type PassiveEffect struct {
	Key      string          `json:"key"`
	Trigger  string          `json:"trigger"`
	Priority int             `json:"priority"`
	When     []PassiveWhen   `json:"when,omitempty"`
	Actions  []PassiveAction `json:"actions"`
}

type PassiveWhen struct {
	Left  string `json:"left"`
	Op    string `json:"op"`
	Right any    `json:"right"`
}

type GearItem struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Slot        string        `json:"slot"`
	Stats       map[string]int `json:"stats"`
	Sprite      Sprite        `json:"sprite"`
	Passive     *PassiveEffect `json:"passive,omitempty"`
}

type ConsumableEffectDesc struct {
	Kind         string `json:"kind"`
	Pct          int    `json:"pct,omitempty"`
	Amount       int    `json:"amount,omitempty"`
	Wins         int    `json:"wins,omitempty"`
	Fights       int    `json:"fights,omitempty"`
	Charges      int    `json:"charges,omitempty"`
	CooldownDays int    `json:"cooldownDays,omitempty"`
	Value        int    `json:"value,omitempty"`
}

type ConsumableItem struct {
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	Sprite           Sprite                `json:"sprite"`
	ConsumableEffect ConsumableEffectDesc  `json:"consumableEffect"`
}

type MaterialItem struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Sprite Sprite `json:"sprite"`
}

type TierEntry struct {
	Tier          string
	UnlockLevel   int
	MaterialPrices []int
	Gear          []GearItem
	Consumables   []ConsumableItem
	Materials     []MaterialItem
}

var TierConfig = []TierEntry{
	{
		Tier: "Rookie", UnlockLevel: 1, MaterialPrices: []int{90, 120, 150},
		Gear: []GearItem{
			{ID: "rustblade_weapon", Name: "Rustblade", Slot: "weapon", Stats: map[string]int{"power": 6}, Sprite: Sprite{Sheet: "game.png", Row: 4, Col: 1, Size: 32}, Passive: &PassiveEffect{Key: "opening_slash", Trigger: "onAttack", Priority: 10, When: []PassiveWhen{{Left: "attack.turn", Op: "==", Right: float64(1)}}, Actions: []PassiveAction{{Type: "addFlatDamage", Value: 4}}}},
			{ID: "twigbow_weapon", Name: "Twig Bow", Slot: "weapon", Stats: map[string]int{"power": 4, "speed": 3}, Sprite: Sprite{Sheet: "game.png", Row: 5, Col: 4, Size: 32}, Passive: &PassiveEffect{Key: "first_actor_boost", Trigger: "onAttack", Priority: 9, When: []PassiveWhen{{Left: "attack.isFirstActor", Op: "==", Right: true}}, Actions: []PassiveAction{{Type: "scaleDamagePct", Value: 8}}}},
			{ID: "patchwork_helm", Name: "Patchwork Helm", Slot: "armor", Stats: map[string]int{"guard": 6, "hp": 10}, Sprite: Sprite{Sheet: "game.png", Row: 6, Col: 3, Size: 32}, Passive: &PassiveEffect{Key: "patchwork_deflect", Trigger: "onDamageTaken", Priority: 6, Actions: []PassiveAction{{Type: "reduceIncomingDamageFlat", Value: 2, ChancePct: 10}}}},
			{ID: "copper_ring", Name: "Copper Ring", Slot: "charm", Stats: map[string]int{"luck": 4}, Sprite: Sprite{Sheet: "game.png", Row: 7, Col: 4, Size: 32}, Passive: &PassiveEffect{Key: "coin_blessing_small", Trigger: "onWin", Priority: 4, Actions: []PassiveAction{{Type: "rewardBonusPct", Target: "coins", Value: 5}}}},
		},
		Consumables: []ConsumableItem{
			{ID: "red_tonic", Name: "Red Tonic", Sprite: Sprite{Sheet: "game.png", Row: 8, Col: 0, Size: 32}, ConsumableEffect: ConsumableEffectDesc{Kind: "shield_fight_start", Amount: 20, Charges: 5}},
			{ID: "green_draft", Name: "Green Draft", Sprite: Sprite{Sheet: "game.png", Row: 8, Col: 2, Size: 32}, ConsumableEffect: ConsumableEffectDesc{Kind: "exp_boost", Pct: 20, Fights: 10}},
			{ID: "amber_draft", Name: "Amber Draft", Sprite: Sprite{Sheet: "game.png", Row: 8, Col: 3, Size: 32}, ConsumableEffect: ConsumableEffectDesc{Kind: "coin_boost", Pct: 20, Fights: 10}},
		},
		Materials: []MaterialItem{
			{ID: "driftwood_shard", Name: "Driftwood Shard", Sprite: Sprite{Sheet: "game.png", Row: 16, Col: 0, Size: 32}},
			{ID: "satchel_cloth", Name: "Satchel Cloth", Sprite: Sprite{Sheet: "game.png", Row: 6, Col: 12, Size: 32}},
			{ID: "timber_plank", Name: "Timber Plank", Sprite: Sprite{Sheet: "game.png", Row: 18, Col: 11, Size: 32}},
		},
	},
	{
		Tier: "Bronze", UnlockLevel: 8, MaterialPrices: []int{360, 420, 480},
		Gear: []GearItem{
			{ID: "riversteel_saber", Name: "Riversteel Saber", Slot: "weapon", Stats: map[string]int{"power": 12, "speed": 4}, Sprite: Sprite{Sheet: "game.png", Row: 4, Col: 2, Size: 32}, Passive: &PassiveEffect{Key: "riversteel_edge", Trigger: "onDamageDealt", Priority: 10, Actions: []PassiveAction{{Type: "bonusCritChancePct", Value: 10}}}},
			{ID: "guard_cap", Name: "Guard Cap", Slot: "armor", Stats: map[string]int{"guard": 12}, Sprite: Sprite{Sheet: "game.png", Row: 6, Col: 1, Size: 32}, Passive: &PassiveEffect{Key: "guard_cap_focus", Trigger: "onDamageTaken", Priority: 6, Actions: []PassiveAction{{Type: "grantTempGuard", Value: 4, Turns: 1, ChancePct: 20}}}},
			{ID: "iron_cuirass", Name: "Iron Cuirass", Slot: "armor", Stats: map[string]int{"guard": 10, "hp": 18}, Sprite: Sprite{Sheet: "game.png", Row: 6, Col: 6, Size: 32}, Passive: &PassiveEffect{Key: "thorn_reflect_small", Trigger: "onDamageTaken", Priority: 7, Actions: []PassiveAction{{Type: "reflectFlatDamage", Value: 2}}}},
			{ID: "azure_ring", Name: "Azure Ring", Slot: "charm", Stats: map[string]int{"luck": 8, "speed": 5}, Sprite: Sprite{Sheet: "game.png", Row: 7, Col: 5, Size: 32}, Passive: &PassiveEffect{Key: "evasion_primer", Trigger: "onFightStart", Priority: 8, Actions: []PassiveAction{{Type: "addEvasionPct", Value: 4}}}},
		},
		Consumables: []ConsumableItem{
			{ID: "frost_elixir", Name: "Frost Elixir", Sprite: Sprite{Sheet: "game.png", Row: 8, Col: 5, Size: 32}, ConsumableEffect: ConsumableEffectDesc{Kind: "evade_next_fight", Pct: 15, Fights: 5}},
			{ID: "viridian_elixir", Name: "Viridian Elixir", Sprite: Sprite{Sheet: "game.png", Row: 8, Col: 6, Size: 32}, ConsumableEffect: ConsumableEffectDesc{Kind: "upgrade_lowest_rarity", Charges: 1}},
			{ID: "fuse_bomb", Name: "Fuse Bomb", Sprite: Sprite{Sheet: "game.png", Row: 9, Col: 11, Size: 32}, ConsumableEffect: ConsumableEffectDesc{Kind: "first_hit_true_damage", Value: 25, Charges: 1}},
		},
		Materials: []MaterialItem{
			{ID: "azure_ore", Name: "Azure Ore", Sprite: Sprite{Sheet: "game.png", Row: 15, Col: 1, Size: 32}},
			{ID: "gold_ingot", Name: "Gold Ingot", Sprite: Sprite{Sheet: "game.png", Row: 15, Col: 2, Size: 32}},
			{ID: "brown_dust", Name: "Brown Dust", Sprite: Sprite{Sheet: "game.png", Row: 18, Col: 1, Size: 32}},
		},
	},
	{
		Tier: "Silver", UnlockLevel: 16, MaterialPrices: []int{1100, 1300, 1500},
		Gear: []GearItem{
			{ID: "dawnfang_blade", Name: "Dawnfang Blade", Slot: "weapon", Stats: map[string]int{"power": 22, "speed": 6}, Sprite: Sprite{Sheet: "game.png", Row: 4, Col: 6, Size: 32}, Passive: &PassiveEffect{Key: "dawnfang_pressure", Trigger: "onAttack", Priority: 11, When: []PassiveWhen{{Left: "defender.hpPct", Op: ">", Right: float64(70)}}, Actions: []PassiveAction{{Type: "scaleDamagePct", Value: 12}}}},
			{ID: "knight_helm", Name: "Knight Helm", Slot: "armor", Stats: map[string]int{"guard": 24, "hp": 20}, Sprite: Sprite{Sheet: "game.png", Row: 6, Col: 5, Size: 32}, Passive: &PassiveEffect{Key: "knight_wall", Trigger: "onDamageTaken", Priority: 8, Actions: []PassiveAction{{Type: "reduceIncomingDamagePct", Value: 8}}}},
			{ID: "laurel_pendant", Name: "Laurel Pendant", Slot: "charm", Stats: map[string]int{"luck": 12}, Sprite: Sprite{Sheet: "game.png", Row: 7, Col: 6, Size: 32}, Passive: &PassiveEffect{Key: "rarity_coin_blessing", Trigger: "onWin", Priority: 5, Actions: []PassiveAction{{Type: "rarityCoinBonusPct", Value: 10}}}},
			{ID: "verdant_core", Name: "Verdant Core", Slot: "charm", Stats: map[string]int{"hp": 24, "luck": 10}, Sprite: Sprite{Sheet: "game.png", Row: 16, Col: 2, Size: 32}, Passive: &PassiveEffect{Key: "verdant_regen", Trigger: "onDamageTaken", Priority: 7, Actions: []PassiveAction{{Type: "healFlat", Value: 4, MaxTriggersPerFight: 3}}}},
		},
		Consumables: []ConsumableItem{
			{ID: "sun_elixir", Name: "Sun Elixir", Sprite: Sprite{Sheet: "game.png", Row: 8, Col: 7, Size: 32}, ConsumableEffect: ConsumableEffectDesc{Kind: "coin_boost", Pct: 40, Fights: 10}},
			{ID: "star_tonic", Name: "Star Tonic", Sprite: Sprite{Sheet: "game.png", Row: 8, Col: 8, Size: 32}, ConsumableEffect: ConsumableEffectDesc{Kind: "exp_boost", Pct: 40, Fights: 10}},
			{ID: "lantern_oil", Name: "Lantern Oil", Sprite: Sprite{Sheet: "game.png", Row: 9, Col: 8, Size: 32}, ConsumableEffect: ConsumableEffectDesc{Kind: "bonus_vs_higher_rarity", Pct: 10, Charges: 1}},
		},
		Materials: []MaterialItem{
			{ID: "azure_powder", Name: "Azure Powder", Sprite: Sprite{Sheet: "game.png", Row: 18, Col: 6, Size: 32}},
			{ID: "verdant_powder", Name: "Verdant Powder", Sprite: Sprite{Sheet: "game.png", Row: 18, Col: 7, Size: 32}},
			{ID: "clear_crystal", Name: "Clear Crystal", Sprite: Sprite{Sheet: "game.png", Row: 15, Col: 3, Size: 32}},
		},
	},
	{
		Tier: "Gold", UnlockLevel: 28, MaterialPrices: []int{4600, 5200, 5800},
		Gear: []GearItem{
			{ID: "twinlight_blades", Name: "Twinlight Blades", Slot: "weapon", Stats: map[string]int{"power": 34, "speed": 12}, Sprite: Sprite{Sheet: "game.png", Row: 4, Col: 9, Size: 32}, Passive: &PassiveEffect{Key: "double_strike", Trigger: "onDamageDealt", Priority: 12, Actions: []PassiveAction{{Type: "extraStrikePct", ChancePct: 12, Value: 40}}}},
			{ID: "waraxe_howl", Name: "Waraxe Howl", Slot: "weapon", Stats: map[string]int{"power": 38, "speed": 8}, Sprite: Sprite{Sheet: "game.png", Row: 4, Col: 10, Size: 32}, Passive: &PassiveEffect{Key: "crit_burst", Trigger: "onDamageDealt", Priority: 10, When: []PassiveWhen{{Left: "attack.critical", Op: "==", Right: true}}, Actions: []PassiveAction{{Type: "scaleDamagePct", Value: 35}}}},
			{ID: "sky_hood", Name: "Sky Hood", Slot: "armor", Stats: map[string]int{"guard": 34, "hp": 30, "luck": 6}, Sprite: Sprite{Sheet: "game.png", Row: 6, Col: 8, Size: 32}, Passive: &PassiveEffect{Key: "sky_last_stand", Trigger: "onDamageTaken", Priority: 11, When: []PassiveWhen{{Left: "self.hpPct", Op: "<", Right: float64(40)}}, Actions: []PassiveAction{{Type: "applyShield", Value: 20, MaxTriggersPerFight: 1}}}},
			{ID: "violet_core", Name: "Violet Core", Slot: "charm", Stats: map[string]int{"luck": 18, "speed": 10}, Sprite: Sprite{Sheet: "game.png", Row: 16, Col: 4, Size: 32}, Passive: &PassiveEffect{Key: "luck_to_power", Trigger: "onFightStart", Priority: 10, Actions: []PassiveAction{{Type: "scaleLuckIntoPowerPct", Value: 20}}}},
		},
		Consumables: []ConsumableItem{
			{ID: "seeker_lens", Name: "Seeker Lens", Sprite: Sprite{Sheet: "game.png", Row: 9, Col: 7, Size: 32}, ConsumableEffect: ConsumableEffectDesc{Kind: "reroll_keep_higher", Charges: 1}},
			{ID: "oath_ribbon", Name: "Oath Ribbon", Sprite: Sprite{Sheet: "game.png", Row: 10, Col: 10, Size: 32}, ConsumableEffect: ConsumableEffectDesc{Kind: "streak_shield", Charges: 2}},
			{ID: "treasure_cache", Name: "Treasure Cache", Sprite: Sprite{Sheet: "game.png", Row: 10, Col: 11, Size: 32}, ConsumableEffect: ConsumableEffectDesc{Kind: "coin_boost", Pct: 80, Fights: 10}},
		},
		Materials: []MaterialItem{
			{ID: "ember_dust", Name: "Ember Dust", Sprite: Sprite{Sheet: "game.png", Row: 18, Col: 4, Size: 32}},
			{ID: "scarlet_dust", Name: "Scarlet Dust", Sprite: Sprite{Sheet: "game.png", Row: 18, Col: 5, Size: 32}},
			{ID: "gray_feather", Name: "Gray Feather", Sprite: Sprite{Sheet: "game.png", Row: 15, Col: 9, Size: 32}},
		},
	},
	{
		Tier: "Mythic", UnlockLevel: 42, MaterialPrices: []int{16000, 18000, 20000},
		Gear: []GearItem{
			{ID: "reaper_glaive", Name: "Reaper Glaive", Slot: "weapon", Stats: map[string]int{"power": 52, "speed": 14}, Sprite: Sprite{Sheet: "game.png", Row: 4, Col: 12, Size: 32}, Passive: &PassiveEffect{Key: "execute_strike", Trigger: "onAttack", Priority: 14, When: []PassiveWhen{{Left: "defender.hpPct", Op: "<", Right: float64(30)}}, Actions: []PassiveAction{{Type: "addFlatDamage", Value: 18}}}},
			{ID: "wyrm_hood", Name: "Wyrm Hood", Slot: "armor", Stats: map[string]int{"guard": 48, "hp": 42, "luck": 8}, Sprite: Sprite{Sheet: "game.png", Row: 6, Col: 9, Size: 32}, Passive: &PassiveEffect{Key: "crit_nullifier", Trigger: "onDamageTaken", Priority: 15, When: []PassiveWhen{{Left: "attack.critical", Op: "==", Right: true}}, Actions: []PassiveAction{{Type: "cancelCritical", MaxTriggersPerFight: 1}}}},
			{ID: "titan_greaves", Name: "Titan Greaves", Slot: "armor", Stats: map[string]int{"guard": 44, "hp": 46}, Sprite: Sprite{Sheet: "game.png", Row: 7, Col: 2, Size: 32}, Passive: &PassiveEffect{Key: "flat_reduction", Trigger: "onDamageTaken", Priority: 9, Actions: []PassiveAction{{Type: "reduceIncomingDamageFlat", Value: 4}}}},
			{ID: "crimson_core", Name: "Crimson Core", Slot: "charm", Stats: map[string]int{"luck": 22, "power": 8}, Sprite: Sprite{Sheet: "game.png", Row: 16, Col: 0, Size: 32}, Passive: &PassiveEffect{Key: "exp_blessing", Trigger: "onWin", Priority: 5, Actions: []PassiveAction{{Type: "rewardBonusPct", Target: "xp", Value: 12}}}},
		},
		Consumables: []ConsumableItem{
			{ID: "prism_draught", Name: "Prism Draught", Sprite: Sprite{Sheet: "game.png", Row: 8, Col: 14, Size: 32}, ConsumableEffect: ConsumableEffectDesc{Kind: "guarantee_ssr_plus", Charges: 1}},
			{ID: "sacred_candles", Name: "Sacred Candles", Sprite: Sprite{Sheet: "game.png", Row: 9, Col: 10, Size: 32}, ConsumableEffect: ConsumableEffectDesc{Kind: "shield_fight_start", Amount: 35, Charges: 5}},
			{ID: "gate_key", Name: "Gate Key", Sprite: Sprite{Sheet: "game.png", Row: 10, Col: 9, Size: 32}, ConsumableEffect: ConsumableEffectDesc{Kind: "cooldown_bypass", Charges: 1}},
		},
		Materials: []MaterialItem{
			{ID: "arcane_powder", Name: "Arcane Powder", Sprite: Sprite{Sheet: "game.png", Row: 18, Col: 8, Size: 32}},
			{ID: "ivory_feather", Name: "Ivory Feather", Sprite: Sprite{Sheet: "game.png", Row: 15, Col: 10, Size: 32}},
			{ID: "rose_crystal", Name: "Rose Crystal", Sprite: Sprite{Sheet: "game.png", Row: 15, Col: 5, Size: 32}},
		},
	},
	{
		Tier: "Cosmic", UnlockLevel: 58, MaterialPrices: []int{52000, 56000, 60000},
		Gear: []GearItem{
			{ID: "orbit_scepter", Name: "Orbit Scepter", Slot: "weapon", Stats: map[string]int{"power": 64, "speed": 20, "luck": 8}, Sprite: Sprite{Sheet: "game.png", Row: 4, Col: 11, Size: 32}, Passive: &PassiveEffect{Key: "speed_surge_damage", Trigger: "onAttack", Priority: 12, Actions: []PassiveAction{{Type: "scaleBySpeedPct", Value: 15}}}},
			{ID: "aegis_crown", Name: "Aegis Crown", Slot: "armor", Stats: map[string]int{"guard": 60, "hp": 56, "luck": 12}, Sprite: Sprite{Sheet: "game.png", Row: 6, Col: 4, Size: 32}, Passive: &PassiveEffect{Key: "first_hits_guard", Trigger: "onDamageTaken", Priority: 16, Actions: []PassiveAction{{Type: "reduceIncomingDamagePct", Value: 30, MaxTriggersPerFight: 2}}}},
			{ID: "azure_core", Name: "Azure Core", Slot: "charm", Stats: map[string]int{"luck": 26, "speed": 16, "guard": 10}, Sprite: Sprite{Sheet: "game.png", Row: 16, Col: 1, Size: 32}, Passive: &PassiveEffect{Key: "counter_burst", Trigger: "onDamageTaken", Priority: 9, Actions: []PassiveAction{{Type: "counterDamagePct", ChancePct: 20, Value: 50}}}},
			{ID: "void_core", Name: "Void Core", Slot: "charm", Stats: map[string]int{"luck": 28, "speed": 18, "power": 10}, Sprite: Sprite{Sheet: "game.png", Row: 16, Col: 5, Size: 32}, Passive: &PassiveEffect{Key: "void_pressure", Trigger: "onFightStart", Priority: 11, Actions: []PassiveAction{{Type: "reduceOpponentLuckPct", Value: 20}}}},
		},
		Consumables: []ConsumableItem{
			{ID: "solar_cauldron", Name: "Solar Cauldron", Sprite: Sprite{Sheet: "game.png", Row: 17, Col: 8, Size: 32}, ConsumableEffect: ConsumableEffectDesc{Kind: "ascension", CooldownDays: 7}},
			{ID: "void_cauldron", Name: "Void Cauldron", Sprite: Sprite{Sheet: "game.png", Row: 17, Col: 9, Size: 32}, ConsumableEffect: ConsumableEffectDesc{Kind: "double_passive_trigger", Fights: 3}},
			{ID: "chrono_vial", Name: "Chrono Vial", Sprite: Sprite{Sheet: "game.png", Row: 17, Col: 4, Size: 32}, ConsumableEffect: ConsumableEffectDesc{Kind: "restore_consumable_charge", Charges: 1}},
		},
		Materials: []MaterialItem{
			{ID: "verdant_gem", Name: "Verdant Gem", Sprite: Sprite{Sheet: "game.png", Row: 18, Col: 9, Size: 32}},
			{ID: "pale_gem", Name: "Pale Gem", Sprite: Sprite{Sheet: "game.png", Row: 18, Col: 10, Size: 32}},
			{ID: "lunar_gem", Name: "Lunar Gem", Sprite: Sprite{Sheet: "game.png", Row: 18, Col: 11, Size: 32}},
		},
	},
}

var ShopTiers []string
var TierUnlockLevels map[string]int
var CARD_IV_MIN = 0
var CARD_IV_MAX = 31
var CHARACTER_FAVORITES_MIN = 53
var CHARACTER_FAVORITES_UR_MIN = 30000
var CATALOG_VERSION = "v2"
var FIGHT_COOLDOWN_MS = 5000
var DAILY_CARD_DRAW_LIMIT = 10

var GEAR_CRAFT_COIN_FEES = []int{80, 350, 1200, 5000, 18000, 60000}
var CONSUMABLE_CRAFT_COIN_FEES = []int{60, 250, 800, 2500, 9000, 30000}

var BASE_PROFILE = map[string]int{
	"level":  1, "xp": 0, "coins": 0, "wins": 0, "losses": 0, "winStreak": 0,
	"hp": 120, "power": 12, "guard": 12, "speed": 10, "luck": 6, "lifetimeCoinsEarned": 0,
}

var LEVEL_UP_GAINS = map[string]int{
	"hp": 8, "power": 2, "guard": 2, "speed": 1, "luck": 1,
}

var LEGACY_ITEM_MAP = map[string]string{
	"tin_sword": "rustblade_weapon", "worn_jacket": "patchwork_helm", "lucky_clip": "copper_ring",
	"cracked_xp_tome": "green_draft", "small_coin_ticket": "amber_draft",
	"iron_katana": "riversteel_saber", "reinforced_vest": "iron_cuirass", "rabbit_foot": "azure_ring",
	"refocus_potion": "viridian_elixir", "streak_shield": "oath_ribbon",
	"moonsteel_blade": "dawnfang_blade", "aegis_coat": "knight_helm", "star_pendant": "laurel_pendant",
	"veteran_manual": "star_tonic", "golden_contract": "sun_elixir",
	"dragonfang_saber": "waraxe_howl", "saint_guard_plate": "sky_hood", "oracle_sigil": "violet_core",
	"ssr_lure": "seeker_lens", "fortune_vault": "treasure_cache",
	"celestial_reaper": "reaper_glaive", "eternal_aegis": "aegis_crown", "fate_crown": "void_core",
	"ur_sigil": "prism_draught", "ascension_scroll": "solar_cauldron",
}

type ShopItem struct {
	ID              string                `json:"id"`
	Name            string                `json:"name"`
	Tier            string                `json:"tier"`
	UnlockLevel     int                   `json:"unlockLevel"`
	Price           int                   `json:"price"`
	Type            string                `json:"type"`
	Slot            string                `json:"slot,omitempty"`
	Acquisition     string                `json:"acquisition"`
	Stats           map[string]int        `json:"stats,omitempty"`
	Sprite          Sprite                `json:"sprite"`
	Passive         *PassiveEffect        `json:"passive,omitempty"`
	ConsumableEffect *ConsumableEffectDesc `json:"consumableEffect,omitempty"`
	RecipeID        string                `json:"recipeId,omitempty"`
}

type ShopRecipe struct {
	ID          string               `json:"id"`
	Tier        string               `json:"tier"`
	UnlockLevel int                  `json:"unlockLevel"`
	Output      RecipeOutput         `json:"output"`
	CoinCost    int                  `json:"coinCost"`
	Inputs      []RecipeInput        `json:"inputs"`
}

type RecipeOutput struct {
	ItemID   string `json:"itemId"`
	Quantity int    `json:"quantity"`
}

type RecipeInput struct {
	ItemID   string `json:"itemId"`
	Quantity int    `json:"quantity"`
}

var ShopItems []ShopItem
var ShopRecipes []ShopRecipe
var ShopItemsByID map[string]ShopItem
var ShopRecipesByID map[string]ShopRecipe

func init() {
	ShopTiers = make([]string, len(TierConfig))
	for i, t := range TierConfig {
		ShopTiers[i] = t.Tier
	}
	TierUnlockLevels = make(map[string]int)
	for _, t := range TierConfig {
		TierUnlockLevels[t.Tier] = t.UnlockLevel
	}

	ShopRecipes = buildRecipes()
	ShopRecipesByID = make(map[string]ShopRecipe)
	recipeByOutput := make(map[string]string)
	for _, r := range ShopRecipes {
		ShopRecipesByID[r.ID] = r
		recipeByOutput[r.Output.ItemID] = r.ID
	}

	ShopItemsByID = make(map[string]ShopItem)
	for _, t := range TierConfig {
		for _, g := range t.Gear {
			recipeID := ""
			if id, ok := recipeByOutput[g.ID]; ok {
				recipeID = id
			}
			si := ShopItem{
				ID: g.ID, Name: g.Name, Tier: t.Tier, UnlockLevel: t.UnlockLevel,
				Price: 0, Type: "gear", Slot: g.Slot, Acquisition: "craft",
				Stats: g.Stats, Sprite: g.Sprite, Passive: g.Passive, RecipeID: recipeID,
			}
			ShopItems = append(ShopItems, si)
			ShopItemsByID[si.ID] = si
		}
		for _, c := range t.Consumables {
			recipeID := ""
			if id, ok := recipeByOutput[c.ID]; ok {
				recipeID = id
			}
			si := ShopItem{
				ID: c.ID, Name: c.Name, Tier: t.Tier, UnlockLevel: t.UnlockLevel,
				Price: 0, Type: "consumable", Acquisition: "craft",
				Sprite: c.Sprite, ConsumableEffect: &c.ConsumableEffect, RecipeID: recipeID,
			}
			ShopItems = append(ShopItems, si)
			ShopItemsByID[si.ID] = si
		}
		for i, m := range t.Materials {
			price := 0
			if i < len(t.MaterialPrices) {
				price = t.MaterialPrices[i]
			}
			si := ShopItem{
				ID: m.ID, Name: m.Name, Tier: t.Tier, UnlockLevel: t.UnlockLevel,
				Price: price, Type: "material", Acquisition: "buy", Sprite: m.Sprite,
			}
			ShopItems = append(ShopItems, si)
			ShopItemsByID[si.ID] = si
		}
	}
}

func buildRecipes() []ShopRecipe {
	var recipes []ShopRecipe
	gearSpecs := [][][]string{
		{{"a", "4"}, {"b", "2"}, {"c", "1"}},
		{{"a", "2"}, {"b", "4"}, {"c", "1"}},
		{{"a", "1"}, {"b", "2"}, {"c", "4"}},
		{{"a", "3"}, {"b", "3"}, {"c", "2"}},
	}
	consumableSpecs := [][][]string{
		{{"a", "2"}, {"c", "1"}},
		{{"b", "2"}, {"a", "1"}},
		{{"c", "2"}, {"b", "1"}},
	}

	for ti, tc := range TierConfig {
		tierSlug := strings.ToLower(tc.Tier)
		matIDs := make([]string, len(tc.Materials))
		for i, m := range tc.Materials {
			matIDs[i] = m.ID
		}
		materialMap := map[string]string{"a": matIDs[0], "b": matIDs[1], "c": matIDs[2]}

		for gi, g := range tc.Gear {
			var inputs []RecipeInput
			if gi < len(gearSpecs) {
				for _, spec := range gearSpecs[gi] {
					itemID := materialMap[spec[0]]
					qty := atoi(spec[1])
					inputs = append(inputs, RecipeInput{ItemID: itemID, Quantity: qty})
				}
			}
			recipes = append(recipes, ShopRecipe{
				ID:          fmt.Sprintf("%s_gear_%d", tierSlug, gi+1),
				Tier:        tc.Tier,
				UnlockLevel: tc.UnlockLevel,
				Output:      RecipeOutput{ItemID: g.ID, Quantity: 1},
				CoinCost:    GEAR_CRAFT_COIN_FEES[ti],
				Inputs:      inputs,
			})
		}

		for ci, c := range tc.Consumables {
			var inputs []RecipeInput
			if ci < len(consumableSpecs) {
				for _, spec := range consumableSpecs[ci] {
					itemID := materialMap[spec[0]]
					qty := atoi(spec[1])
					inputs = append(inputs, RecipeInput{ItemID: itemID, Quantity: qty})
				}
			}
			recipes = append(recipes, ShopRecipe{
				ID:          fmt.Sprintf("%s_cons_%d", tierSlug, ci+1),
				Tier:        tc.Tier,
				UnlockLevel: tc.UnlockLevel,
				Output:      RecipeOutput{ItemID: c.ID, Quantity: 1},
				CoinCost:    CONSUMABLE_CRAFT_COIN_FEES[ti],
				Inputs:      inputs,
			})
		}
	}
	return recipes
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
