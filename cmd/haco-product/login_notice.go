package main

import (
	"fmt"
	"golang.org/x/term"
	"os"
	"strings"
)

const (
	trustedHostNoticeEnglish  = "Entering trusted haco-host. Host authority is available here; use an Environment for ordinary development work."
	trustedHostNoticeJapanese = "信頼済みの haco-host に入ります。ここでは Host 権限を利用できます。通常の開発作業には Environment を使用してください。"
)

func writeTrustedHostNotice(out *os.File) {
	message := trustedHostNotice()
	if term.IsTerminal(int(out.Fd())) && os.Getenv("NO_COLOR") == "" {
		message = "\x1b[33m" + message + "\x1b[0m"
	}
	fmt.Fprintln(out, message)
}
func trustedHostNotice() string {
	locale := ""
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			locale = strings.ToLower(value)
			break
		}
	}
	if separator := strings.IndexAny(locale, ".@"); separator >= 0 {
		locale = locale[:separator]
	}
	if locale == "ja" || strings.HasPrefix(locale, "ja_") || strings.HasPrefix(locale, "ja-") {
		return trustedHostNoticeJapanese
	}
	return trustedHostNoticeEnglish
}
