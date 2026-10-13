package find

import "fmt"

const usageBody = `
Default path is the current directory; default expression is -print.
Expression may consist of: operators, options, tests, and actions.

Operators (decreasing precedence; -and is implicit where no others are given):
      ( EXPR )   ! EXPR   -not EXPR   EXPR1 -a EXPR2   EXPR1 -and EXPR2
      EXPR1 -o EXPR2   EXPR1 -or EXPR2   EXPR1 , EXPR2

Positional options (always true):
      -daystart -follow -nowarn -regextype -warn

Normal options (always true, specified before other expressions):
      -depth -files0-from FILE -maxdepth LEVELS -mindepth LEVELS
      -mount -noleaf -xdev -ignore_readdir_race -noignore_readdir_race

Tests (N can be +N or -N or N):
      -amin N -anewer FILE -atime N -cmin N -cnewer FILE -context CONTEXT
      -ctime N -empty -false -fstype TYPE -gid N -group NAME -ilname PATTERN
      -iname PATTERN -inum N -iwholename PATTERN -iregex PATTERN
      -links N -lname PATTERN -mmin N -mtime N -name PATTERN -newer FILE
      -newerXY REFERENCE -nouser -nogroup -path PATTERN -perm [-/]MODE
      -regex PATTERN -readable -writable -executable
      -wholename PATTERN -size N[bcwkMG] -true -type [bcdpflsD] -uid N
      -used N -user NAME -xtype [bcdpfls]

Actions:
      -delete -print0 -printf FORMAT -fprintf FILE FORMAT -print ` + `
      -fprint0 FILE -fprint FILE -ls -fls FILE -prune -quit
      -exec COMMAND ; -exec COMMAND {} + -ok COMMAND ;
      -execdir COMMAND ; -execdir COMMAND {} + -okdir COMMAND ;

Other common options:
      --help                   display this help and exit
      --version                output version information and exit


In -newerXY, XY stands for the combination [aBcm][aBcmt]; see find(1).

`

const usageTail = `
Use '-D help' for a description of the options, or see find(1).

Please see also the documentation at https://www.gnu.org/software/findutils/.
You can report (and track progress on fixing) bugs in the "%s"
program via the GNU findutils bug-reporting page at
https://savannah.gnu.org/bugs/?group=findutils or, if
you have no web access, by sending email to <bug-findutils@gnu.org>.
`

const versionText = `find (GNU findutils) 4.11.0
Copyright (C) 2026 Free Software Foundation, Inc.
License GPLv3+: GNU GPL version 3 or later <https://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.

Written by Eric B. Decker, James Youngman, Bernhard Voelker,
and Kevin Dalley.
Features enabled: D_TYPE O_NOFOLLOW(enabled) LEAF_OPTIMISATION FTS(FTS_CWDFD) CBO(level=%d) ` + "\n"

// usage is util.c usage(EXIT_SUCCESS). Both the synopsis and
// explain_how_to_report_bugs print program_name, which is argv[0].
func (r *Run) usage() {
	r.out.WriteString(fmt.Sprintf("Usage: %s [-H] [-L] [-P] [-Olevel] [-D debugopts] [path...] [expression]\n", r.prog))
	r.out.WriteString(usageBody)
	r.showValidDebugOptions(false)
	r.out.WriteString(fmt.Sprintf(usageTail, r.prog))
	r.exit(0)
}

// version is parser.c parse_version (version_etc + the Features line).
func (r *Run) version() {
	r.out.WriteString(fmt.Sprintf(versionText, r.opt.OptimisationLevel))
	r.exit(0)
}
