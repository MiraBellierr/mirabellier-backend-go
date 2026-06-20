package arena

import "testing"

func favoriteCount(value int) *int {
	return &value
}

func TestRarityFromFavoritesMakes30KAndAboveUR(t *testing.T) {
	tests := []struct {
		favorites *int
		want      string
	}{
		{favorites: favoriteCount(0), want: "C"},
		{favorites: favoriteCount(53), want: "C"},
		{favorites: favoriteCount(7000), want: "R"},
		{favorites: favoriteCount(15000), want: "SR"},
		{favorites: favoriteCount(25000), want: "SSR"},
		{favorites: favoriteCount(29999), want: "SSR"},
		{favorites: favoriteCount(30000), want: "UR"},
		{favorites: favoriteCount(60000), want: "UR"},
	}

	for _, test := range tests {
		if got := RarityFromFavorites(test.favorites); got != test.want {
			t.Fatalf("RarityFromFavorites(%d) = %q, want %q", *test.favorites, got, test.want)
		}
	}
}
