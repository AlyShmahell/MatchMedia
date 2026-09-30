package match

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJobScalarsKeepJSONTypes(t *testing.T) {
	in := []byte(`{
		"id": 42,
		"source": "scan",
		"title": "Complex Show",
		"year": 2016,
		"season": 4,
		"episode": 1,
		"status": "unmatched",
		"files": [{"path": "a.mkv", "season": 2, "episode": 3}],
		"match": {"provider": "tmdb", "id": 139, "title": "Complex Show", "year": 2016, "score": "0.5"},
		"candidates": [{"provider": "tmdb", "id": 139, "title": "Complex Show", "year": "2016", "score": 0.5}],
		"catalog": [{
			"id": 9,
			"number": 1,
			"title": "Season",
			"year": 2016,
			"episodes": [{"id": 8, "number": 3, "title": "Episode", "year": 2017}]
		}]
	}`)
	var job Job
	if err := json.Unmarshal(in, &job); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(job)
	if err != nil {
		t.Fatal(err)
	}
	body := string(out)
	for _, want := range []string{
		`"id":"42"`,
		`"year":"2016"`,
		`"season":"4"`,
		`"episode":"1"`,
		`"season":"2"`,
		`"episode":"3"`,
		`"id":"139"`,
		`"id":"9"`,
		`"number":"1"`,
		`"id":"8"`,
		`"number":"3"`,
		`"year":"2017"`,
		`"score":0.5`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s in %s", want, body)
		}
	}
	if strings.Contains(body, `"id":42`) || strings.Contains(body, `"year":2016`) || strings.Contains(body, `"score":"0.5"`) {
		t.Fatalf("mixed types: %s", body)
	}
}
