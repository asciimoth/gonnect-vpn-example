package clientcore

import (
	"net/http"
	"testing"
)

func TestParseHeaderLines(t *testing.T) {
	headers, err := parseHeaderLines("Accept: text/plain\nX-Test: one\nX-Test: two\n")
	if err != nil {
		t.Fatalf("parseHeaderLines returned error: %v", err)
	}

	if got := headers.Get("Accept"); got != "text/plain" {
		t.Fatalf("unexpected Accept header: %q", got)
	}

	values := headers.Values("X-Test")
	if len(values) != 2 || values[0] != "one" || values[1] != "two" {
		t.Fatalf("unexpected X-Test headers: %#v", values)
	}
}

func TestParseHeaderLinesRejectsInvalidLine(t *testing.T) {
	if _, err := parseHeaderLines("broken-header"); err == nil {
		t.Fatal("expected invalid header error")
	}
}

func TestFormatHTTPResponse(t *testing.T) {
	resp := &http.Response{
		Status: "200 OK",
		Header: http.Header{
			"Content-Type": []string{"text/plain"},
			"X-Test":       []string{"1"},
		},
	}

	got := formatHTTPResponse(resp, []byte("hello"))
	if got == "" {
		t.Fatal("expected formatted response")
	}
	if want := "200 OK\n"; got[:len(want)] != want {
		t.Fatalf("response prefix mismatch: %q", got)
	}
	if want := "hello"; got[len(got)-len(want):] != want {
		t.Fatalf("response body mismatch: %q", got)
	}
}
