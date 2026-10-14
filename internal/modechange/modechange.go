// Package modechange is gnulib modechange.c (mode_compile, mode_adjust)
// as find -perm uses it: umask 0, no reference file.
package modechange

const (
	suid, sgid, svtx = 04000, 02000, 01000
	rusr, wusr, xusr = 0400, 0200, 0100
	rgrp, wgrp, xgrp = 040, 020, 010
	roth, woth, xoth = 04, 02, 01
	rwxu, rwxg, rwxo = 0700, 070, 07
	allm             = 07777 // every octal mode bit
	chmodModeBits    = allm  // S_ISUID|S_ISGID|S_ISVTX|S_IRWXUGO
	rwxugo           = 0777
)

type flag uint8

const (
	ordinaryChange flag = iota + 1 // MODE_ORDINARY_CHANGE
	xIfAnyX                        // MODE_X_IF_ANY_X
	copyExisting                   // MODE_COPY_EXISTING
)

type change struct {
	op                         byte // '=', '+' or '-'
	flag                       flag
	affected, value, mentioned uint32
}

// Changes is a compiled mode (struct mode_change[]).
type Changes []change

// Compile is mode_compile: an octal mode, or symbolic
// [ugoa]*([-+=]([rwxXst]*|[ugo]))+|[-+=][0-7]+ clauses joined by ','.
func Compile(s string) (Changes, bool) {
	at := func(i int) byte {
		if i < len(s) {
			return s[i]
		}
		return 0
	}
	if c := at(0); c >= '0' && c < '8' {
		var octal uint32
		i := 0
		for ; at(i) >= '0' && at(i) < '8'; i++ {
			if octal = 8*octal + uint32(at(i)-'0'); octal > allm {
				return nil, false
			}
		}
		if i != len(s) {
			return nil, false
		}
		mentioned := uint32(chmodModeBits)
		if i < 5 {
			mentioned = octal&(suid|sgid) | svtx | rwxugo
		}
		return Changes{{op: '=', flag: ordinaryChange, affected: chmodModeBits, value: octal, mentioned: mentioned}}, true
	}
	var mc Changes
	p := 0
	for {
		var affected uint32
	who:
		for ; ; p++ {
			switch at(p) {
			case 'u':
				affected |= suid | rwxu
			case 'g':
				affected |= sgid | rwxg
			case 'o':
				affected |= svtx | rwxo
			case 'a':
				affected |= chmodModeBits
			case '=', '+', '-':
				break who
			default:
				return nil, false
			}
		}
		for {
			op := at(p)
			p++
			var value, mentioned uint32
			fl := copyExisting
			switch c := at(p); {
			case c >= '0' && c < '8':
				var octal uint32
				for ; at(p) >= '0' && at(p) < '8'; p++ {
					if octal = 8*octal + uint32(at(p)-'0'); octal > allm {
						return nil, false
					}
				}
				if affected != 0 || (at(p) != 0 && at(p) != ',') {
					return nil, false
				}
				affected, mentioned, value, fl = chmodModeBits, chmodModeBits, octal, ordinaryChange
			case c == 'u':
				value = rwxu
				p++
			case c == 'g':
				value = rwxg
				p++
			case c == 'o':
				value = rwxo
				p++
			default:
				fl = ordinaryChange
			perms:
				for ; ; p++ {
					switch at(p) {
					case 'r':
						value |= rusr | rgrp | roth
					case 'w':
						value |= wusr | wgrp | woth
					case 'x':
						value |= xusr | xgrp | xoth
					case 'X':
						fl = xIfAnyX
					case 's':
						value |= suid | sgid
					case 't':
						value |= svtx
					default:
						break perms
					}
				}
			}
			if mentioned == 0 {
				mentioned = value
				if affected != 0 {
					mentioned = affected & value
				}
			}
			mc = append(mc, change{op, fl, affected, value, mentioned})
			if c := at(p); c != '=' && c != '+' && c != '-' {
				break
			}
		}
		if at(p) != ',' {
			break
		}
		p++
	}
	if p != len(s) {
		return nil, false
	}
	return mc, true
}

// Adjust is mode_adjust(oldmode, dir, 0, changes, NULL).
func (mc Changes) Adjust(oldmode uint32, dir bool) uint32 {
	newmode := oldmode & chmodModeBits
	for _, c := range mc {
		var omit uint32
		if dir {
			omit = (suid | sgid) &^ c.mentioned
		}
		value := c.value
		switch c.flag {
		case copyExisting:
			value &= newmode
			var spread uint32
			if value&(rusr|rgrp|roth) != 0 {
				spread |= rusr | rgrp | roth
			}
			if value&(wusr|wgrp|woth) != 0 {
				spread |= wusr | wgrp | woth
			}
			if value&(xusr|xgrp|xoth) != 0 {
				spread |= xusr | xgrp | xoth
			}
			value |= spread
		case xIfAnyX:
			if newmode&(xusr|xgrp|xoth) != 0 || dir {
				value |= xusr | xgrp | xoth
			}
		}
		mask := ^uint32(0) // ~umask_value with umask 0
		if c.affected != 0 {
			mask = c.affected
		}
		value &= mask &^ omit
		switch c.op {
		case '=':
			preserved := omit
			if c.affected != 0 {
				preserved |= ^c.affected
			}
			newmode = newmode&preserved | value
		case '+':
			newmode |= value
		case '-':
			newmode &^= value
		}
	}
	return newmode
}
