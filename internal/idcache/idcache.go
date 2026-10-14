// Package idcache maps user and group IDs to names and back through the C
// library's NSS: with cgo, os/user calls getpwuid_r, getgrgid_r,
// getpwnam_r and getgrnam_r. Hits and misses are cached for the run, as
// gnulib's idcache does, and are safe for concurrent use.
package idcache

import (
	"os/user"
	"strconv"
	"sync"
)

type name struct {
	s  string
	ok bool
}

type id struct {
	n  uint32
	ok bool
}

var users, groups, userIDs, groupIDs sync.Map

func cached[K comparable, V any](m *sync.Map, k K, load func() V) V {
	if v, ok := m.Load(k); ok {
		return v.(V)
	}
	v := load()
	m.Store(k, v)
	return v
}

// UserName is getpwuid(uid)->pw_name.
func UserName(uid uint32) (string, bool) {
	n := cached(&users, uid, func() name {
		if u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10)); err == nil {
			return name{u.Username, true}
		}
		return name{}
	})
	return n.s, n.ok
}

// GroupName is getgrgid(gid)->gr_name.
func GroupName(gid uint32) (string, bool) {
	n := cached(&groups, gid, func() name {
		if g, err := user.LookupGroupId(strconv.FormatUint(uint64(gid), 10)); err == nil {
			return name{g.Name, true}
		}
		return name{}
	})
	return n.s, n.ok
}

// UserExists is getpwuid(uid) != NULL (-nouser).
func UserExists(uid uint32) bool { _, ok := UserName(uid); return ok }

// GroupExists is getgrgid(gid) != NULL (-nogroup).
func GroupExists(gid uint32) bool { _, ok := GroupName(gid); return ok }

// LookupUser is getpwnam(s)->pw_uid; getpwnam("") is NULL.
func LookupUser(s string) (uint32, bool) {
	v := cached(&userIDs, s, func() id {
		if u, err := user.Lookup(s); s != "" && err == nil {
			if n, err := strconv.ParseUint(u.Uid, 10, 32); err == nil {
				return id{uint32(n), true}
			}
		}
		return id{}
	})
	return v.n, v.ok
}

// LookupGroup is getgrnam(s)->gr_gid.
func LookupGroup(s string) (uint32, bool) {
	v := cached(&groupIDs, s, func() id {
		if g, err := user.LookupGroup(s); s != "" && err == nil {
			if n, err := strconv.ParseUint(g.Gid, 10, 32); err == nil {
				return id{uint32(n), true}
			}
		}
		return id{}
	})
	return v.n, v.ok
}
