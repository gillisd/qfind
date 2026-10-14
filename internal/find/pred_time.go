package find

import "math"

// timespecSub is gnulib timespec_sub: a-b, saturating at the time_t limits.
func timespecSub(a, b Timespec) Timespec {
	nsdiff := a.Nsec - b.Nsec
	var borrow int64
	if nsdiff < 0 {
		borrow = 1
	}
	rs, v1 := subOverflows(a.Sec, b.Sec)
	rs, v2 := subOverflows(rs, borrow)
	if v1 == v2 {
		return Timespec{rs, nsdiff + 1e9*borrow}
	}
	if 0 < rs { // (TYPE_MINIMUM + TYPE_MAXIMUM) / 2 is 0
		return Timespec{math.MinInt64, 0}
	}
	return Timespec{math.MaxInt64, 1e9 - 1}
}

// subOverflows is ckd_sub on time_t: the wrapped difference and overflow.
func subOverflows(a, b int64) (int64, bool) {
	d := a - b
	return d, (b > 0 && d > a) || (b < 0 && d < a)
}

// timeWindow is pred.c pred_timewindow.
func timeWindow(ts Timespec, tv timeVal, window int64) bool {
	switch tv.kind {
	case compGT:
		return ts.cmp(tv.ts) > 0
	case compLT:
		return ts.cmp(tv.ts) < 0
	}
	delta := timespecSub(ts, tv.ts)
	return delta.cmp(Timespec{}) > 0 && delta.cmp(Timespec{window, 0}) <= 0
}

func predAmin(r *Run, e *Entry, p *Predicate) bool {
	return timeWindow(tsOf(e.st.Atim), p.args.reftime, 60)
}

func predAtime(r *Run, e *Entry, p *Predicate) bool {
	return timeWindow(tsOf(e.st.Atim), p.args.reftime, daySecs)
}

func predCmin(r *Run, e *Entry, p *Predicate) bool {
	return timeWindow(tsOf(e.st.Ctim), p.args.reftime, 60)
}

func predCtime(r *Run, e *Entry, p *Predicate) bool {
	return timeWindow(tsOf(e.st.Ctim), p.args.reftime, daySecs)
}

func predMmin(r *Run, e *Entry, p *Predicate) bool {
	return timeWindow(tsOf(e.st.Mtim), p.args.reftime, 60)
}

func predMtime(r *Run, e *Entry, p *Predicate) bool {
	return timeWindow(tsOf(e.st.Mtim), p.args.reftime, daySecs)
}

func predNewer(r *Run, e *Entry, p *Predicate) bool {
	return tsOf(e.st.Mtim).cmp(p.args.reftime.ts) > 0
}

func predAnewer(r *Run, e *Entry, p *Predicate) bool {
	return tsOf(e.st.Atim).cmp(p.args.reftime.ts) > 0
}

func predCnewer(r *Run, e *Entry, p *Predicate) bool {
	return tsOf(e.st.Ctim).cmp(p.args.reftime.ts) > 0
}

// predNewerXY is pred.c pred_newerXY (birth time cannot occur here).
func predNewerXY(r *Run, e *Entry, p *Predicate) bool {
	var ts Timespec
	switch p.args.reftime.xval {
	case xvalAtime:
		ts = tsOf(e.st.Atim)
	case xvalCtime:
		ts = tsOf(e.st.Ctim)
	default:
		ts = tsOf(e.st.Mtim)
	}
	return ts.cmp(p.args.reftime.ts) > 0
}

// predUsed is pred.c pred_used: days from the status change to the last
// access; false when the access is the earlier.
func predUsed(r *Run, e *Entry, p *Predicate) bool {
	at, ct := tsOf(e.st.Atim), tsOf(e.st.Ctim)
	if at.cmp(ct) < 0 {
		return false
	}
	delta := Timespec{ct.Sec - at.Sec, ct.Nsec - at.Nsec}
	if delta.Nsec < 0 {
		delta.Nsec += 1e9
		delta.Sec--
	}
	return timeWindow(delta, p.args.reftime, daySecs)
}
