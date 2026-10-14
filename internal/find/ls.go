package find

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/sys/unix"

	"qfind/internal/filemode"
	"qfind/internal/idcache"
	"qfind/internal/libc"
)

// lsWidths are listfile.c's static column widths: they only ever grow.
// mu is taken inside a stream's lock (commit renders -ls lines there).
type lsWidths struct {
	mu                                       sync.Mutex
	inode, blocks, nlink, owner, group, size int
	major, minor                             int
}

func newLsWidths() *lsWidths {
	return &lsWidths{inode: 9, blocks: 6, nlink: 3, owner: 8, group: 8, size: 8, major: 3, minor: 3}
}

func grow(w *int, n int) {
	if n > *w {
		*w = n
	}
}

// listFile is lib/listfile.c list_file for one entry. Everything that
// reads the file system happens now; the line itself is rendered when the
// entry's output is committed, because its column widths are listfile.c's
// statics, shared by all entries and grown in output order. The pieces are
// list_file's stdio calls with their failed_at stages, so a failed write
// dies with "Failed to write output (at stage N)" exactly as GNU does.
func (r *Run) listFile(e *Entry, out *Stream) {
	st := e.st // copied: the closures run at commit time
	obs := uint64(r.opt.OutputBlockSize)
	blocks := (uint64(st.Blocks)*512 + obs - 1) / obs
	user, haveUser := idcache.UserName(st.Uid)
	group, haveGroup := idcache.GroupName(st.Gid)
	date, dateStage := r.lsDate(st.Mtim.Sec)
	name := lsQuote(e.path)
	isLink := st.Mode&unix.S_IFMT == unix.S_IFLNK
	link, linkErr := "", error(nil)
	if isLink {
		var target string
		if target, linkErr = readlink(e.sysPath); linkErr == nil {
			link = lsQuote(target)
		}
	}
	w := r.lsw
	head := func() []piece {
		w.mu.Lock()
		defer w.mu.Unlock()
		ps := make([]piece, 0, 18)
		add := func(text string, stage int) { ps = append(ps, piece{text, stage}) }
		add(w.right(&w.inode, strconv.FormatUint(st.Ino, 10)), 100)
		add(" ", 150)
		add(w.right(&w.blocks, strconv.FormatUint(blocks, 10)), 200)
		add(" ", 250)
		add(filemode.String(st.Mode)+" ", 275) // strmode's trailing space
		add(w.right(&w.nlink, strconv.FormatUint(uint64(st.Nlink), 10)), 300)
		add(" ", 250)
		if haveUser {
			grow(&w.owner, r.mbswidth(user))
			add(fmt.Sprintf("%-*s ", w.owner, user), 400)
		} else {
			t := fmt.Sprintf("%-8d ", st.Uid)
			grow(&w.owner, len(t)) // the trailing space counts, as in listfile.c
			add(t, 450)
		}
		if haveGroup {
			grow(&w.group, r.mbswidth(group))
			add(fmt.Sprintf("%-*s ", w.group, group), 500)
		} else {
			t := fmt.Sprintf("%-*d", w.group, st.Gid)
			grow(&w.group, len(t))
			add(t, 550)
			add(" ", 525)
		}
		switch st.Mode & unix.S_IFMT {
		case unix.S_IFCHR, unix.S_IFBLK:
			add(w.right(&w.major, strconv.FormatUint(uint64(unix.Major(st.Rdev)), 10)), 600)
			add(", ", 625)
			add(w.right(&w.minor, strconv.FormatUint(uint64(unix.Minor(st.Rdev)), 10)), 650)
		default:
			add(w.right(&w.size, strconv.FormatInt(st.Size, 10)), 800)
		}
		add(" ", 850)
		add(date+" ", dateStage)
		add(name, 1200)
		if isLink && linkErr == nil {
			add(" -> ", 1300)
			add(link, 1350)
		}
		return ps
	}
	tail := func() []piece { return []piece{{"\n", 1400}} }
	if linkErr == nil {
		r.emitRendered(e, out, func() []piece { return append(head(), tail()...) })
		return
	}
	// list_file's error(0, errno, "%s", name) comes between the name and
	// the newline, and flushes stdout first: the record holds its sinks
	// across the diagnostic (record.go). The exit status is unchanged.
	if out.sink == nil {
		r.lsStderrLinkError(e, head, linkErr)
		return
	}
	r.openRecord(e, out)
	r.emitRendered(e, out, head)
	r.errorAt(e, linkErr, "%s", e.path)
	r.emitRendered(e, out, tail)
	r.closeRecord(e)
}

// lsStderrLinkError is list_file on unbuffered stderr (-fls /dev/stderr)
// when readlink failed: the head of the line, error()'s stdout flush and
// message, then the newline. stdout's lock and then stderrMu are held
// throughout, so neither another entry's record nor its diagnostic lands
// inside this one.
func (r *Run) lsStderrLinkError(e *Entry, head func() []piece, linkErr error) {
	r.commit(e) // e's earlier output precedes the diagnostic, as in errorAt
	o := r.out
	o.lock()
	ps := head()
	msg := r.errorText(linkErr, "%s", e.path)
	var b strings.Builder
	for _, pc := range ps {
		b.WriteString(pc.text)
	}
	r.stderrMu.Lock()
	stage := ps[0].stage
	_, err := os.Stderr.WriteString(b.String())
	if err == nil {
		o.f.flushBuf()
		stage = 1400
		_, err = os.Stderr.WriteString(msg + "\n")
	}
	if err != nil {
		r.stderrFailed.Store(true)
	}
	r.stderrMu.Unlock()
	o.unlock()
	if err != nil {
		r.fatalf(err, "Failed to write output (at stage %d)", stage)
	}
}

// right is fprintf("%*s", *width, s), widening the column when s is wider.
func (w *lsWidths) right(width *int, s string) string {
	t := fmt.Sprintf("%*s", *width, s)
	grow(width, len(t))
	return t
}

// lsLocaltimeLimit bounds the times handed to Go's calendar: far beyond
// any year that fits a C int, and far from int64 overflow inside time.
const lsLocaltimeLimit = 1 << 60

// lsDate is list_file's timestamp column and its failed_at stage:
// strftime of the local time, "%b %e %H:%M" within six months (30-day
// months) before and one hour after find's start, "%b %e  %Y" otherwise;
// when localtime would fail (the year does not fit an int), the seconds.
func (r *Run) lsDate(mt int64) (string, int) {
	ok := mt > -lsLocaltimeLimit && mt < lsLocaltimeLimit
	var t time.Time
	if ok {
		t = time.Unix(mt, 0).In(r.tz)
		y := int64(t.Year()) - 1900
		ok = y >= -1<<31 && y < 1<<31
	}
	if !ok {
		if mt < 0 {
			num := strconv.FormatUint(-uint64(mt), 10)
			return fmt.Sprintf("%*s%s", max(12-len(num), 0), "-", num), 1000
		}
		return fmt.Sprintf("%12d", mt), 1100
	}
	now := r.opt.StartTime.Sec
	format := "%b %e  %Y"
	if now-6*30*24*60*60 <= mt && mt <= now+60*60 {
		format = "%b %e %H:%M"
	}
	return libc.Strftime(format, t), 900
}

// mbswidth is gnulib mbswidth(s, 0) for the owner and group columns:
// bytes in a single-byte locale, characters in a UTF-8 one (wide East
// Asian characters count once; user and group names are ASCII in practice).
func (r *Run) mbswidth(s string) int {
	if r.loc.UTF8 {
		return utf8.RuneCountInString(s)
	}
	return len(s)
}

// lsQuote is listfile.c print_name_with_quoting (literal_control_chars is
// always false in find): byte-wise, locale-independent.
func lsQuote(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\b':
			b.WriteString(`\b`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\f':
			b.WriteString(`\f`)
		case ' ':
			b.WriteString(`\ `)
		case '"':
			b.WriteString(`\"`)
		default:
			if c > 040 && c < 0177 {
				b.WriteByte(c)
			} else {
				fmt.Fprintf(&b, "\\%03o", c)
			}
		}
	}
	return b.String()
}

// predLs is pred.c pred_ls and pred_fls.
func predLs(r *Run, e *Entry, p *Predicate) bool {
	r.listFile(e, p.args.out)
	return true
}

// insertFls is parser.c insert_fls: insert_primary_noarg, even for -fls.
func (ps *parser) insertFls(e *parserEntry, name string, hasName bool) bool {
	p := ps.insertPrimaryNoArg(e)
	if hasName {
		p.args.out = ps.openOutputFile(name)
	} else {
		p.args.out = ps.r.out.stdoutStream
	}
	if ps.r.lsw == nil {
		ps.r.lsw = newLsWidths()
	}
	p.sideEffects, p.noDefaultPrint = true, true
	p.estSuccessRate = 1.0
	return true
}

func parseLs(ps *parser, e *parserEntry) bool { return ps.insertFls(e, "", false) }

func parseFls(ps *parser, e *parserEntry) bool {
	name, ok := ps.collectArg()
	if !ok {
		return false
	}
	return ps.insertFls(e, name, true)
}
