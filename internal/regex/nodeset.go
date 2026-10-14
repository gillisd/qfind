package regex

import "slices"

// nodeSet is re_node_set: a sorted set of node indexes (insertLast may
// append out of order, as glibc's re_node_set_insert_last does). Sets
// share backing arrays the way glibc's struct copies share elems, so every
// mutation copies first.
type nodeSet []int

func newSet1(e int) nodeSet { return nodeSet{e} }

func newSet2(a, b int) nodeSet {
	switch {
	case a == b:
		return nodeSet{a}
	case a < b:
		return nodeSet{a, b}
	}
	return nodeSet{b, a}
}

func (s nodeSet) clone() nodeSet { return slices.Clone(s) }

// contains is re_node_set_contains: index+1, or 0 when absent.
func (s nodeSet) contains(e int) int {
	if len(s) == 0 {
		return 0
	}
	lo, hi := 0, len(s)-1
	for lo < hi {
		mid := (lo + hi) / 2
		if s[mid] < e {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if s[lo] == e {
		return lo + 1
	}
	return 0
}

func (s nodeSet) has(e int) bool { return s.contains(e) != 0 }

// union is re_node_set_init_union.
func union(a, b nodeSet) nodeSet {
	out := make(nodeSet, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] > b[j]:
			out = append(out, b[j])
			j++
			continue
		case a[i] == b[j]:
			j++
		}
		out = append(out, a[i])
		i++
	}
	out = append(out, a[i:]...)
	return append(out, b[j:]...)
}

// merge is re_node_set_merge: dest |= src.
func (s *nodeSet) merge(src nodeSet) {
	if len(src) == 0 {
		return
	}
	if len(*s) == 0 {
		*s = src.clone()
		return
	}
	*s = union(*s, src)
}

// insert is re_node_set_insert.
func (s *nodeSet) insert(e int) {
	i, found := slices.BinarySearch(*s, e)
	if !found {
		*s = slices.Insert(s.clone(), i, e)
	}
}

// insertLast is re_node_set_insert_last; it is only used on sets the
// caller owns, so it appends in place.
func (s *nodeSet) insertLast(e int) { *s = append(*s, e) }

// removeAt is re_node_set_remove_at.
func (s *nodeSet) removeAt(i int) {
	if i < 0 || i >= len(*s) {
		return
	}
	*s = slices.Delete(s.clone(), i, i+1)
}

// remove is re_node_set_remove.
func (s *nodeSet) remove(e int) { s.removeAt(s.contains(e) - 1) }

// equal is re_node_set_compare.
func (s nodeSet) equal(o nodeSet) bool { return slices.Equal(s, o) }

// addIntersect is re_node_set_add_intersect: dest |= src1 & src2.
func (s *nodeSet) addIntersect(a, b nodeSet) {
	if len(a) == 0 || len(b) == 0 {
		return
	}
	var both nodeSet
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			both = append(both, a[i])
			i++
			j++
		case a[i] < b[j]:
			i++
		default:
			j++
		}
	}
	s.merge(both)
}
