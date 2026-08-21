// Package catalog is a fixed, in-code gift list -- same reasoning as
// chat-service's moderation.bannedWords: this demonstrates the mechanism
// (looking up a type, getting a coin cost, rejecting anything not in the
// list), not a real catalog. A real one would be a DB table an ops team
// or creators themselves manage, not a Go map.
package catalog

var Gifts = map[string]int64{
	"rose":    10,
	"heart":   50,
	"diamond": 500,
	"rocket":  1000,
}

func CoinCost(giftType string) (int64, bool) {
	cost, ok := Gifts[giftType]
	return cost, ok
}
