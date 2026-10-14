package find

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"golang.org/x/sys/unix"

	"qfind/internal/filemode"
	"qfind/internal/idcache"
	"qfind/internal/libc"
)

// lsWidths are listfile.c's static column widths: they only ever grow,
// in output order. cur is replaced, never modified, under mu, which is
// taken inside a stream's lock (commit renders -ls lines there); it is
// also read without mu, to render a line ahead of its commit.
type lsWidths struct {
	mu  sync.Mutex
	cur atomic.Pointer[lsWidthVals]
}

type lsWidthVals struct {
	inode, blocks, nlink, owner, group, size int
	major, minor                             int
}

func newLsWidths() *lsWidths {
	w := &lsWidths{}
	w.cur.Store(&lsWidthVals{inode: 9, blocks: 6, nlink: 3, owner: 8, group: 8, size: 8, major: 3, minor: 3})
	return w
}

func grow(w *int, n int) {
	if n > *w {
		*w = n
	}
}

// listFile is lib/listfile.c list_file for one entry. Everything that
// reads the file system happens now; the line itself is committed in
// output order, because its column widths are listfile.c's statics, shared
// by all entries and grown in output order. The pieces are list_file's
// stdio calls with their failed_at stages, so a failed write dies with
// "Failed to write output (at stage N)" exactly as GNU does.
func (r *Run) listFile(e *Entry, out *Stream) {
	st := &e.st
	obs := uint64(r.opt.OutputBlockSize)
	l := &lsRecord{lw: r.lsw, mode: filemode.String(st.Mode), name: lsQuote(e.path)}
	l.date, l.dateStage = r.lsDate(st.Mtim.Sec)
	l.ino = strconv.FormatUint(st.Ino, 10)
	l.blocks = strconv.FormatUint((uint64(st.Blocks)*512+obs-1)/obs, 10)
	l.nlink = strconv.FormatUint(uint64(st.Nlink), 10)
	l.user, l.haveUser = idcache.UserName(st.Uid)
	if l.haveUser {
		l.userW = r.mbswidth(l.user)
	} else {
		l.user = strconv.FormatUint(uint64(st.Uid), 10)
	}
	l.group, l.haveGroup = idcache.GroupName(st.Gid)
	if l.haveGroup {
		l.groupW = r.mbswidth(l.group)
	} else {
		l.group = strconv.FormatUint(uint64(st.Gid), 10)
	}
	switch st.Mode & unix.S_IFMT {
	case unix.S_IFCHR, unix.S_IFBLK:
		l.isDev = true
		l.major = strconv.FormatUint(uint64(unix.Major(st.Rdev)), 10)
		l.minor = strconv.FormatUint(uint64(unix.Minor(st.Rdev)), 10)
	default:
		l.size = strconv.FormatInt(st.Size, 10)
	}
	var linkErr error
	if st.Mode&unix.S_IFMT == unix.S_IFLNK {
		var target string
		if target, linkErr = readlink(e.sysPath); linkErr == nil {
			l.isLink, l.link = true, lsQuote(target)
		}
	}
	l.newline = linkErr == nil
	// The line is rendered now, outside the stream's lock, with the
	// current widths; at commit it is used as is if no line has changed
	// them since and it changes none (lsRecord.head).
	l.snap = l.lw.cur.Load()
	v := *l.snap
	l.pre = l.render(&v)
	if v != *l.snap {
		l.pre = nil
	}
	if linkErr == nil {
		r.emitRendered(e, out, l.head)
		return
	}
	head := l.head
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
	r.emitRendered(e, out, lsNewline)
	r.closeRecord(e)
}

func lsNewline() []piece { return []piece{{"\n", 1400}} }

// lsRecord is one -ls line's text, formatted ahead of its commit; only the
// padding to the shared column widths waits for it.
type lsRecord struct {
	lw                                          *lsWidths
	snap                                        *lsWidthVals // the widths pre was rendered with
	pre                                         []piece      // nil when rendering changed the widths
	ino, blocks, mode, nlink, user, group       string
	size, major, minor, date, name, link        string
	userW, groupW, dateStage                    int
	haveUser, haveGroup, isDev, isLink, newline bool
}

// head is the line's calls at commit time, in output order: the line
// rendered ahead when the widths are still those it was rendered with,
// else the line rendered now, growing the widths.
func (l *lsRecord) head() []piece {
	l.lw.mu.Lock()
	defer l.lw.mu.Unlock()
	cur := l.lw.cur.Load()
	if l.pre != nil && cur == l.snap {
		return l.pre
	}
	v := *cur
	ps := l.render(&v)
	if v != *cur {
		l.lw.cur.Store(&v)
	}
	return ps
}

// render formats the line with widths w, growing them as listfile.c does.
func (l *lsRecord) render(w *lsWidthVals) []piece {
	var arr [256]byte
	b := lsLine{buf: arr[:0]}
	b.right(&w.inode, l.ino, 100)
	b.add(" ", 150)
	b.right(&w.blocks, l.blocks, 200)
	b.add(" ", 250)
	b.add(l.mode, 275)
	b.add(" ", 275) // strmode's trailing space
	b.right(&w.nlink, l.nlink, 300)
	b.add(" ", 250)
	if l.haveUser {
		grow(&w.owner, l.userW)
		b.left(w.owner-len(l.user), l.user, 400) // "%-*s ": printf pads bytes
		b.add(" ", 400)
	} else {
		b.left(8-len(l.user), l.user, 450) // "%-8d "
		b.add(" ", 450)
		grow(&w.owner, max(8, len(l.user))+1) // the trailing space counts, as in listfile.c
	}
	if l.haveGroup {
		grow(&w.group, l.groupW)
		b.left(w.group-len(l.group), l.group, 500)
		b.add(" ", 500)
	} else {
		b.left(w.group-len(l.group), l.group, 550) // "%-*d"
		grow(&w.group, max(w.group, len(l.group)))
		b.add(" ", 525)
	}
	if l.isDev {
		b.right(&w.major, l.major, 600)
		b.add(", ", 625)
		b.right(&w.minor, l.minor, 650)
	} else {
		b.right(&w.size, l.size, 800)
	}
	b.add(" ", 850)
	b.add(l.date, l.dateStage)
	b.add(" ", l.dateStage)
	b.add(l.name, 1200)
	if l.isLink {
		b.add(" -> ", 1300)
		b.add(l.link, 1350)
	}
	if l.newline {
		b.add("\n", 1400)
	}
	return b.pieces()
}

// lsStderrLinkError is list_file on unbuffered stderr (-fls /dev/stderr)
// when readlink failed: the head of the line, error()'s stdout flush and
// message, then the newline. stdout's lock and then stderrMu are held
// throughout, so neither another entry's record nor its diagnostic lands
// inside this one.
func (r *Run) lsStderrLinkError(e *Entry, head func() []piece, linkErr error) {
	r.commit(e) // e's earlier output precedes the diagnostic, as in errorAt
	o := r.out
	held := e.holds(o)
	if !held {
		o.lock()
	}
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
	if !held {
		o.unlock()
	}
	if err != nil {
		r.fatalAt(e, err, "Failed to write output (at stage %d)", stage)
	}
}

// lsLine builds one -ls line in a single buffer, remembering where each
// stdio call (and so each failed_at stage) begins. Consecutive text of
// the same stage forms one piece.
type lsLine struct {
	buf    []byte
	n      int
	starts [24]int
	stages [24]int
}

func (l *lsLine) mark(stage int) {
	if l.n == 0 || l.stages[l.n-1] != stage {
		l.starts[l.n], l.stages[l.n] = len(l.buf), stage
		l.n++
	}
}

func (l *lsLine) add(s string, stage int) {
	l.mark(stage)
	l.buf = append(l.buf, s...)
}

func (l *lsLine) pad(n int) {
	for ; n > 0; n-- {
		l.buf = append(l.buf, ' ')
	}
}

// right is fprintf("%*s", *width, s), widening the column when s is wider.
func (l *lsLine) right(width *int, s string, stage int) {
	l.mark(stage)
	l.pad(*width - len(s))
	l.buf = append(l.buf, s...)
	grow(width, len(s))
}

// left is "%-*s": s, then padding to n further columns.
func (l *lsLine) left(n int, s string, stage int) {
	l.mark(stage)
	l.buf = append(l.buf, s...)
	l.pad(n)
}

// pieces splits the line into its calls; they share one string.
func (l *lsLine) pieces() []piece {
	s := string(l.buf)
	ps := make([]piece, l.n)
	for i := range ps {
		end := len(s)
		if i+1 < l.n {
			end = l.starts[i+1]
		}
		ps[i] = piece{s[l.starts[i]:end], l.stages[i]}
	}
	return ps
}

// lsLocaltimeLimit bounds the times handed to Go's calendar: far beyond
// any year that fits a C int, and far from int64 overflow inside time.
const lsLocaltimeLimit = 1 << 60

// lsDate is list_file's timestamp column and its failed_at stage:
// strftime of the local time, "%b %e %H:%M" within six months (30-day
// months) before and one hour after find's start, "%b %e  %Y" otherwise;
// when localtime would fail (the year does not fit an int), the seconds.
func (r *Run) lsDate(mt int64) (string, int) {
	// Files of one tree often share a modification second; the last
	// answer is kept (lsDateLast) to skip localtime and strftime.
	if c := r.lsDateLast.Load(); c != nil && c.mt == mt {
		return c.text, c.stage
	}
	text, stage := r.lsDateUncached(mt)
	r.lsDateLast.Store(&lsDateMemo{mt, text, stage})
	return text, stage
}

type lsDateMemo struct {
	mt    int64
	text  string
	stage int
}

func (r *Run) lsDateUncached(mt int64) (string, int) {
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
	i := 0
	for i < len(s) && s[i] > 040 && s[i] < 0177 && s[i] != '\\' && s[i] != '"' {
		i++
	}
	if i == len(s) {
		return s // nothing to quote
	}
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
