package panel

import (
	"os"
	"slices"
	"strings"
)

// Locale identifies a supported UI language.
type Locale string

const (
	LocaleZhCN Locale = "zh-CN"
	LocaleEn   Locale = "en"
)

// IsZh reports whether the locale is Simplified Chinese.
func (l Locale) IsZh() bool { return l == LocaleZhCN }

// MatchLocale resolves the best supported locale from a preference list.
func MatchLocale(preferred []string) Locale {
	for _, raw := range preferred {
		tag := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(raw), "_", "-"))
		if tag == "" {
			continue
		}
		parts := strings.Split(tag, "-")
		switch parts[0] {
		case "zh":
			if len(parts) > 1 && (parts[1] == "hans" || parts[1] == "cn" || parts[1] == "sg") {
				return LocaleZhCN
			}
			if !slices.Contains(parts[1:], "hant") &&
				!slices.Contains(parts[1:], "tw") &&
				!slices.Contains(parts[1:], "hk") &&
				!slices.Contains(parts[1:], "mo") {
				return LocaleZhCN
			}
		case "en":
			return LocaleEn
		}
	}
	return LocaleEn
}

func envLocale() Locale {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		v := strings.TrimSpace(os.Getenv(name))
		if v == "" {
			continue
		}
		if i := strings.IndexAny(v, ".@"); i >= 0 {
			v = v[:i]
		}
		if v == "" || strings.EqualFold(v, "C") || strings.EqualFold(v, "POSIX") {
			continue
		}
		return MatchLocale([]string{v})
	}
	return LocaleEn
}
