package find

import "strings"

// rootPaths maps fastwalk's spelling of paths under a start point to fts's.
// fastwalk trims every trailing slash from the root (cleanRootPath) and
// joins with one "/". fts keeps the start point verbatim (FTS_VERBATIM) and
// builds children as NAPPEND(parent) + "/" + name, where NAPPEND drops one
// trailing slash ("dir//" → "dir//x", "/" → "/x", "//" → "//x").
type rootPaths struct {
	arg    string // the start point as given
	fw     string // fastwalk's root path
	fwSep  string // fw with a trailing "/" (fastwalk's child prefix)
	prefix string // fts child prefix
}

func newRootPaths(arg string) rootPaths {
	fw := strings.TrimRight(arg, "/")
	if fw == "" && arg != "" {
		fw = "/"
	}
	fwSep := fw
	if !strings.HasSuffix(fwSep, "/") {
		fwSep += "/"
	}
	return rootPaths{arg: arg, fw: fw, fwSep: fwSep, prefix: strings.TrimSuffix(arg, "/") + "/"}
}

// gnu converts a fastwalk path below the root to fts_path.
func (rp *rootPaths) gnu(fwPath string) string {
	if rp.prefix == rp.fwSep {
		return fwPath
	}
	return rp.prefix + fwPath[len(rp.fwSep):]
}

// parentKey is the fastwalk path of fwPath's directory.
func (rp *rootPaths) parentKey(fwPath string, depth int) string {
	if depth == 1 {
		return rp.fw
	}
	return fwPath[:strings.LastIndexByte(fwPath, '/')]
}
