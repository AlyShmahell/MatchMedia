package clean

import (
	"testing"

	"github.com/alyshmahell/matchmedia/src/mod/config"
)

func TestApplyStripsTags(t *testing.T) {
	cfg := testCfg(t)
	got := Apply(cfg, "Show S02 1080p Dual Audio WEBRip DD+ x265-ZZ")
	if got != "Show S02" {
		t.Fatalf("release name %q", got)
	}
}

func TestApplyKeepsTitleBlocks(t *testing.T) {
	cfg := testCfg(t)
	got := Apply(cfg, "Show [1080p][HEVC x265 10bit][Dual-Audio] [OVA] [Oshi no Ko]")
	if got != "Show [OVA] [Oshi no Ko]" {
		t.Fatalf("blocks %q", got)
	}
}

func TestApplyParensAndHash(t *testing.T) {
	cfg := testCfg(t)
	got := Apply(cfg, "Show (Dual Audio_10bit_1080p_x265) (2019) (Snow White with the Red Hair) Steeplechase (3) [A1B2C3D4]")
	if got != "Show (2019) (Snow White with the Red Hair) Steeplechase (3)" {
		t.Fatalf("parens %q", got)
	}
	got = Apply(cfg, "Show (06642005)")
	if got != "Show" {
		t.Fatalf("decimal checksum %q", got)
	}
}

func TestApplyUnclosedBlocks(t *testing.T) {
	cfg := testCfg(t)
	cases := []struct {
		in, want string
	}{
		{"Show - 07 - Title (1080p x265 Dual Audio", "Show 07 Title"},
		{"Show [1080p", "Show"},
		{"Show (Foo (1080p) more", "Show Foo more"},
	}
	for _, tc := range cases {
		got := Apply(cfg, tc.in)
		if got != tc.want {
			t.Fatalf("%q: got %q", tc.in, got)
		}
	}
}

func testCfg(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
