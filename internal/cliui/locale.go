// Package cliui selects human-facing CLI messages. It does not change process
// environments, localize protocol data, or interpret approval decisions.
package cliui

import (
	"fmt"
	"regexp"
)

// Language is a value owned by a caller, never a process-wide mutable setting.
// The zero value and unsupported values use English.
type Language string

const (
	English  Language = "en"
	Japanese Language = "ja"
)

// Accept normal POSIX Japanese locales and the existing ja-JP spelling. Match
// the whole value: java, ja_, control characters and shell fragments are not ja.
var japaneseLocale = regexp.MustCompile(`(?i)^ja(?:[_-](?:[a-z]{2}|[0-9]{3}))?(?:\.[a-z0-9][a-z0-9_-]*)?(?:@[a-z0-9][a-z0-9_-]*)?$`)

// Resolve selects the first nonempty LC_ALL, LC_MESSAGES or LANG value. An
// unsupported higher-priority value selects English; it must not expose a lower
// priority Japanese setting. No installed OS locale or setlocale call is needed.
func Resolve(getenv func(string) string) Language {
	if getenv == nil {
		return English
	}
	for _, key := range [...]string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := getenv(key); value != "" {
			return ParseLocale(value)
		}
	}
	return English
}

// ParseLocale treats malformed, unknown, C and POSIX locales as English.
func ParseLocale(value string) Language {
	if japaneseLocale.MatchString(value) {
		return Japanese
	}
	return English
}

// Catalogs are initialized once and only read after package initialization.
// Splitting by surface does not introduce a registry or mutable language state.
var messageCatalogs = [...]map[string]translation{catalog, environmentCatalog, commandCatalog}

func lookupMessage(id string) (translation, bool) {
	for _, messages := range messageCatalogs {
		if message, ok := messages[id]; ok {
			return message, true
		}
	}
	return translation{}, false
}

// Text selects a message template. Unknown IDs are returned verbatim so a
// missing catalog entry never panics or changes the outcome of an operation.
func (language Language) Text(id string) string {
	message, ok := lookupMessage(id)
	if !ok {
		return id
	}
	return selectText(language, message)
}

func selectText(language Language, message translation) string {
	if language == Japanese && message.ja != "" {
		return message.ja
	}
	return message.en
}

// Format substitutes values only after selecting the trusted message template.
// Callers remain responsible for their existing escaping/redaction boundaries.
func (language Language) Format(id string, args ...any) string {
	return fmt.Sprintf(language.Text(id), args...)
}
