package fonts

import (
	"strings"
	"testing"
)

func TestCatalog(t *testing.T) {
	want := map[string]string{"wenkai": "LXGW WenKai Lite", "jetbrains": "JetBrains Mono", "fira": "Fira Code"}
	resources := Catalog()
	if len(resources) != len(want) {
		t.Fatalf("count got %d, want %d", len(resources), len(want))
	}
	for _, r := range resources {
		if r.Family != want[r.ID] || r.License != "OFL-1.1" || !validMetadata(r) || !strings.HasPrefix(r.URL, "https://raw.githubusercontent.com/") {
			t.Errorf("invalid resource: %+v", r)
		}
		if strings.Contains(strings.ToLower(r.URL+r.ID+r.Family), "dank") {
			t.Errorf("unexpected Dank remote resource: %+v", r)
		}
		got, ok := Find(r.ID)
		if !ok || got != r {
			t.Errorf("Find(%q) got %+v, %v", r.ID, got, ok)
		}
	}
	resources[0].Family = "changed"
	if Catalog()[0].Family == "changed" {
		t.Fatal("catalog not copied")
	}
	if _, ok := Find("dank"); ok {
		t.Fatal("unexpected Dank resource")
	}
}

func TestLocalKeys(t *testing.T) {
	for _, family := range []string{"Dank Mono", "霞鹜文楷", "JetBrains Mono", strings.Repeat("a", 200)} {
		key := LocalKey(family)
		if key != "local:"+family || LocalFamily(key) != family {
			t.Errorf("round trip failed: %q -> %q", family, key)
		}
	}
	for _, family := range []string{"", " a", "a ", "a,b", "a\nb", "a\x00b", "a\u007fb", "a\u0085b", strings.Repeat("a", 201), strings.Repeat("界", 67), string([]byte{0xff})} {
		if got := LocalKey(family); got != "" {
			t.Errorf("LocalKey(%q) got %q", family, got)
		}
		if got := LocalFamily("local:" + family); got != "" {
			t.Errorf("LocalFamily accepted %q", family)
		}
	}
	for _, key := range []string{"Dank Mono", "local", "LOCAL:Dank Mono"} {
		if got := LocalFamily(key); got != "" {
			t.Errorf("LocalFamily(%q) got %q", key, got)
		}
	}
}
