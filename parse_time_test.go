package main

import (
    "testing"
    "time"
)

func TestParsePublished(t *testing.T) {
    ts := time.Date(2006, 1, 2, 15, 4, 5, 0, time.FixedZone("UTC-7", -7*3600))
    // Use a subset of layouts we know parsePublished attempts & that we can produce
    layouts := []string{time.RubyDate, time.RFC1123Z, time.RFC3339}
    for _, layout := range layouts {
        s := ts.Format(layout)
        if _, ok := parsePublished(s); !ok {
            t.Fatalf("expected to parse layout %s => %s", layout, s)
        }
    }
    raw := ts.Format(time.RubyDate)
    if _, ok := parsePublished(""); ok {
        t.Fatalf("expected empty string to fail parse")
    }
    if _, ok := parsePublished("not a date"); ok {
        t.Fatalf("expected invalid string to fail parse")
    }
    if _, ok := parsePublished(raw); !ok {
        t.Fatalf("expected raw to parse")
    }
}
