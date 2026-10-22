package bytesize

import "testing"

func TestParse(t *testing.T) {
	for raw, want := range map[string]uint64{"65GiB": 65 << 30, "2TiB": 2 << 40, "1B": 1} {
		got, err := Parse(raw, 1<<59)
		if err != nil || got != want {
			t.Fatal(raw, got, err)
		}
	}
	for _, raw := range []string{"0B", "-1GiB", "+1TiB", " 1TiB", "1.5TiB", "unlimited", "18446744073709551615TiB", "3TiB"} {
		if _, err := Parse(raw, 2<<40); err == nil {
			t.Fatal(raw)
		}
	}
}
