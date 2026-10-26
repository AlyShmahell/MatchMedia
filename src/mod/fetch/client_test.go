package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLogRedactsAPIKey(t *testing.T) {
	var got []string
	SetRequestLog(func(method, rawURL string) {
		got = append(got, method+" "+rawURL)
	})
	t.Cleanup(func() { SetRequestLog(nil) })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "secret" || r.URL.Query().Get("apikey") != "other" {
			t.Errorf("request query %s", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := &httpClient{hc: srv.Client(), retries: 1}
	raw := srv.URL + "/search/shows?q=Foo&api_key=secret&apikey=other"
	if _, _, err := c.get(context.Background(), raw, nil, ""); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("logged %d: %v", len(got), got)
	}
	line := got[0]
	if !strings.HasPrefix(line, "GET ") {
		t.Fatalf("method: %s", line)
	}
	if !strings.Contains(line, "/search/shows?") || !strings.Contains(line, "q=Foo") {
		t.Fatalf("url: %s", line)
	}
	if strings.Contains(line, "secret") || strings.Contains(line, "other") {
		t.Fatalf("key leaked: %s", line)
	}
}

func TestBytesCache(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path == "/bad" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("img"))
	}))
	t.Cleanup(srv.Close)
	c := &Client{http: &httpClient{hc: srv.Client(), retries: 1, bodies: newBodyCache()}}
	for range 2 {
		b, ctype, err := c.Bytes(context.Background(), srv.URL+"/ok", "")
		if err != nil || string(b) != "img" || ctype != "image/jpeg" {
			t.Fatalf("bytes %q %q %v", b, ctype, err)
		}
	}
	if hits != 1 {
		t.Fatalf("cached hits %d", hits)
	}
	for range 2 {
		if _, _, err := c.Bytes(context.Background(), srv.URL+"/bad", ""); err == nil {
			t.Fatal("expected status error")
		}
	}
	if hits != 3 {
		t.Fatalf("error hits %d", hits)
	}
}
