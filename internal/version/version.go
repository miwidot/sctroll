// Package version haelt die Programmversion an einer Stelle.
package version

import (
	"strconv"
	"strings"
)

// Current ist die Version dieses Builds. Muss zur productVersion in wails.json
// und zum Release-Tag auf GitHub passen -- ein Test wacht darueber.
const Current = "1.0.13"

// Repo ist das GitHub-Repository, in dem nach neuen Versionen gesucht wird.
const Repo = "miwidot/sctroll"

// Compare vergleicht zwei Versionen und liefert -1, 0 oder 1.
// Ein fuehrendes "v" ist erlaubt, fehlende Stellen zaehlen als 0.
func Compare(a, b string) int {
	pa, pb := parse(a), parse(b)
	for i := 0; i < 3; i++ {
		switch {
		case pa[i] < pb[i]:
			return -1
		case pa[i] > pb[i]:
			return 1
		}
	}
	return 0
}

// IsNewer meldet, ob other neuer ist als die laufende Version.
func IsNewer(other string) bool {
	return Compare(other, Current) > 0
}

func parse(v string) [3]int {
	v = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(v), "v"))
	// Vorabversionen und Build-Angaben abschneiden: "1.2.3-beta1+abc" -> "1.2.3"
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}

	var out [3]int
	for i, part := range strings.SplitN(v, ".", 3) {
		if i > 2 {
			break
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return out
		}
		out[i] = n
	}
	return out
}
