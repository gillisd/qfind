// Package mounts maps devices to file system types from
// /proc/self/mountinfo (gnulib mountlist.c read_file_system_list).
package mounts

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sys/unix"
)

var (
	once     sync.Once
	readable bool              // read_file_system_list succeeded
	types    map[uint64]string // dev → type; later lines win (fstype.c keeps the last match)
	names    map[string]bool
)

func load() {
	types, names = map[uint64]string{}, map[string]bool{}
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return
	}
	defer f.Close()
	readable = true
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		sep := -1
		for i, fld := range fields {
			if fld == "-" {
				sep = i
				break
			}
		}
		if len(fields) < 5 || sep < 0 || sep+1 >= len(fields) {
			continue
		}
		majmin := strings.SplitN(fields[2], ":", 2)
		if len(majmin) != 2 {
			continue
		}
		maj, _ := strconv.ParseUint(majmin[0], 10, 32)
		min, _ := strconv.ParseUint(majmin[1], 10, 32)
		t := unescape(fields[sep+1])
		if t == "ignore" { // MNTTYPE_IGNORE entries are skipped by fstype.c
			continue
		}
		types[unix.Mkdev(uint32(maj), uint32(min))] = t
		names[t] = true
	}
}

// FSType is fstype.c filesystem_type: the type of the last mount entry on
// dev, or "unknown". ok is false when the mount list cannot be read, which
// GNU treats as fatal ("Cannot read mounted file system list").
func FSType(dev uint64) (fstype string, ok bool) {
	once.Do(load)
	if !readable {
		return "", false
	}
	if t, found := types[dev]; found {
		return t, true
	}
	return "unknown", true
}

// HasType is parser.c is_used_fs_type (for -fstype at -O2 and above):
// "afs" and an unreadable mount list both count as in use.
func HasType(name string) bool {
	once.Do(load)
	return name == "afs" || !readable || names[name]
}

// SELinuxEnabled is libselinux is_selinux_enabled() > 0 as find sees it:
// selinuxfs is mounted and /etc/selinux/config exists (has_selinux_config).
func SELinuxEnabled() bool {
	once.Do(load)
	_, err := os.Stat("/etc/selinux/config")
	return names["selinuxfs"] && err == nil
}

// unescape undoes mountinfo's \ooo escapes.
func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			if n, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
