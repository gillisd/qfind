// Package filemode is gnulib filemode.c strmode (without the trailing space).
package filemode

import "golang.org/x/sys/unix"

func ftypelet(m uint32) byte {
	switch m & unix.S_IFMT {
	case unix.S_IFREG:
		return '-'
	case unix.S_IFDIR:
		return 'd'
	case unix.S_IFBLK:
		return 'b'
	case unix.S_IFCHR:
		return 'c'
	case unix.S_IFLNK:
		return 'l'
	case unix.S_IFIFO:
		return 'p'
	case unix.S_IFSOCK:
		return 's'
	}
	return '?'
}

// String is the 10-character mode string, e.g. "-rwsr-xr-t".
func String(m uint32) string {
	b := []byte{ftypelet(m), '-', '-', '-', '-', '-', '-', '-', '-', '-'}
	set := func(i int, bit uint32, c byte) {
		if m&bit != 0 {
			b[i] = c
		}
	}
	set(1, 0o400, 'r')
	set(2, 0o200, 'w')
	set(4, 0o040, 'r')
	set(5, 0o020, 'w')
	set(7, 0o004, 'r')
	set(8, 0o002, 'w')
	b[3] = special(m&0o100 != 0, m&unix.S_ISUID != 0, 'x', 's', 'S')
	b[6] = special(m&0o010 != 0, m&unix.S_ISGID != 0, 'x', 's', 'S')
	b[9] = special(m&0o001 != 0, m&unix.S_ISVTX != 0, 'x', 't', 'T')
	return string(b)
}

func special(x, s bool, plain, both, only byte) byte {
	switch {
	case s && x:
		return both
	case s:
		return only
	case x:
		return plain
	}
	return '-'
}
