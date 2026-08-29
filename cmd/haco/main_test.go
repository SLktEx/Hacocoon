package main

import "testing"

func TestParseSize(t *testing.T) {
	cases := map[string]int64{"1G": 1 << 30, "2GiB": 2 << 30, "4096": 4096, "3MB": 3 << 20}
	for input, want := range cases {
		got, err := parseSize(input)
		if err != nil {
			t.Fatalf("%s: %v", input, err)
		}
		if got != want {
			t.Fatalf("%s: got %d want %d", input, got, want)
		}
	}
}
