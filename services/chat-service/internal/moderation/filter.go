// Package moderation holds pure content-policy checks: no state, no I/O.
// Session-scoped moderation state (mutes, rate limits) lives in
// internal/realtime instead, since that's backed by Redis.
package moderation

import "strings"

// bannedWords is intentionally small and obvious -- this demonstrates the
// mechanism (reject before it's ever broadcast or persisted), not a real
// moderation wordlist. A production system would swap this for a managed
// list or a third-party moderation API without touching the call site.
var bannedWords = []string{"badword1", "badword2", "slur1"}

func ContainsBannedWord(body string) bool {
	lower := strings.ToLower(body)
	for _, w := range bannedWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

var allowedReactions = map[string]bool{
	"👍": true, "❤️": true, "🔥": true, "😂": true, "😮": true, "🎉": true,
}

func IsAllowedReaction(body string) bool {
	return allowedReactions[body]
}

const MaxMessageLength = 500
