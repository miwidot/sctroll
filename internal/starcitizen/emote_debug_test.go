package starcitizen

import "testing"

// Emotes haben in Star Citizen keine Standardbelegung -- wer sie benutzt, hat
// sie selbst gebunden. Genau diese Belegungen muessen aus dem Profil ankommen,
// sonst drueckt sctroll bei jedem Emote ins Leere.
func TestEmoteOverridesAreRead(t *testing.T) {
	path := realActionMaps(t)
	am, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	binds := am.AllKeyboardBinds()

	emotes := 0
	for k := range binds {
		if _, name, ok := splitBindKey(k); ok && name != "" {
			if _, isEmote := binds["player_emotes|"+name]; isEmote {
				emotes++
			}
		}
	}
	if emotes == 0 {
		t.Skip("Profil enthält keine eigenen Emote-Belegungen")
	}

	for _, name := range []string{"emote_dance", "emote_wave", "emote_salute"} {
		key := "player_emotes|" + name
		got := binds[key]
		direct := am.KeyboardBind("player_emotes", name)
		if got != direct {
			t.Errorf("%s: AllKeyboardBinds sagt %q, KeyboardBind sagt %q", key, got, direct)
		}
		if got != "" && DefaultBinds()[key].Key != "" {
			t.Errorf("%s: hat wider Erwarten eine Standardbelegung (%q)", key, DefaultBinds()[key].Key)
		}
		t.Logf("%-28s Profil=%q", key, got)
	}
}

func splitBindKey(k string) (actionmap, action string, ok bool) {
	for i := range k {
		if k[i] == '|' {
			return k[:i], k[i+1:], true
		}
	}
	return "", "", false
}
