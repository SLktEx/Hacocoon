package cliui

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestLocaleForms(t *testing.T) {
	for _, value := range []string{"ja", "ja_JP", "ja_JP.UTF-8", "ja_JP.utf8", "ja_JP.eucJP", "ja_JP.UTF-8@calendar", "ja@calendar", "ja-JP", "JA-jp"} {
		if got := ParseLocale(value); got != Japanese {
			t.Errorf("ParseLocale(%q)=%q, want ja", value, got)
		}
	}
	for _, value := range []string{"", "en", "en_US.UTF-8", "C", "C.UTF-8", "POSIX", "fr_FR.UTF-8", "java", "japanese", "ja_", "ja-JP-extra", "ja_foo", "ja_JP.", "ja_JP@", "ja_JP.UTF-8;echo", "ja_JP\n", " ja", " ", "ja\x00", "ja/JP"} {
		if got := ParseLocale(value); got != English {
			t.Errorf("ParseLocale(%q)=%q, want en", value, got)
		}
	}
}

func TestLocalePrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, all, messages, lang string
		want                      Language
	}{
		{"LANG", "", "", "ja_JP.UTF-8", Japanese},
		{"LC_MESSAGES", "", "ja_JP.UTF-8", "en_US.UTF-8", Japanese},
		{"LC_ALL", "ja", "C", "C", Japanese},
		{"C override", "C", "ja_JP.UTF-8", "ja_JP.UTF-8", English},
		{"unsupported override", "de_DE.UTF-8", "ja", "ja", English},
		{"malformed override", "java", "ja", "ja", English},
		{"whitespace is not empty", " ", "ja", "ja", English},
		{"unsupported messages", "", "fr_FR.UTF-8", "ja", English},
		{"empty settings", "", "", "", English},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{"LC_ALL": tc.all, "LC_MESSAGES": tc.messages, "LANG": tc.lang}
			if got := Resolve(func(key string) string { return env[key] }); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
	if Resolve(nil) != English {
		t.Fatal("nil environment must use English")
	}
}

func TestResolverStopsAtFirstNonemptyValue(t *testing.T) {
	var keys []string
	got := Resolve(func(key string) string {
		keys = append(keys, key)
		if key == "LC_ALL" {
			return "not-a-locale"
		}
		return "ja"
	})
	if got != English || !reflect.DeepEqual(keys, []string{"LC_ALL"}) {
		t.Fatalf("language=%q reads=%v", got, keys)
	}
}

func TestCatalogCompletenessAndPlaceholders(t *testing.T) {
	// Catalog templates deliberately use ordinary, ordered fmt directives.
	verbs := regexp.MustCompile(`%[-+# 0]*[0-9]*(?:\.[0-9]+)?[a-zA-Z%]`)
	for id, entry := range catalog {
		t.Run(id, func(t *testing.T) {
			if id == "" || entry.en == "" || entry.ja == "" {
				t.Fatal("message ID and both translations are required")
			}
			en, ja := verbs.FindAllString(entry.en, -1), verbs.FindAllString(entry.ja, -1)
			if !reflect.DeepEqual(en, ja) {
				t.Fatalf("placeholder mismatch: en=%v ja=%v", en, ja)
			}
			var args []any
			for _, verb := range en {
				switch verb[len(verb)-1] {
				case '%':
				case 'd':
					args = append(args, 42)
				default:
					args = append(args, "dev-UNCHANGED")
				}
			}
			for _, language := range []Language{English, Japanese, "unsupported", ""} {
				if got := language.Format(id, args...); strings.Contains(got, "%!") {
					t.Fatalf("invalid formatted message: %q", got)
				}
			}
		})
	}
}

func TestSafeFallback(t *testing.T) {
	if got := selectText(Japanese, translation{en: "English fallback"}); got != "English fallback" {
		t.Fatalf("missing Japanese translation: %q", got)
	}
	if got := Language("fr").Text("help.usage"); got != "Usage:" {
		t.Fatalf("unknown language: %q", got)
	}
	if got := Japanese.Text("unknown.message"); got != "unknown.message" {
		t.Fatalf("missing ID: %q", got)
	}
}

func TestLanguageValuesAreConcurrentAndIndependent(t *testing.T) {
	for i := 0; i < 32; i++ {
		language := English
		if i%2 != 0 {
			language = Japanese
		}
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			want := catalog["approval.approved"].en
			if language == Japanese {
				want = catalog["approval.approved"].ja
			}
			for j := 0; j < 200; j++ {
				if got := language.Text("approval.approved"); got != want {
					t.Fatalf("language leak: %q", got)
				}
			}
		})
	}
}

func TestFormattingPreservesAndQuotesArguments(t *testing.T) {
	name := "dev-%s-\x1b[31m-日本語"
	for _, language := range []Language{English, Japanese} {
		got := language.Format("error.unknown_command", name)
		if !strings.Contains(got, strconv.Quote(name)) || strings.Contains(got, "\x1b") {
			t.Fatalf("argument was changed or unescaped: %q", got)
		}
	}
}

func TestLiteralMessageIDsUsedByAdaptersExist(t *testing.T) {
	// Check the maintained callers without importing runtime/provider packages.
	for _, directory := range []string{"../../cmd/haco-product", "../capability"} {
		entries, err := os.ReadDir(directory)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(directory, entry.Name())
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}
				matched := false
				switch function := call.Fun.(type) {
				case *ast.Ident:
					matched = function.Name == "cliMessage"
				case *ast.SelectorExpr:
					if function.Sel.Name == "Text" {
						switch receiver := function.X.(type) {
						case *ast.Ident:
							matched = receiver.Name == "language"
						case *ast.SelectorExpr:
							matched = receiver.Sel.Name == "language"
						}
					}
				}
				literal, ok := call.Args[0].(*ast.BasicLit)
				if !matched || !ok || literal.Kind != token.STRING {
					return true
				}
				id, err := strconv.Unquote(literal.Value)
				if err != nil {
					t.Fatal(err)
				}
				if _, ok := catalog[id]; !ok {
					t.Errorf("%s: missing message %q", path, id)
				}
				return true
			})
		}
	}
}

func FuzzParseLocale(f *testing.F) {
	for _, seed := range []string{"ja", "JA-jp", "ja_JP.UTF-8", "C", "POSIX", "", "ja_;echo", "ja\x00"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		language := ParseLocale(value)
		if language != English && language != Japanese {
			t.Fatalf("invalid language: %q", language)
		}
		got := Resolve(func(key string) string {
			if key == "LC_ALL" {
				return value
			}
			return "C"
		})
		if got != language {
			t.Fatalf("resolution mismatch: %q != %q", got, language)
		}
	})
}
