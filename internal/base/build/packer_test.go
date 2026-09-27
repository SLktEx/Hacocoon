package basebuild

import (
	"testing"
)

func validTemplate() *PackerTemplate {
	return &PackerTemplate{Files: []SourceFile{{Path: "base.pkr.hcl", Data: []byte("source")}, {Path: "setup.sh", Data: []byte("true")}}}
}

func TestPackerContextRejectsPathsCollisionsAndBounds(t *testing.T) {
	for _, name := range []string{"/etc/shadow", "../secret", "a/../secret", "a//b", "-option", "C:/secret", "a\\b", ".env", "a/./b"} {
		p := validTemplate()
		p.Files = append(p.Files, SourceFile{Path: name})
		if p.Validate() == nil {
			t.Fatal("unsafe source accepted", name)
		}
	}
	for _, paths := range [][]string{{"setup.sh"}, {"dir", "dir/file"}, {"dir/file", "dir"}} {
		p := validTemplate()
		for _, name := range paths {
			p.Files = append(p.Files, SourceFile{Path: name})
		}
		if p.Validate() == nil {
			t.Fatal("collision accepted", paths)
		}
	}
	p := validTemplate()
	p.Files[0].Data = make([]byte, MaxContextBytes+1)
	if p.Validate() == nil {
		t.Fatal("unbounded source")
	}
}
