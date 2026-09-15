package cache

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestHostConfigurationRejectsAmbiguityAndUnboundedInput(t *testing.T) {
	for _, input := range []string{
		`null`, `[]`, `{}`, // Empty object is checked separately below.
		`{"areas":[],"areas":[]}`,
		`{"areas":[],"AREAS":[]}`,
		`{"areas":[],"unexpected":true}`,
		`{"areas":[]} {"areas":[]}`,
		`{"areas":[{"name":"compiler","name":"other","path":"/root/cache","compatibility":"v1"}]}`,
		`{"areas":[{"name":"compiler","path":"/root/cache","compatibility":"v1","unknown":1}]}`,
		`{"areas":[[[[[[]]]]]]}`,
		strings.Repeat(" ", MaxConfigurationBytes) + `{}`,
	} {
		_, err := DecodeConfiguration([]byte(input))
		if input == `{}` { // An explicitly empty document selects no caches.
			if err != nil {
				t.Fatal(err)
			}
			continue
		}
		if !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("ambiguous or invalid document accepted: %q: %v", input[:min(len(input), 100)], err)
		}
	}
	data, err := json.Marshal(Configuration{Areas: []Area{area()}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeConfiguration(data)
	if err != nil || len(got.Areas) != 1 || got.Areas[0] != area() {
		t.Fatal(got, err)
	}
}
