%{
// Grammar of gnulib parse-datetime.y (findutils 4.11.0). go generate turns
// it into parse.go with goyacc; bison and goyacc both resolve this
// grammar's 31 shift/reduce conflicts by shifting.
package datetime
%}

%union {
	intval  int64
	textint textint
	ts      timespec
	rel     relTime
}

%token <intval> tAGO
%token tDST
%token tYEAR_UNIT tMONTH_UNIT tHOUR_UNIT tMINUTE_UNIT tSEC_UNIT
%token <intval> tDAY_UNIT tDAY_SHIFT
%token <intval> tDAY tDAYZONE tLOCAL_ZONE tMERIDIAN
%token <intval> tMONTH tORDINAL tZONE
%token <textint> tSNUMBER tUNUMBER tUNUMBER_DOTTED
%token <ts> tSDECIMAL_NUMBER tUDECIMAL_NUMBER

%type <intval> o_colon_minutes
%type <ts> seconds signed_seconds unsigned_seconds
%type <rel> relunit relunit_snumber dayshift

%%

spec:
	timespec
|	items
;

timespec:
	'@' seconds
	{ pc := ctl(yylex); pc.seconds = $2; pc.timespecSeen = true }
;

items:
	/* empty */
|	items item
;

item:
	datetime   { pc := ctl(yylex); pc.timesSeen++; pc.datesSeen++ }
|	time       { ctl(yylex).timesSeen++ }
|	local_zone { ctl(yylex).localZonesSeen++ }
|	'J'        { ctl(yylex).jZonesSeen++ }
|	zone       { ctl(yylex).zonesSeen++ }
|	date       { ctl(yylex).datesSeen++ }
|	day        { ctl(yylex).daysSeen++ }
|	rel
|	number
|	hybrid
;

datetime:
	iso_8601_datetime
;

iso_8601_datetime:
	iso_8601_date 'T' iso_8601_time
;

time:
	tUNUMBER tMERIDIAN
	{ pc := ctl(yylex); pc.setHHMMSS($1.value, 0, 0, 0); pc.meridian = int($2) }
|	tUNUMBER ':' tUNUMBER tMERIDIAN
	{ pc := ctl(yylex); pc.setHHMMSS($1.value, $3.value, 0, 0); pc.meridian = int($4) }
|	tUNUMBER ':' tUNUMBER ':' unsigned_seconds tMERIDIAN
	{ pc := ctl(yylex); pc.setHHMMSS($1.value, $3.value, $5.sec, $5.nsec); pc.meridian = int($6) }
|	iso_8601_time
;

iso_8601_time:
	tUNUMBER zone_offset
	{ pc := ctl(yylex); pc.setHHMMSS($1.value, 0, 0, 0); pc.meridian = mer24 }
|	tUNUMBER ':' tUNUMBER o_zone_offset
	{ pc := ctl(yylex); pc.setHHMMSS($1.value, $3.value, 0, 0); pc.meridian = mer24 }
|	tUNUMBER ':' tUNUMBER ':' unsigned_seconds o_zone_offset
	{ pc := ctl(yylex); pc.setHHMMSS($1.value, $3.value, $5.sec, $5.nsec); pc.meridian = mer24 }
;

o_zone_offset:
	/* empty */
|	zone_offset
;

zone_offset:
	tSNUMBER o_colon_minutes
	{ pc := ctl(yylex); pc.zonesSeen++; if !pc.timeZoneHHMM($1, $2) { pc.abort() } }
;

local_zone:
	tLOCAL_ZONE
	{ ctl(yylex).localIsDST = $1 }
|	tLOCAL_ZONE tDST
	{ pc := ctl(yylex); pc.localIsDST = 1; pc.dstsSeen++ }
;

zone:
	tZONE
	{ ctl(yylex).timeZone = $1 }
|	'T'
	{ ctl(yylex).timeZone = -hour(7) }
|	tZONE relunit_snumber
	{ pc := ctl(yylex); pc.timeZone = $1; if !pc.applyRelativeTime($2, 1) { pc.abort() } }
|	'T' relunit_snumber
	{ pc := ctl(yylex); pc.timeZone = -hour(7); if !pc.applyRelativeTime($2, 1) { pc.abort() } }
|	tZONE tSNUMBER o_colon_minutes
	{
		pc := ctl(yylex)
		if !pc.timeZoneHHMM($2, $3) {
			pc.abort()
		} else if z, ovf := ckdAddInt(pc.timeZone, $1); ovf {
			pc.abort()
		} else {
			pc.timeZone = z
		}
	}
|	tDAYZONE
	{ ctl(yylex).timeZone = $1 + 60*60 }
|	tZONE tDST
	{ ctl(yylex).timeZone = $1 + 60*60 }
;

day:
	tDAY
	{ pc := ctl(yylex); pc.dayOrdinal = 0; pc.dayNumber = $1 }
|	tDAY ','
	{ pc := ctl(yylex); pc.dayOrdinal = 0; pc.dayNumber = $1 }
|	tORDINAL tDAY
	{ pc := ctl(yylex); pc.dayOrdinal = $1; pc.dayNumber = $2 }
|	tUNUMBER tDAY
	{ pc := ctl(yylex); pc.dayOrdinal = $1.value; pc.dayNumber = $2 }
;

date:
	tUNUMBER '/' tUNUMBER
	{ pc := ctl(yylex); pc.month = $1.value; pc.day = $3.value }
|	tUNUMBER '/' tUNUMBER '/' tUNUMBER
	{
		pc := ctl(yylex)
		if 4 <= $1.digits { // YYYY/MM/DD
			pc.year, pc.month, pc.day = $1, $3.value, $5.value
		} else { // MM/DD/YY[YY]
			pc.month, pc.day, pc.year = $1.value, $3.value, $5
		}
	}
|	tUNUMBER '.' tUNUMBER '.'
	{ pc := ctl(yylex); pc.day = $1.value; pc.month = $3.value }
|	tUNUMBER '.' tUNUMBER_DOTTED '.' tUNUMBER
	{ pc := ctl(yylex); pc.day = $1.value; pc.month = $3.value; pc.year = $5 }
|	tUNUMBER tMONTH tSNUMBER
	{
		pc := ctl(yylex)
		pc.day, pc.month = $1.value, $2
		if y, ovf := ckdSub(0, $3.value); ovf {
			pc.abort()
		} else {
			pc.year.value, pc.year.digits = y, $3.digits
		}
	}
|	tMONTH tSNUMBER tSNUMBER
	{
		pc := ctl(yylex)
		pc.month = $1
		d, ovf1 := ckdSub(0, $2.value)
		y, ovf2 := ckdSub(0, $3.value)
		if ovf1 || ovf2 {
			pc.abort()
		} else {
			pc.day, pc.year.value, pc.year.digits = d, y, $3.digits
		}
	}
|	tMONTH tUNUMBER
	{ pc := ctl(yylex); pc.month = $1; pc.day = $2.value }
|	tMONTH tUNUMBER ',' tUNUMBER
	{ pc := ctl(yylex); pc.month = $1; pc.day = $2.value; pc.year = $4 }
|	tUNUMBER tMONTH
	{ pc := ctl(yylex); pc.day = $1.value; pc.month = $2 }
|	tUNUMBER tMONTH tUNUMBER
	{ pc := ctl(yylex); pc.day = $1.value; pc.month = $2; pc.year = $3 }
|	iso_8601_date
;

iso_8601_date:
	tUNUMBER tSNUMBER tSNUMBER
	{
		pc := ctl(yylex)
		pc.year = $1
		m, ovf1 := ckdSub(0, $2.value)
		d, ovf2 := ckdSub(0, $3.value)
		if ovf1 || ovf2 {
			pc.abort()
		} else {
			pc.month, pc.day = m, d
		}
	}
;

rel:
	relunit tAGO
	{ pc := ctl(yylex); if !pc.applyRelativeTime($1, $2) { pc.abort() } }
|	relunit
	{ pc := ctl(yylex); if !pc.applyRelativeTime($1, 1) { pc.abort() } }
|	dayshift
	{ pc := ctl(yylex); if !pc.applyRelativeTime($1, 1) { pc.abort() } }
;

relunit:
	tORDINAL tYEAR_UNIT       { $$ = relTime{year: $1} }
|	tUNUMBER tYEAR_UNIT       { $$ = relTime{year: $1.value} }
|	tYEAR_UNIT                { $$ = relTime{year: 1} }
|	tORDINAL tMONTH_UNIT      { $$ = relTime{month: $1} }
|	tUNUMBER tMONTH_UNIT      { $$ = relTime{month: $1.value} }
|	tMONTH_UNIT               { $$ = relTime{month: 1} }
|	tORDINAL tDAY_UNIT        { $$ = ctl(yylex).dayUnit($1, $2) }
|	tUNUMBER tDAY_UNIT        { $$ = ctl(yylex).dayUnit($1.value, $2) }
|	tDAY_UNIT                 { $$ = relTime{day: $1} }
|	tORDINAL tHOUR_UNIT       { $$ = relTime{hour: $1} }
|	tUNUMBER tHOUR_UNIT       { $$ = relTime{hour: $1.value} }
|	tHOUR_UNIT                { $$ = relTime{hour: 1} }
|	tORDINAL tMINUTE_UNIT     { $$ = relTime{minutes: $1} }
|	tUNUMBER tMINUTE_UNIT     { $$ = relTime{minutes: $1.value} }
|	tMINUTE_UNIT              { $$ = relTime{minutes: 1} }
|	tORDINAL tSEC_UNIT        { $$ = relTime{seconds: $1} }
|	tUNUMBER tSEC_UNIT        { $$ = relTime{seconds: $1.value} }
|	tSDECIMAL_NUMBER tSEC_UNIT { $$ = relTime{seconds: $1.sec, ns: $1.nsec} }
|	tUDECIMAL_NUMBER tSEC_UNIT { $$ = relTime{seconds: $1.sec, ns: $1.nsec} }
|	tSEC_UNIT                 { $$ = relTime{seconds: 1} }
|	relunit_snumber
;

relunit_snumber:
	tSNUMBER tYEAR_UNIT   { $$ = relTime{year: $1.value} }
|	tSNUMBER tMONTH_UNIT  { $$ = relTime{month: $1.value} }
|	tSNUMBER tDAY_UNIT    { $$ = ctl(yylex).dayUnit($1.value, $2) }
|	tSNUMBER tHOUR_UNIT   { $$ = relTime{hour: $1.value} }
|	tSNUMBER tMINUTE_UNIT { $$ = relTime{minutes: $1.value} }
|	tSNUMBER tSEC_UNIT    { $$ = relTime{seconds: $1.value} }
;

dayshift:
	tDAY_SHIFT { $$ = relTime{day: $1} }
;

seconds: signed_seconds | unsigned_seconds;

signed_seconds:
	tSDECIMAL_NUMBER
|	tSNUMBER { $$ = timespec{sec: $1.value} } /* time_overflow: time_t is intmax_t here */
;

unsigned_seconds:
	tUDECIMAL_NUMBER
|	tUNUMBER { $$ = timespec{sec: $1.value} }
;

number:
	tUNUMBER
	{ ctl(yylex).digitsToDateTime($1) }
;

hybrid:
	tUNUMBER relunit_snumber
	{ pc := ctl(yylex); pc.digitsToDateTime($1); if !pc.applyRelativeTime($2, 1) { pc.abort() } }
;

o_colon_minutes:
	/* empty */  { $$ = -1 }
|	':' tUNUMBER { $$ = $2.value }
;
