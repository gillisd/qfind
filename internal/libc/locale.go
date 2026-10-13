package libc

import "strings"

// Locale is the outcome of setlocale(LC_ALL, "") for the categories find
// reads: LC_CTYPE (charset, MB_CUR_MAX), LC_MESSAGES (yes/no) and LC_TIME.
type Locale struct {
	CType, Messages, Time string
	UTF8                  bool // LC_CTYPE codeset is UTF-8, so MB_CUR_MAX > 1
}

// LocaleFromEnv applies glibc precedence: LC_ALL, then LC_<CATEGORY>, then
// LANG; an unset or empty value falls through, and the result defaults to "C".
// No findutils translations are installed, so only these names matter.
func LocaleFromEnv(lookup func(string) (string, bool)) Locale {
	pick := func(category string) string {
		for _, name := range []string{"LC_ALL", category, "LANG"} {
			if v, ok := lookup(name); ok && v != "" {
				return v
			}
		}
		return "C"
	}
	l := Locale{CType: pick("LC_CTYPE"), Messages: pick("LC_MESSAGES"), Time: pick("LC_TIME")}
	l.UTF8 = isUTF8(l.CType)
	return l
}

// isUTF8 reports whether a locale name's codeset is UTF-8 ("C.UTF-8",
// "en_US.utf8", "de_DE.UTF-8@euro").
func isUTF8(name string) bool {
	if i := strings.IndexByte(name, '@'); i >= 0 {
		name = name[:i]
	}
	i := strings.IndexByte(name, '.')
	if i < 0 {
		return false
	}
	cs := strings.ToLower(strings.ReplaceAll(name[i+1:], "-", ""))
	return cs == "utf8"
}

func isCLocale(name string) bool {
	return name == "C" || name == "POSIX" || strings.HasPrefix(name, "C.")
}
