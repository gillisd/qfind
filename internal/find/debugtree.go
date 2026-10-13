package find

import (
	"fmt"
	"strings"
)

// costNames is tree.c cost_table[].
var costNames = [...]string{"Nothing", "InodeNumber", "Type", "StatInfo", "LinkName",
	"AccessInfo", "SyncDiskHit", "EventualExec", "ImmediateExec", "UserInteraction", "Unknown"}

// costName is tree.c cost_name.
func costName(c Cost) string {
	if int(c) < len(costNames) {
		return costNames[c]
	}
	return "unknown"
}

// typeNames and precNames are tree.c type_table[] and prec_table[].
var typeNames = [...]string{"no", "primary", "uni_op", "bi_op", "open_paren  ", "close_paren "}
var precNames = [...]string{"no", "comma", "or", "and", "negate", "max"}

// printPredicate is tree.c print_predicate.
func (r *Run) printPredicate(p *Predicate) {
	if p.hasArgText {
		r.debugf("%s %s", p.name, p.argText)
	} else {
		r.debugf("%s", p.name)
	}
}

// printTree is tree.c print_tree. C's %#.4g keeps trailing zeros exactly
// as Go's does; the float is promoted to double in both.
func (r *Run) printTree(n *Predicate, indent int) {
	if n == nil {
		return
	}
	pad := strings.Repeat("    ", indent)
	var b strings.Builder
	b.WriteString(pad + "pred=[")
	if n.hasArgText {
		b.WriteString(n.name + " " + n.argText)
	} else {
		b.WriteString(n.name)
	}
	fmt.Fprintf(&b, "] type=%s prec=%s", typeNames[n.typ], precNames[n.prec])
	side := "no "
	if n.sideEffects {
		side = ""
	}
	fmt.Fprintf(&b, " cost=%s est_success_rate=%#.4g %sside effects ", costName(n.cost), float64(n.estSuccessRate), side)
	if n.needStat || n.needType || n.needInum {
		b.WriteString("Needs ")
		var needs []string
		if n.needStat {
			needs = append(needs, "stat")
		}
		if n.needInum {
			needs = append(needs, "inode")
		}
		if n.needType {
			needs = append(needs, "type")
		}
		b.WriteString(strings.Join(needs, ","))
	}
	b.WriteString("\n" + pad)
	r.debugf("%s", b.String())
	if n.left == nil && n.right == nil {
		r.debugf("no children.\n")
		return
	}
	if n.left != nil {
		r.debugf("left:\n")
		r.printTree(n.left, indent+1)
	} else {
		r.debugf("no left.\n")
	}
	r.debugf("%s", pad)
	if n.right != nil {
		r.debugf("right:\n")
		r.printTree(n.right, indent+1)
	} else {
		r.debugf("no right.\n")
	}
}

// printList is pred.c print_list (blank_rtrim trims blanks and tabs).
func (r *Run) printList(p *Predicate) {
	var b strings.Builder
	for ; p != nil; p = p.next {
		fmt.Fprintf(&b, "[%s] ", strings.TrimRight(p.name, " \t"))
	}
	r.debugf("%s\n", b.String())
}

// printOptlist is pred.c print_optlist.
func (r *Run) printOptlist(p *Predicate) {
	if p == nil {
		return
	}
	r.printParenthesised(p.left)
	var flags string
	if p.needStat {
		flags += "[call stat] "
	}
	if p.needType {
		flags += "[need type] "
	}
	if p.needInum {
		flags += "[need inum] "
	}
	r.debugf("%s", flags)
	r.printPredicate(p)
	r.debugf(" [est success rate %.4g] ", float64(p.estSuccessRate))
	if r.opt.Debug&DebugSuccessRates != 0 {
		s, v := p.successes.Load(), p.visits.Load()
		r.debugf("[real success rate %d/%d", s, v)
		if v != 0 {
			r.debugf("=%.4g] ", float64(s)/float64(v))
		} else {
			r.debugf("=_] ")
		}
	}
	r.printParenthesised(p.right)
}

// printParenthesised is pred.c print_parenthesised.
func (r *Run) printParenthesised(n *Predicate) {
	if n == nil {
		return
	}
	if (n.is(fnOr) || n.is(fnAnd)) && n.left == nil {
		r.printParenthesised(n.right)
		return
	}
	parens := n.left != nil || n.right != nil
	if parens {
		r.debugf(" ( ")
	}
	r.printOptlist(n)
	if parens {
		r.debugf(" ) ")
	}
}

// showSuccessRates is pred.c show_success_rates.
func (r *Run) showSuccessRates(t *Predicate) {
	if r.opt.Debug&DebugSuccessRates != 0 {
		r.debugf("Predicate success rates after completion:\n")
		r.printOptlist(t)
		r.debugf("\n")
	}
}
