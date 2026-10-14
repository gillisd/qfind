package libc

import (
	"bytes"
	"encoding/binary"
	"os"
	"strings"
	"sync"
)

// Locale is the outcome of setlocale(LC_ALL, "") for the categories find
// reads: LC_CTYPE (charset, MB_CUR_MAX), LC_MESSAGES (yes/no) and LC_TIME.
type Locale struct {
	CType, Messages, Time string
	UTF8                  bool // LC_CTYPE codeset is UTF-8, so MB_CUR_MAX > 1
}

// categories are the LC_* categories setlocale(LC_ALL, "") loads; if any
// one is unavailable the call fails and every category stays "C".
var categories = []string{"LC_CTYPE", "LC_NUMERIC", "LC_TIME", "LC_COLLATE", "LC_MONETARY",
	"LC_MESSAGES", "LC_PAPER", "LC_NAME", "LC_ADDRESS", "LC_TELEPHONE", "LC_MEASUREMENT",
	"LC_IDENTIFICATION"}

// LocaleFromEnv applies glibc precedence: LC_ALL, then LC_<CATEGORY>, then
// LANG; an unset or empty value falls through, and the result defaults to "C".
// No findutils translations are installed, so only these names matter.
// Like setlocale(LC_ALL, "") (glibc locale/setlocale.c), it is all or
// nothing: one category naming an uninstalled locale leaves every one "C".
func LocaleFromEnv(lookup func(string) (string, bool)) Locale {
	pick := func(category string) string {
		for _, name := range []string{"LC_ALL", category, "LANG"} {
			if v, ok := lookup(name); ok && v != "" {
				return v
			}
		}
		return "C"
	}
	for _, category := range categories {
		if !available(pick(category), lookup) {
			return Locale{CType: "C", Messages: "C", Time: "C"}
		}
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

// available reports whether glibc can load a locale name (port of
// locale/findlocale.c _nl_find_locale): the built-in C, POSIX and C.UTF-8,
// an entry of the locale archive, or a directory under $LOCPATH or
// /usr/lib/locale, with the codeset normalised as _nl_normalize_codeset
// does ("UTF-8" is "utf8").
func available(name string, lookup func(string) (string, bool)) bool {
	if isCLocale(name) && (name == "C" || name == "POSIX" || isUTF8(name)) {
		return true
	}
	if strings.Contains(name, "/") {
		return false
	}
	norm := normalizeLocale(name)
	if dir, ok := lookup("LOCPATH"); ok && dir != "" {
		return dirExists(dir+"/"+name) || dirExists(dir+"/"+norm)
	}
	archive := archiveNames()
	return archive[norm] || archive[name] ||
		dirExists("/usr/lib/locale/"+name) || dirExists("/usr/lib/locale/"+norm)
}

// normalizeLocale is _nl_normalize_codeset applied to the codeset part: it
// lower-cases the codeset and drops its punctuation, prefixing "iso" when
// only digits remain ("en_US.UTF-8" is "en_US.utf8").
func normalizeLocale(name string) string {
	dot := strings.IndexByte(name, '.')
	if dot < 0 {
		return name
	}
	end := len(name)
	if at := strings.IndexByte(name[dot:], '@'); at >= 0 {
		end = dot + at
	}
	var cs strings.Builder
	digitsOnly := true
	for _, c := range []byte(name[dot+1 : end]) {
		switch {
		case c >= 'A' && c <= 'Z':
			cs.WriteByte(c + 'a' - 'A')
			digitsOnly = false
		case c >= 'a' && c <= 'z':
			cs.WriteByte(c)
			digitsOnly = false
		case c >= '0' && c <= '9':
			cs.WriteByte(c)
		}
	}
	codeset := cs.String()
	if digitsOnly {
		codeset = "iso" + codeset
	}
	return name[:dot+1] + codeset + name[end:]
}

// archiveNames reads the locale names of /usr/lib/locale/locale-archive
// (locale/locarchive.h): struct locarhead (magic 0xde020109, then the
// namehash offset at byte 8 and its size at byte 16), whose namehashent
// records {hashval, name_offset, locrec_offset} point at NUL-terminated
// names. It uses ReadAt because the archive can be hundreds of megabytes.
var archiveNames = sync.OnceValue(func() map[string]bool {
	names := map[string]bool{}
	f, err := os.Open("/usr/lib/locale/locale-archive")
	if err != nil {
		return names
	}
	defer f.Close()
	var head [56]byte
	if _, err := f.ReadAt(head[:], 0); err != nil || binary.NativeEndian.Uint32(head[0:]) != 0xde020109 {
		return names
	}
	off, size := binary.NativeEndian.Uint32(head[8:]), binary.NativeEndian.Uint32(head[16:])
	table := make([]byte, 12*int(size))
	if _, err := f.ReadAt(table, int64(off)); err != nil {
		return names
	}
	buf := make([]byte, 256)
	for i := 0; i < int(size); i++ {
		nameOff := binary.NativeEndian.Uint32(table[12*i+4:])
		if nameOff == 0 {
			continue
		}
		n, _ := f.ReadAt(buf, int64(nameOff))
		if j := bytes.IndexByte(buf[:n], 0); j > 0 {
			names[string(buf[:j])] = true
		}
	}
	return names
})

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
