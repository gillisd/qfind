package libc

// IsYes is gnulib yesno()'s `0 < rpmatch(line)`: glibc's YESEXPR for
// LC_MESSAGES is "^[yY]" in C/POSIX/C.UTF-8 and "^[+1yY]" in en_US and the
// other installed locales (spec/README.md: with en_US.UTF-8, "1" is yes).
func (l Locale) IsYes(line string) bool {
	if line == "" {
		return false
	}
	switch line[0] {
	case 'y', 'Y':
		return true
	case '+', '1':
		return !isCLocale(l.Messages)
	}
	return false
}
