package find

import (
	"math"

	"qfind/internal/idcache"
	"qfind/internal/libc"
)

// parseUser is parser.c parse_user: a name known to getpwnam, else a
// decimal UID no larger than UID_T_MAX.
func parseUser(ps *parser, e *parserEntry) bool {
	name, ok := ps.collectArg()
	if !ok {
		return false
	}
	uid, found := idcache.LookupUser(name)
	if !found {
		v, ok := libc.Xstrtoumax(name)
		if !ok || v > math.MaxUint32 {
			ps.r.fatalf(nil, "invalid user name or UID argument to -user: %s", ps.r.quote(name))
		}
		uid = uint32(v)
	}
	p := ps.insertPrimary(e, name, true)
	p.args.uid = uid
	p.estSuccessRate = 0.2
	if uid < 100 {
		p.estSuccessRate = 0.99
	}
	return true
}

// parseGroup is parser.c parse_group.
func parseGroup(ps *parser, e *parserEntry) bool {
	name, ok := ps.collectArg()
	if !ok {
		return false
	}
	gid, found := idcache.LookupGroup(name)
	if !found {
		v, ok := libc.Xstrtoumax(name)
		if !ok || v > math.MaxUint32 {
			ps.r.fatalf(nil, "invalid group name or GID argument to -group: %s", ps.r.quote(name))
		}
		gid = uint32(v)
	}
	p := ps.insertPrimary(e, name, true)
	p.args.gid = gid
	p.estSuccessRate = 0.2
	if gid < 100 {
		p.estSuccessRate = 0.99
	}
	return true
}

func parseNouser(ps *parser, e *parserEntry) bool {
	p := ps.insertPrimaryNoArg(e)
	p.estSuccessRate = 1e-3
	return true
}

func parseNogroup(ps *parser, e *parserEntry) bool {
	p := ps.insertPrimaryNoArg(e)
	p.estSuccessRate = 1e-4
	return true
}

func predUser(r *Run, e *Entry, p *Predicate) bool  { return e.st.Uid == p.args.uid }
func predGroup(r *Run, e *Entry, p *Predicate) bool { return e.st.Gid == p.args.gid }

// predNouser and predNogroup are getpwuid/getgrgid == NULL, cached.
func predNouser(r *Run, e *Entry, p *Predicate) bool  { return !idcache.UserExists(e.st.Uid) }
func predNogroup(r *Run, e *Entry, p *Predicate) bool { return !idcache.GroupExists(e.st.Gid) }
