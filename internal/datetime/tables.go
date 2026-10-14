package datetime

// The lexical tables of gnulib parse-datetime.y (findutils 4.11.0).

// table is parse-datetime.y's table: a word and the token it lexes as.
type table struct {
	name  string
	typ   int // token: tMONTH, tDAY, tZONE, … or a character such as 'J'
	value int64
}

// Meridian: am, pm, or 24-hour style (enum { MERam, MERpm, MER24 }).
const (
	merAM = iota
	merPM
	mer24
)

// hour is the HOUR macro: h hours in seconds.
func hour(h int64) int64 { return h * 60 * 60 }

var meridianTable = []table{{"AM", tMERIDIAN, merAM}, {"A.M.", tMERIDIAN, merAM},
	{"PM", tMERIDIAN, merPM}, {"P.M.", tMERIDIAN, merPM}}

var dstTable = table{"DST", tDST, 0}

var monthAndDayTable = []table{
	{"JANUARY", tMONTH, 1}, {"FEBRUARY", tMONTH, 2}, {"MARCH", tMONTH, 3}, {"APRIL", tMONTH, 4},
	{"MAY", tMONTH, 5}, {"JUNE", tMONTH, 6}, {"JULY", tMONTH, 7}, {"AUGUST", tMONTH, 8},
	{"SEPTEMBER", tMONTH, 9}, {"SEPT", tMONTH, 9}, {"OCTOBER", tMONTH, 10}, {"NOVEMBER", tMONTH, 11},
	{"DECEMBER", tMONTH, 12}, {"SUNDAY", tDAY, 0}, {"MONDAY", tDAY, 1}, {"TUESDAY", tDAY, 2},
	{"TUES", tDAY, 2}, {"WEDNESDAY", tDAY, 3}, {"WEDNES", tDAY, 3}, {"THURSDAY", tDAY, 4},
	{"THUR", tDAY, 4}, {"THURS", tDAY, 4}, {"FRIDAY", tDAY, 5}, {"SATURDAY", tDAY, 6},
}

var timeUnitsTable = []table{
	{"YEAR", tYEAR_UNIT, 1}, {"MONTH", tMONTH_UNIT, 1}, {"FORTNIGHT", tDAY_UNIT, 14},
	{"WEEK", tDAY_UNIT, 7}, {"DAY", tDAY_UNIT, 1}, {"HOUR", tHOUR_UNIT, 1},
	{"MINUTE", tMINUTE_UNIT, 1}, {"MIN", tMINUTE_UNIT, 1}, {"SECOND", tSEC_UNIT, 1},
	{"SEC", tSEC_UNIT, 1},
}

var relativeTimeTable = []table{
	{"TOMORROW", tDAY_SHIFT, 1}, {"YESTERDAY", tDAY_SHIFT, -1}, {"TODAY", tDAY_SHIFT, 0},
	{"NOW", tDAY_SHIFT, 0}, {"LAST", tORDINAL, -1}, {"THIS", tORDINAL, 0}, {"NEXT", tORDINAL, 1},
	{"FIRST", tORDINAL, 1}, {"THIRD", tORDINAL, 3}, {"FOURTH", tORDINAL, 4},
	{"FIFTH", tORDINAL, 5}, {"SIXTH", tORDINAL, 6}, {"SEVENTH", tORDINAL, 7},
	{"EIGHTH", tORDINAL, 8}, {"NINTH", tORDINAL, 9}, {"TENTH", tORDINAL, 10},
	{"ELEVENTH", tORDINAL, 11}, {"TWELFTH", tORDINAL, 12}, {"AGO", tAGO, -1}, {"HENCE", tAGO, 1},
}

var universalTimeZoneTable = []table{{"GMT", tZONE, 0}, {"UT", tZONE, 0}, {"UTC", tZONE, 0}}

var timeZoneTable = []table{
	{"WET", tZONE, 0}, {"WEST", tDAYZONE, 0}, {"BST", tDAYZONE, 0},
	{"ART", tZONE, -hour(3)}, {"BRT", tZONE, -hour(3)}, {"BRST", tDAYZONE, -hour(3)},
	{"NST", tZONE, -(hour(3) + 30*60)}, {"NDT", tDAYZONE, -(hour(3) + 30*60)},
	{"AST", tZONE, -hour(4)}, {"ADT", tDAYZONE, -hour(4)}, {"CLT", tZONE, -hour(4)},
	{"CLST", tDAYZONE, -hour(4)}, {"EST", tZONE, -hour(5)}, {"EDT", tDAYZONE, -hour(5)},
	{"CST", tZONE, -hour(6)}, {"CDT", tDAYZONE, -hour(6)}, {"MST", tZONE, -hour(7)},
	{"MDT", tDAYZONE, -hour(7)}, {"PST", tZONE, -hour(8)}, {"PDT", tDAYZONE, -hour(8)},
	{"AKST", tZONE, -hour(9)}, {"AKDT", tDAYZONE, -hour(9)}, {"HST", tZONE, -hour(10)},
	{"HAST", tZONE, -hour(10)}, {"HADT", tDAYZONE, -hour(10)}, {"SST", tZONE, -hour(12)},
	{"WAT", tZONE, hour(1)}, {"CET", tZONE, hour(1)}, {"CEST", tDAYZONE, hour(1)},
	{"MET", tZONE, hour(1)}, {"MEZ", tZONE, hour(1)}, {"MEST", tDAYZONE, hour(1)},
	{"MESZ", tDAYZONE, hour(1)}, {"EET", tZONE, hour(2)}, {"EEST", tDAYZONE, hour(2)},
	{"CAT", tZONE, hour(2)}, {"SAST", tZONE, hour(2)}, {"EAT", tZONE, hour(3)},
	{"MSK", tZONE, hour(3)}, {"MSD", tDAYZONE, hour(3)}, {"IST", tZONE, hour(5) + 30*60},
	{"SGT", tZONE, hour(8)}, {"KST", tZONE, hour(9)}, {"JST", tZONE, hour(9)},
	{"GST", tZONE, hour(10)}, {"NZST", tZONE, hour(12)}, {"NZDT", tDAYZONE, hour(12)},
}

var militaryTable = []table{
	{"A", tZONE, hour(1)}, {"B", tZONE, hour(2)}, {"C", tZONE, hour(3)}, {"D", tZONE, hour(4)},
	{"E", tZONE, hour(5)}, {"F", tZONE, hour(6)}, {"G", tZONE, hour(7)}, {"H", tZONE, hour(8)},
	{"I", tZONE, hour(9)}, {"J", 'J', 0}, {"K", tZONE, hour(10)}, {"L", tZONE, hour(11)},
	{"M", tZONE, hour(12)}, {"N", tZONE, -hour(1)}, {"O", tZONE, -hour(2)}, {"P", tZONE, -hour(3)},
	{"Q", tZONE, -hour(4)}, {"R", tZONE, -hour(5)}, {"S", tZONE, -hour(6)}, {"T", 'T', 0},
	{"U", tZONE, -hour(8)}, {"V", tZONE, -hour(9)}, {"W", tZONE, -hour(10)}, {"X", tZONE, -hour(11)},
	{"Y", tZONE, -hour(12)}, {"Z", tZONE, 0},
}
