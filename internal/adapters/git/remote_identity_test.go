package gitadapter

import "testing"

func TestRepositoryTransportIdentity(t *testing.T) {
	for _, raw := range []string{"https://github.com/Foo/Bar.git", "https://github.com/foo/bar", "git@github.com:foo/bar.git", "ssh://git@github.com/foo/bar.git", "https://GitHub.com/Foo/Bar.git/"} {
		got, err := CanonicalRemote(raw)
		if err != nil || got != "https://github.com/foo/bar" {
			t.Fatal(raw, got, err)
		}
	}
}
func TestRepositoryIdentityRejectsCredentialsAndForeignHosts(t *testing.T) {
	for _, raw := range []string{"https://token@github.com/foo/bar", "ssh://git:secret@github.com/foo/bar", "git@evil:foo/bar.git", "https://github.com/foo/bar?token=x", "--upload-pack=bad"} {
		if _, err := CanonicalRemote(raw); err == nil {
			t.Fatal(raw)
		}
	}
}
