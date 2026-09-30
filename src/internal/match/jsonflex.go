package match

import (
	"bytes"
	"encoding/json"
	"strconv"
)

func (j *Job) UnmarshalJSON(b []byte) error {
	fixed, err := coerceScalars(b)
	if err != nil {
		return err
	}
	type alias Job
	var out alias
	if err := json.Unmarshal(fixed, &out); err != nil {
		return err
	}
	*j = Job(out)
	return nil
}

func (f *JobFile) UnmarshalJSON(b []byte) error {
	fixed, err := coerceScalars(b)
	if err != nil {
		return err
	}
	type alias JobFile
	var out alias
	if err := json.Unmarshal(fixed, &out); err != nil {
		return err
	}
	*f = JobFile(out)
	return nil
}

func (c *Candidate) UnmarshalJSON(b []byte) error {
	fixed, err := coerceScalars(b)
	if err != nil {
		return err
	}
	type alias Candidate
	var out alias
	if err := json.Unmarshal(fixed, &out); err != nil {
		return err
	}
	*c = Candidate(out)
	return nil
}

func (s *CatalogSeason) UnmarshalJSON(b []byte) error {
	fixed, err := coerceScalars(b)
	if err != nil {
		return err
	}
	type alias CatalogSeason
	var out alias
	if err := json.Unmarshal(fixed, &out); err != nil {
		return err
	}
	*s = CatalogSeason(out)
	return nil
}

func (e *CatalogEpisode) UnmarshalJSON(b []byte) error {
	fixed, err := coerceScalars(b)
	if err != nil {
		return err
	}
	type alias CatalogEpisode
	var out alias
	if err := json.Unmarshal(fixed, &out); err != nil {
		return err
	}
	*e = CatalogEpisode(out)
	return nil
}

func coerceScalars(b []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	coerceValue(v)
	return json.Marshal(v)
}

func coerceValue(v any) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			switch k {
			case "id", "year", "season", "episode", "number":
				if s, ok := scalarString(val); ok {
					t[k] = s
					continue
				}
			case "score":
				if n, ok := scalarFloat(val); ok {
					t[k] = n
					continue
				}
			}
			coerceValue(val)
		}
	case []any:
		for _, item := range t {
			coerceValue(item)
		}
	}
}

func scalarString(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case json.Number:
		return t.String(), true
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10), true
		}
		return strconv.FormatFloat(t, 'f', -1, 64), true
	default:
		return "", false
	}
}

func scalarFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case json.Number:
		n, err := t.Float64()
		return n, err == nil
	case string:
		n, err := strconv.ParseFloat(stringsTrim(t), 64)
		return n, err == nil
	default:
		return 0, false
	}
}

func stringsTrim(s string) string {
	return string(bytes.TrimSpace([]byte(s)))
}
