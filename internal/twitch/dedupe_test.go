package twitch

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"sctroll/internal/config"
)

// notification baut die Nutzlast einer Einloesungs-Benachrichtigung nach.
func notification(redemptionID, subscriptionID string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{
		"subscription": {"id": %q, "type": "channel.channel_points_custom_reward_redemption.add"},
		"event": {"id": %q, "user_name": "tester", "reward": {"id": "reward-1", "title": "Boost!"}}
	}`, subscriptionID, redemptionID))
}

// TestDuplicateRedemptionRunsOnce haelt den Fehler fest, der im Log eines
// Nutzers 20-mal auftrat: Twitch stellte dieselbe Einloesung zweimal zu, die
// zweite Kopie lief in den Cooldown der ersten, und der Cooldown-Zweig
// erstattete die Punkte -- die Aktion lief also, der Zuschauer bezahlte nicht.
func TestDuplicateRedemptionRunsOnce(t *testing.T) {
	c := NewClient(&config.TwitchConfig{})

	var got []string
	c.SetOnRedemption(func(rewardID, redemptionID, userName, rewardTitle string) {
		got = append(got, redemptionID)
	})

	c.handleNotification("msg-1", notification("redemption-1", "sub-1"))
	c.handleNotification("msg-1", notification("redemption-1", "sub-1"))

	if len(got) != 1 {
		t.Fatalf("Einloesung %d-mal ausgefuehrt, erwartet genau einmal: %v", len(got), got)
	}
}

// Eine Doppelung ueber eine zweite Anmeldung traegt eine andere message_id und
// eine andere subscription_id. Genau deshalb wird nach der Einloesungs-ID
// entdoppelt und nicht nach der message_id, die Twitch dafuer vorschlaegt.
func TestDuplicateAcrossSubscriptionsRunsOnce(t *testing.T) {
	c := NewClient(&config.TwitchConfig{})

	calls := 0
	c.SetOnRedemption(func(rewardID, redemptionID, userName, rewardTitle string) { calls++ })

	c.handleNotification("msg-1", notification("redemption-1", "sub-1"))
	c.handleNotification("msg-2", notification("redemption-1", "sub-2"))

	if calls != 1 {
		t.Fatalf("Einloesung %d-mal ausgefuehrt, erwartet genau einmal", calls)
	}
}

func TestDistinctRedemptionsAllRun(t *testing.T) {
	c := NewClient(&config.TwitchConfig{})

	calls := 0
	c.SetOnRedemption(func(rewardID, redemptionID, userName, rewardTitle string) { calls++ })

	for i := 0; i < 5; i++ {
		c.handleNotification("msg", notification(fmt.Sprintf("redemption-%d", i), "sub-1"))
	}

	if calls != 5 {
		t.Fatalf("%d von 5 Einloesungen ausgefuehrt", calls)
	}
}

// Ohne Einloesungs-ID laesst sich nicht entdoppeln. Dann lieber ausfuehren als
// verwerfen: eine verschluckte Einloesung waere der schlimmere Fehler.
func TestRedemptionWithoutIDStillRuns(t *testing.T) {
	c := NewClient(&config.TwitchConfig{})

	calls := 0
	c.SetOnRedemption(func(rewardID, redemptionID, userName, rewardTitle string) { calls++ })

	c.handleNotification("msg-1", notification("", "sub-1"))
	c.handleNotification("msg-2", notification("", "sub-1"))

	if calls != 2 {
		t.Fatalf("%d Ausfuehrungen, erwartet 2", calls)
	}
}

// Die Merkliste darf nicht unbegrenzt wachsen.
func TestSeenListStaysBounded(t *testing.T) {
	c := NewClient(&config.TwitchConfig{})
	c.SetOnRedemption(func(rewardID, redemptionID, userName, rewardTitle string) {})

	for i := 0; i < dedupeMax*3; i++ {
		c.handleNotification("msg", notification(fmt.Sprintf("redemption-%d", i), "sub-1"))
	}

	c.mu.RLock()
	n := len(c.seen)
	c.mu.RUnlock()

	if n > dedupeMax {
		t.Fatalf("Merkliste auf %d Eintraege gewachsen, erlaubt sind %d", n, dedupeMax)
	}
}

// Abgelaufene Eintraege werden beim Aufraeumen bevorzugt verworfen, frische
// bleiben liegen -- sonst faellt die Doppelung durch, die gerade unterwegs ist.
func TestExpiredEntriesDroppedFirst(t *testing.T) {
	c := NewClient(&config.TwitchConfig{})
	c.SetOnRedemption(func(rewardID, redemptionID, userName, rewardTitle string) {})

	c.mu.Lock()
	c.seen = make(map[string]time.Time, dedupeMax)
	for i := 0; i < dedupeMax-1; i++ {
		c.seen[fmt.Sprintf("alt-%d", i)] = time.Now().Add(-dedupeWindow - time.Minute)
	}
	c.mu.Unlock()

	// Loest das Aufraeumen aus und muss danach selbst noch gemerkt sein.
	c.handleNotification("msg-1", notification("frisch", "sub-1"))

	calls := 0
	c.SetOnRedemption(func(rewardID, redemptionID, userName, rewardTitle string) { calls++ })
	c.handleNotification("msg-2", notification("frisch", "sub-1"))

	if calls != 0 {
		t.Fatal("frischer Eintrag beim Aufraeumen verworfen, Doppelung kam durch")
	}
}
