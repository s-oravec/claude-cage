package images

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// Pinned SHA-256 of base-aliases.json. Bump in lockstep with the byte-identical
// vendored copy in cage-hub at apps/api/src/config/base-aliases.json
// (sister-PR required). If this test fails, the error message prints the
// actual hash - paste it here.
const expectedBaseAliasesSHA256 = "4a90ab534b7cc6e77e89373122a3ec3c2dab29b6ff812db0b4561d8c84325dc5"

func TestBaseAliasesJSON_PinnedHash(t *testing.T) {
	sum := sha256.Sum256(baseAliasesData)
	got := hex.EncodeToString(sum[:])
	if got != expectedBaseAliasesSHA256 {
		t.Fatalf("base-aliases.json hash drifted\n\twant: %s\n\tgot:  %s\n\tIf this edit is intentional, paste the new hash into expectedBaseAliasesSHA256 AND open a sister-PR in cage-hub to update its vendored copy + EXPECTED_BASE_ALIASES_SHA256.",
			expectedBaseAliasesSHA256, got)
	}
}

func TestBaseAliasesJSON_NonEmpty(t *testing.T) {
	if len(baseAliases) == 0 {
		t.Fatal("baseAliases is empty - JSON failed to parse at init")
	}
}
