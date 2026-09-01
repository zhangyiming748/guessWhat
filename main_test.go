package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBuildURLCoversAllCombinations(t *testing.T) {
	template := "http://x.com/a*b*"
	positions := []int{14, 16}
	total := len(charset) * len(charset)

	seen := make(map[string]bool, total)
	for idx := uint64(0); idx < uint64(total); idx++ {
		u := buildURL(template, positions, idx)
		if len(u) != len(template) {
			t.Fatalf("长度变化: %q", u)
		}
		if seen[u] {
			t.Fatalf("组合重复: %q", u)
		}
		seen[u] = true
	}
	if len(seen) != total {
		t.Fatalf("组合数 %d, 期望 %d", len(seen), total)
	}
}

func TestIsCorrect(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"base64", "aGVsbG8gd29ybGQ=", true},
		{"base64 raw", "aGVsbG8gd29ybGQ", true},
		{"error json", `{"error": "not found"}`, false},
		{"json without error", `{"data": "SGVsbG8="}`, false},
		{"plain text", "not a valid response!", false},
		{"empty", "", false},
		{"too short", "YQ==", false}, // 低于默认 min-len 8，避免误判
	}
	for _, c := range cases {
		if got := isCorrect([]byte(c.body), 8); got != c.want {
			t.Errorf("%s: isCorrect(%q) = %v, 期望 %v", c.name, c.body, got, c.want)
		}
	}
}

func TestFetchEndToEnd(t *testing.T) {
	const secret = "c2VjcmV0LXRva2Vu" // 正确 URL 返回的 Base64 串
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/K7x" {
			fmt.Fprint(w, secret)
			return
		}
		fmt.Fprint(w, `{"error":"not found"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	ctx := context.Background()

	if ok, _ := fetch(ctx, client, srv.URL+"/K7x", 8); !ok {
		t.Error("正确 URL 未命中")
	}
	if ok, _ := fetch(ctx, client, srv.URL+"/aaa", 8); ok {
		t.Error("错误 URL 被误判为命中")
	}
}
