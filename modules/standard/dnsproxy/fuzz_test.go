package dnsproxy

import (
	"testing"

	"golang.org/x/net/dns/dnsmessage"
)

func FuzzDNSQuestionAndFailure(f *testing.F) {
	f.Add([]byte{})
	name, _ := dnsmessage.NewName("example.com.")
	seed, _ := (&dnsmessage.Message{Questions: []dnsmessage.Question{{Name: name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET}}}).Pack()
	f.Add(seed)
	f.Add([]byte{0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 192, 12, 0, 1, 0, 1})
	f.Fuzz(func(t *testing.T, input []byte) {
		query, err := question(input)
		result := failure(input)
		if err != nil {
			if result != nil {
				t.Fatal("malformed input produced reply")
			}
			return
		}
		var decoded dnsmessage.Message
		if len(result) > MaxMessageBytes || decoded.Unpack(result) != nil || !decoded.Response || decoded.ID != query.ID || decoded.RCode != dnsmessage.RCodeServerFailure {
			t.Fatal("invalid bounded failure reply")
		}
	})
}
