# qfind — GNU find 4.11.0 in Go on fastwalk — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `qfind`, a parallel Go reimplementation of GNU find 4.11.0 on top of `github.com/charlievieth/fastwalk`, installed as `dist/find`, that passes the existing black-box RSpec suite exactly as GNU find does (0 failures, the same 547 pending examples), except for the 15 `rlimit_nproc: 0` examples that no Go binary can start under (Global Constraints).

**Architecture:** Port GNU find's *semantics* faithfully and replace only its *traversal*. The command-line parser (`parser.c`), expression tree builder and optimiser (`tree.c`), predicates (`pred.c`), `-printf`/`-ls` formatting (`print.c`, `listfile.c`), command execution (`exec.c`, `buildcmd.c`) and the per-entry visiting logic (`ftsfind.c consider_visiting`) are ported structure-for-structure into one Go package (`internal/find`). That makes evaluation order, `stat()` timing, diagnostics and `-D` output match GNU at every `-O` level. fts is replaced by fastwalk, used unmodified as fzf uses it. Each starting point is walked in order, and its subtree is walked and evaluated in parallel inside fastwalk's callbacks.

Concurrency has five rules:
- **Output transactions.** Each entry's stdout and output-file writes are buffered in an entry-local transaction and committed atomically. Commits happen at the end of the entry's evaluation and at GNU's flush points: before a child process, before a `-ok` prompt, and at `\c`. One entry's records therefore never interleave with another's.
  - `-ls` lines are formatted at commit time, so listfile.c's shared column widths only ever grow in output order.
  - stderr is written immediately, as C's unbuffered stderr is.
- **One child at a time.** At most one child process runs at a time, held under a global exec lock, as GNU waits for each child.
- **Serialised evaluation.** By default `consider_visiting` and evaluation run in parallel. One evaluation lock covers them whenever GNU's sequential semantics are observable across entries:
  - the expression contains `-quit`, `-exec`, `-execdir`, `-ok` or `-okdir`;
  - a `-printf` format contains `\c`;
  - any `-D` flag is set.

  fastwalk keeps reading directories in parallel either way.
- **`-depth`.** Files are evaluated as they stream. Directories are deferred and evaluated deepest level first, after the starting point's walk.
- **One way to die.** A fatal error commits the entry being evaluated, then claims the process-wide exit (the first fatal error wins; a later one on another goroutine blocks without printing), reports, and flushes stdout and the output files as gnulib `close_stdout` does. It exits without running pending `-exec … +` command lines.

glibc behaviour that Go does not provide lives in small leaf packages: errno text, quoting, locales, POSIX `TZ` strings, `ARG_MAX`, `rpmatch`, GNU regex and fnmatch, and gnulib modechange, filemode and parse-datetime.

**Tech Stack:** Go 1.27 (installed Fedora go1.27.2; `go.env` pins `GOTOOLCHAIN=local`), `github.com/charlievieth/fastwalk` v1.0.14 (unmodified), `golang.org/x/sys/unix`, cgo enabled (for `os/user` NSS lookups). Ruby 4 + RSpec suite in `spec/` is the only test bed. Rake builds everything.

**Spec:** `spec/` (19,214 examples: 547 must stay pending, and the other 18,667 must pass except the 15 accepted `rlimit_nproc: 0` examples), `spec/README.md` (harness, quirks, the 4 known 4.11.0 bugs that must stay pending) and `CLAUDE.md`. For behaviour questions use the GNU source in `findutils-4.11.0/` (find/*.c, lib/*.c, gl/lib/*.c) and the oracle binary `findutils-4.11.0/find/find`.

## Context

The repository already contains an exhaustive black-box RSpec suite (`spec/`, 91 files, 19,214 examples) written against GNU find 4.11.0 (commit `5af8dad`). The user now wants a new tool, **qfind**: GNU find reimplemented in Go on the fastwalk library, 100% congruent with GNU find but parallel and much faster. The suite needs only a binary at `dist/find` and is the sole test bed; no Go tests may be written. Output order is not guaranteed, and the suite already compares output order-insensitively except where order is inherent (per-file action sequences, `-depth` post-order, starting points in command-line order).

Research that shaped the design (all verified in this session):

- **fastwalk** (read in full in `references/fastwalk`, identical API in the pinned v1.0.14):
  - It calls the callback for the root and for each directory *before* reading that directory.
  - Non-directory entries are reported inline, sequentially per directory. Subdirectories are queued and their callbacks happen later on any worker.
  - There is no post-order callback.
  - `Walk` calls `os.Stat` on the root, which follows symlinks, and strips trailing slashes from it.
  - `Follow` silently skips symlink loops.
  - Read errors arrive as a second callback for the same directory.
  - Directories are opened without `O_CLOEXEC`.
- **The traversal is pinned down by GNU's code.** GNU `ftsfind.c` uses `FTS_NOSTAT|FTS_TIGHT_CYCLE_CHECK|FTS_CWDFD|FTS_VERBATIM`, and `consider_visiting()` decides per fts state whether to diagnose, visit, skip or descend. Every one of those branches has a spec. `tree.c`'s optimiser reorders `-a`/`-o` chains by cost at `-O1`..`-O3`. That changes which `stat()` calls happen and therefore which diagnostics appear, and `optimisation_spec.rb` asserts this. So the optimiser must be ported, not reinvented.
- **Ordering assertions (23 `.in_order` uses), all satisfiable without sibling order:**
  - starting points are processed in command-line order;
  - a directory comes before its contents (pre-order), or after them with `-depth`;
  - one file's actions run in sequence;
  - find's stdout is flushed before a child command runs.
- **`-quit` must be linearisable.** `quit_spec.rb` "prints only the first match" expects exactly one line from two matches in different directories.
- **Go/glibc gaps found:**
  - Go's errno text differs ("no such file or directory" vs "No such file or directory").
  - Go's `time` silently falls back to UTC for the POSIX `TZ` strings the specs use (`JST-9`, `EST5`).
  - Go has no locale support (quote characters, `-ok` yes/no patterns, case folding).
  - Go's `regexp` is RE2: no backreferences and no GNU syntax variants.
  - `path.Match` is not `fnmatch`.
- **Five spec contexts (15 examples) are impossible for any Go binary.** Under `rlimit_nproc: 0`, the Go runtime aborts before `main` ("runtime: failed to create new OS thread … fatal error: newosproc", exit 2; reproduced with `/usr/lib/golang/bin/gofmt`), because Linux counts the runtime's second OS thread against `RLIMIT_NPROC`. The affected contexts each have 3 examples and are all `:unprivileged`:
  - the "no more processes" context of `spec/support/shared_examples/exec.rb`, used for both `-exec` and `-ok`;
  - the same context of `execdir.rb`, used for both `-execdir` and `-okdir`;
  - `exec_spec.rb` "with a command line when no more processes can be created".

  The alternatives would need a single-threaded runtime: TinyGo, a patched Go runtime, or a C reimplementation behind a cgo constructor. All three contradict "reimplement in Go on fastwalk", so the plan accepts these 15 and reports them. See Global Constraints.
- **Execution semantics from the actions research:**
  - stderr is unbuffered;
  - `/dev/stdout` and `/dev/stderr` are special-cased by exact string;
  - output files are opened at parse time and shared by (dev, ino);
  - every child is preceded by a flush of stdout;
  - `-ok` children run with fd 0 closed;
  - `-exec … +` uses `buildcmd.c`'s exact byte accounting: the RashuBug fixture totals 131,073 bytes against a 131,072 limit and must split 3718 + 1.
- **The user's direction on fastwalk.** It is the deliberate choice ("fzf uses fastwalk, one of the gold-standard CLI tools of the last decade"). It is used as an unmodified dependency, and qfind adapts to its model instead of forking it.
- **Independent design review.** A Plan agent stress-tested this design against `fts.c`, `ftsfind.c`, `exec.c`, `pred.c` and fastwalk's sources. Its corrections are built into Tasks 3, 4, 8 and 9:
  - Serialise every exec-family action and every `-D` run; reasons are stdin sharing, the flush before each child, fatal exits and the fd budget.
  - fts's own stats print nothing under `-D stat`; only `options.xstat` calls print `debug_stat (NAME)`.
  - Under `-L`, fts sets `FTS_NOCHDIR`, so `fts_accpath` is the full path.
  - A directory callback must stat the entry itself and return `SkipDir` unless it is a directory to descend into. fastwalk opens with plain `O_RDONLY`, so a FIFO swapped in would block a worker.
  - Non-directory callbacks return nil, and error callbacks return nil; anything else aborts the walk.
  - A failed stat (`FTS_NS`) is visited at once with no type, even under `-depth`.
  - A cycle is never deferred.
  - An ENOENT read race under `-ignore_readdir_race`, and a mid-read error (`FTS_ERR`), cancel the deferred `-depth` visit.
  - `-execdir … +` batches are keyed by (directory, depth), because `.` and `/` share their children's working directory.
  - `exec.Cmd` semantics are wrong for GNU on four counts: stdin, PATH fallback, the working-directory probe, and the leak check.
- **fastwalk configuration decided here:**
  - `Sort: SortNone`: streaming, with memory bounded per directory. A sorted mode reads the whole directory first (1e6 entries is hundreds of MB), and POSIX leaves unspecified whether entries removed during a read are returned.
  - `Follow: false`, because `-L` is implemented with `ErrTraverseLink` plus our own loop checks.
  - `MaxDepth` mirrors `-maxdepth`.
  - `NumWorkers` is bounded by `RLIMIT_NOFILE`.
- **Accepted divergences (documented, not tested by the suite, decided now so no executor relitigates them):**
  - The 15 `rlimit_nproc` examples.
  - `-inum` stats entries that GNU matches by `d_ino`, because fastwalk does not expose `d_ino`. Only `-D stat` output differs.
  - TOCTOU between qfind's `lstat` and fastwalk's `open` of a directory, and renaming an ancestor mid-walk.
  - On filesystems without `d_type`, fastwalk abandons the rest of a directory when one entry's `lstat` fails.
  - `-depth` defers directories until the starting point's walk ends: memory is O(directories) and never O(files).
  - `-quit` stops at the first match in qfind's traversal order. The user ruled that order does not matter.
  - `-execdir … +` groups by directory, so command lines can be longer and fewer than GNU's level-change batches.
  - Without `-depth`, `-D search` does not narrate the (ignored) `FTS_DP` entries.
  - Paths beyond `PATH_MAX` until Task 15's fallback.

## Global Constraints

- The binary is `dist/find`, built from Go sources by `rake qfind:build` (`go build -trimpath -o dist/find ./cmd/qfind`). Remove `dist/find` before building, because the old file is a hard link to the GNU binary and a copy-on-build would overwrite it.
- Use Go only. The module is `qfind` with `go 1.27` in `go.mod`. The installed toolchain is Fedora go1.27.2 with `GOTOOLCHAIN=local`, so never declare a newer `go` line and never use Go 1.28 syntax (`p = {1, 2}`).
- Traversal uses `github.com/charlievieth/fastwalk` v1.0.14, unmodified, and is the only directory walker.
- **Do not write any tests in Go.** The RSpec suite is the only test bed in the repository. The Review Focus differential checks in git-ignored `.scratch/diff/` only compare qfind with the oracle; they are not tests. Do not edit `spec/` except where a task explicitly says so; none does.
- Success means `bundle exec rspec` reports **0 failures and 547 pending**, exactly like `FIND=findutils-4.11.0/find/find bundle exec rspec`. The single exception is the 15 `rlimit_nproc: 0` examples (5 contexts × 3), which no Go runtime can start under. Report them; do not hack around them. As uid 1000, the final run therefore reads **15 failures, 547 pending**, and every failure is one of those contexts.
- Every RSpec gate pipes its run through `grep -E '^[0-9]+ examples?, '`, which prints only the summary line. If a gate prints nothing, RSpec stopped before its summary; rerun it without the `grep` to see why.
- Concurrency invariants:
  - every entry's stdout and output-file writes form one atomic transaction;
  - stderr writes are immediate and record-atomic;
  - at most one child process exists at any time;
  - `-quit`, `-exec`, `-execdir`, `-ok`, `-okdir`, `\c` in a format, and any `-D` flag force serialised `consider_visiting` and evaluation;
  - starting points are strictly sequential;
  - lock order is `evalMu` → `execVal.mu` → `execMu` → a shared file's lock → stdout's lock → `lsWidths.mu` (with `stderrMu` innermost), and no code takes them in another order. `errorf` takes stdout's lock (glibc `error()` flushes stdout), so it is never called while that lock is held: `commit` and `flushStream` report through `reportWriteError` instead;
  - `go build -race` runs of the suite report no data race (Task 15).
- The 4 known-bug `pending` examples (`-ok`/`-okdir` stdin and descriptor leak) must stay pending. qfind reproduces the 4.11.0 bug: the confirmed command runs with stdin closed, and under `GNU_FINDUTILS_FD_LEAK_CHECK` it dies of `SIGABRT`.
- Do not guarantee output order beyond what GNU semantics force (starting-point order, pre-order or post-order, a single file's action sequence).
- Diagnostics go to stderr prefixed with `argv[0]` verbatim, exactly as glibc `error()` uses `program_invocation_name`. The suite runs with `argv0 = "find"`, so messages read `find: …`. `Usage: %s` and `Try '%s --help'` use the same name.
- Keep glibc-observable behaviour:
  - errno text from glibc's `strerror`;
  - locale from `LC_ALL` > `LC_<CAT>` > `LANG`, with only `C`/`POSIX`, `C.UTF-8` and `*.UTF-8` distinguished;
  - POSIX `TZ` strings;
  - `ARG_MAX` from `RLIMIT_STACK`.
- Scratch probes and differential helpers live in git-ignored `.scratch/` and never under `/tmp`.
- Write idiomatic Go (gofmt, `go vet` clean) with focused files under about 600 lines. Comments explain *why* and cite the GNU function being ported (e.g. `// port of tree.c opt_expr`).

## Review Focus

These five conditions are implied by "look, feel and work like GNU find", but no task's specs exercise them. They are the most likely to bite real users. With Go tests forbidden and the spec suite fixed, each is pinned by a **differential check** against the oracle in `.scratch/diff/`, added by the owning task.

1. **Output integrity under parallelism.** Millions of `-print`/`-print0`/`-printf` records and concurrent `-fprint` to one file must never interleave partial records. Every record must appear exactly once. (Task 4 adds `.scratch/diff/many.sh`.)
2. **Very deep trees.** GNU walks paths longer than `PATH_MAX` (4096 bytes) through `FTS_CWDFD`. fastwalk uses full paths and would get `ENAMETOOLONG`. qfind re-roots fastwalk at `/proc/self/fd/N` before a path gets that long, and does every system call through the short `Entry.sysPath`, so output matches GNU at any depth. (Task 4 adds `.scratch/diff/deep.sh`; Task 15 makes it pass.)
3. **Huge directories and `-depth`/`-delete` memory.** One directory of 1e6 entries, and `-depth -delete` on 1e6 files, must stream with bounded memory: files are never deferred, only directories are. (Task 4 adds `.scratch/diff/huge.sh`.)
4. **Broken pipes and full disks.** `qfind … | head -1` must exit silently by `SIGPIPE` like GNU. Writing search output to `/dev/full` must print exactly GNU's two lines, `find: 'standard output': No space left on device` (from `cleanup()`'s fflush) and then `find: write error` (from `close_stdout`, errno 0), with exit status 1. (Task 3 adds `.scratch/diff/pipes.sh`.)
5. **Non-UTF-8 and control-character names.** These must flow through `-regex`, `-name`, `-printf`, `-ls` and terminal quoting with GNU's byte semantics in the C locale. (Task 10 adds `.scratch/diff/bytes.sh`.)

---

## File Structure

One Go package, `internal/find`, holds the port of `find/*.c`; GNU's modules share global state and call each other freely, so splitting them across packages would only add import cycles. Leaf packages hold the gnulib/glibc pieces that have no knowledge of find. Every file has one responsibility and cites the C function(s) it ports.

```
cmd/qfind/main.go                  os.Exit(find.Main(os.Args)); optional QFIND_CPUPROFILE hook (Task 15)
internal/find/                     (task that creates each file in brackets)
  main.go            Main: defaults, locale, leading options, tree, needsSerial, start points, cleanup   [2,3,4]  (ftsfind.c main)
  options.go         Options, DebugFlags, SymlinkMode, set_option_defaults, set_follow_state, isatty       [2]  (util.c, parser.c)
  leading.go         -H -L -P -D -O, -D help, process_debug_options, process_optimisation_option          [2]  (util.c)
  run.go             Run: exit status, locks (evalMu, execMu), exit claim, shared state                   [2]
  diag.go            errorf/fatalf/exit (close_stdout), writeStderr, debugf, usageFailure, cbyte          [2]  (glibc error, gnulib closeout)
  help.go            --help and --version texts                                                           [2]  (util.c usage, parser.c parse_version)
  predicate.go       Predicate, enums, FuncID, predArgs, apply_predicate                                  [3]  (defs.h, util.c)
  pred_logic.go      pred_and/or/comma/negate/openparen/closeparen/true/false                             [3]  (pred.c)
  parsetable.go      parse_table, find_parser, found_parser                                               [3]  (parser.c)
  parser.go          build_expression_tree, collect_arg, insert_primary*, get_new_pred*                   [3]  (tree.c, parser.c, util.c)
  parse_options.go   option, positional-option and punctuation parsers, insert_depthspec                  [3]  (parser.c)
  parse_actions.go   -print -print0 -prune -quit parsers                                                  [3]  (parser.c)
  tree.go            get_expr, scan_rest, set_new_parent, merge_pred                                      [3]  (tree.c)
  optimise.go        costs, rates, opt_expr, predlists, arm swaps                                         [3]  (tree.c)
  debugtree.go       print_tree, print_list, print_optlist, show_success_rates                            [3]  (tree.c, pred.c)
  cfile.go           glibc stdio FILE emulation (buffer sizes, flush points, ferror)                      [3]  (libio)
  output.go          streams, sinks, per-entry transactions, commit, close_stdout                         [3]  (print.c, util.c)
  pred_actions.go    pred_print, pred_fprint0, pred_prune, pred_quit                                      [3]  (pred.c)
  entry.go           Entry, get_info, get_statinfo, xstat, fts info, dirPath                              [3,4]  (util.c)
  traverse.go        find() for one start point, rootInfo, walk                                           [3,4]  (ftsfind.c)
  paths.go           fts path spelling over fastwalk paths                                                [4]
  visit.go           fastwalk callback, consider_visiting, read errors                                    [4]  (ftsfind.c)
  cycles.go          FTS_TIGHT_CYCLE_CHECK over active ancestors                                          [4]
  depth.go           -depth deferred post-order visits                                                    [4]
  files0.go          process_all_startpoints incl. -files0-from                                           [4]  (ftsfind.c)
  workers.go         fastwalk NumWorkers from RLIMIT_NOFILE                                               [4]
  parse_names.go     -name -iname -path -wholename -ipath -iwholename -lname -ilname parsers             [5]  (parser.c)
  pred_names.go      their predicates, base_name, strip_trailing_slashes, readlink                       [5]  (pred.c)
  pred_types.go      -type -xtype, mode_to_filetype                                                       [6]  (parser.c, pred.c)
  sharefile.go       output files shared by (dev, ino), open_output_file                                  [7]  (sharefile.c, parser.c)
  cprintf.go         C printf %s %d %o %g with C flag semantics                                           [7]
  printf.go          insert_fprintf, make_segment                                                         [7]  (print.c)
  printf_render.go   pred_fprintf, do_fprintf directives                                                  [7]  (print.c)
  printf_time.go     ctime_format, format_date, do_time_format                                            [7]  (print.c)
  parse_print.go     -printf -fprintf -fprint -fprint0 parsers                                            [7]  (parser.c)
  buildcmd.go        bc_init_controlinfo, bc_push_arg, bc_do_insert                                       [8]  (lib/buildcmd.c)
  exec_parse.go      insert_exec_ok, check_path_safety                                                    [8]  (parser.c)
  exec.go            impl_pred_exec, -execdir batches, record_exec_dir, pending execs                     [8]  (exec.c, util.c)
  launch.go          launch: flush, execvp lookup, one child at a time, -ok bug emulation, CLOEXEC        [8]  (exec.c)
  prompt.go          -ok/-okdir question and answer                                                       [8]  (pred.c is_ok, gnulib yesno)
  ls.go              -ls/-fls, list_file rendered at commit                                               [9]  (lib/listfile.c)
  delete.go          -delete                                                                              [9]  (parser.c, pred.c)
  pred_regex.go      -regex -iregex -regextype                                                            [10] (parser.c, pred.c, lib/regextype.c)
  parse_numeric.go   get_comp_type, get_num, insert_num, -size/-inum/-links/-uid/-gid/-samefile parsers   [11] (parser.c)
  pred_numeric.go    numeric predicates, -samefile, -empty                                                [11] (pred.c)
  pred_owner.go      -user -group -nouser -nogroup                                                        [11] (parser.c, pred.c)
  parse_time.go      get_relative_timestamp, time parsers, -newerXY                                       [12] (parser.c)
  pred_time.go       pred_timewindow, timespec_sub, time predicates                                       [12] (pred.c, gnulib)
  pred_access.go     -perm, -readable, -writable, -executable                                             [13] (parser.c, pred.c)
  pred_misc.go       -fstype, -context                                                                    [13] (parser.c, pred.c, fstype.c)
  deep.go            re-rooting fastwalk at /proc/self/fd/N for paths near PATH_MAX                       [15]
internal/libc/       errno.go quote.go locale.go rpmatch.go tz.go sysconf.go ctime.go [2], strftime.go [7],
                     strtol.go [11], strtod.go [12]
internal/fnmatch/    glibc fnmatch for FNM_CASEFOLD or no flags, single-byte and UTF-8                    [5]
internal/regex/      glibc regcomp parser, bracket sets, Thompson program, Pike VM and backtracker        [10]
internal/filemode/   gnulib strmode                                                                       [7]
internal/idcache/    uid/gid ↔ name through NSS (os/user with cgo), cached                                [7]
internal/mounts/     /proc/self/mountinfo: type by device, is_used_fs_type, is_selinux_enabled            [7,13]
internal/datetime/   gnulib parse-datetime: goyacc grammar, lexer, tables, mktime emulation               [12]
internal/modechange/ gnulib modechange: mode_compile, mode_adjust                                         [13]
```

Build and helper files:

```
go.mod / go.sum                     module qfind, go 1.27, ignore ./references, fastwalk v1.0.14, x/sys v0.48.0; tool goyacc (x/tools v0.50.0)
Rakefile                            + qfind:build (dist/find); upstream:build no longer links dist/find
CLAUDE.md                           qfind task statement (Task 1)
_claude/plans/master_plan.md        this plan (already committed)
.scratch/gnufind, .scratch/qf       (git-ignored) run the oracle / qfind with the suite's environment
.scratch/failures.rb                (git-ignored) failures-per-file summary from `rspec --format json`
.scratch/diff/*.sh                  (git-ignored) differential checks of the Review Focus
```

---

## Task 1: Project scaffolding, build wiring and task statement

**Files:**
- Modify: `CLAUDE.md` (replace whole file)
- Modify: `Rakefile` (keep the user's committed `golang/go` and `charlievieth/fastwalk` references; add `qfind` namespace; stop linking GNU find into `dist/`)
- Create: `go.mod`, `go.sum`, `cmd/qfind/main.go`, `internal/find/main.go`
- Create (git-ignored): `.scratch/gnufind`, `.scratch/qf`, `.scratch/failures.rb`
- Memory: `/home/dev/.claude/projects/-workspace-qfind-master/memory/fastwalk-unmodified.md` + `MEMORY.md` line

**Interfaces:**
- Consumes: nothing.
- Produces: `find.Main(args []string) int` (stub), `rake qfind:build` producing `dist/find`, the oracle at `findutils-4.11.0/find/find`.

- [ ] **Step 1: Unlink the GNU hard link from `dist/` and prove the oracle is green**

```bash
rm dist/find
stat -c '%h' findutils-4.11.0/find/find          # expect: 1
FIND=$PWD/findutils-4.11.0/find/find bundle exec rspec 2>&1 | grep -E '^[0-9]+ examples?, '
```
Expected: `19214 examples, 0 failures, 547 pending`.

- [ ] **Step 2: Create the Go module**

```bash
go mod init qfind
go mod edit -go=1.27
go mod edit -ignore=./references
go get github.com/charlievieth/fastwalk@v1.0.14 golang.org/x/sys@v0.48.0
```
The `ignore` directive (Go 1.25+) keeps `./...` out of `references/`, the git-ignored clones that `rake` puts inside this module. `references/go` has no `go.mod` at its root, so without the directive every `go vet ./...` gate would load `references/go/test`, Go's compiler test corpus, and fail on its mixed `package` clauses. Dependencies are pinned to releases at least two weeks old: x/sys v0.48.0 is from 2026-08-31, and Task 12's x/tools v0.50.0 is from 2026-09-08. The module proxy is reachable from this machine.

- [ ] **Step 3: Write the stub entry points**

`cmd/qfind/main.go`:
```go
// Command qfind is GNU find 4.11.0 reimplemented in Go on fastwalk.
package main

import (
	"os"

	"qfind/internal/find"
)

func main() {
	os.Exit(find.Main(os.Args))
}
```

`internal/find/main.go` (replaced in Task 2):
```go
// Package find is a port of GNU findutils 4.11.0 find/*.c with fts
// replaced by github.com/charlievieth/fastwalk.
package find

import (
	"fmt"
	"os"
)

// Main runs find with argv and returns the process exit status.
func Main(args []string) int {
	fmt.Fprintln(os.Stderr, "find: not implemented yet")
	return 1
}
```

- [ ] **Step 4: Wire the build into the Rakefile**

The user has already committed two Rakefile changes: `golang/go` and `charlievieth/fastwalk` are in the `references` repos. Keep both.

Make these changes in `namespace :upstream`:
- delete the `directory archive.dist_dir` and `file archive.dist_binname` tasks;
- change `CLOBBER.include FileList[archive.extracted_name, archive.dist_dir]` to `CLOBBER.include FileList[archive.extracted_name, "dist"]`;
- build GNU find in place, as the oracle:
```ruby
  task build: archive.binname
```
Delete the now-unused `dist_dir` and `dist_binname` methods of `UpstreamArchive`. Add this after the `references` namespace:
```ruby
namespace :qfind do
  binary = "dist/find"
  sources = FileList["go.mod", "go.sum", "cmd/**/*.go", "internal/**/*.go", "internal/**/*.y"]

  directory "dist"

  file binary => ["dist", *sources] do
    rm_f binary
    run "go", "build", "-trimpath", "-o", binary, "./cmd/qfind"
  end

  desc "Build qfind into dist/find"
  task build: binary
end
```
Replace the default task. rubocop runs before the suite: as uid 1000 the suite always ends with the 15 accepted `rlimit_nproc: 0` failures (Global Constraints), and RSpec's rake task aborts `rake` on any failure, so a `:rubocop` after `:spec` would never run.
```ruby
task default: [:setup, "qfind:build", :rubocop, :spec]
```

- [ ] **Step 5: Build and confirm the suite runs against the stub**

```bash
bundle exec rake qfind:build && ./dist/find; echo "exit=$?"
bundle exec rspec 2>&1 | grep -E '^[0-9]+ examples?, '
```
Expected: `find: not implemented yet`, `exit=1`; the suite runs to completion with thousands of failures and `547 pending` or more. Record the failure count in the commit message.

- [ ] **Step 6: Scratch helpers (git-ignored)**

`.scratch/gnufind` (a caller's `TZ` wins, as an example's `env:` does in the suite; `LC_ALL` stays pinned to `C`, because the login shell exports `LC_ALL=en_US.UTF-8`):
```bash
#!/bin/bash
export LC_ALL=C TZ=${TZ:-UTC} GNU_FINDUTILS_FD_LEAK_CHECK=1
unset POSIXLY_CORRECT LANG LANGUAGE
exec -a find /workspace/qfind.master/findutils-4.11.0/find/find "$@"
```
`.scratch/qf`: the same with `/workspace/qfind.master/dist/find`. To reproduce an example that sets `LC_ALL` or `LANG`, run the binary directly with that example's environment instead of a wrapper.
`.scratch/failures.rb`:
```ruby
require "json"
report = JSON.parse(File.read(ARGV.fetch(0)))
failed = report["examples"].select { it["status"] == "failed" }
failed.group_by { it["file_path"] }.sort_by { -it.last.size }.each do |file, examples|
  puts format("%5d  %s", examples.size, file)
end
puts report["summary_line"]
```
Use: `bundle exec rspec --format json --out .scratch/run.json --format progress; ruby .scratch/failures.rb .scratch/run.json`.

- [ ] **Step 7: Replace `CLAUDE.md`**

```markdown
# qfind

## Task

Reimplement all of GNU find (findutils 4.11.0) in Go as a new tool, **qfind**, specifically on top of the fastwalk Go library (`github.com/charlievieth/fastwalk`, source in `references/fastwalk`).

- The goal is 100% congruence with GNU find, but with parallelism. It should look, feel and work like GNU find, albeit much faster.
- An exhaustive black-box RSpec suite has been prepared in `spec/`. It only requires a binary at `dist/find`.
- Do not write any tests in Go. Rely solely on the spec suite; the throwaway differential checks against GNU find in git-ignored `.scratch/diff/` are not tests.
- This is a streaming problem, so order does not matter (GNU find does not guarantee it either). Do not provide any order guarantees beyond what GNU semantics force: starting points in command-line order, a directory before its contents (after them with `-depth`), and one file's actions in sequence.
- fastwalk is deliberate: fzf uses it, and fzf is one of the gold-standard CLI tools of the last decade. Use it unmodified and adapt qfind to its model rather than forking it.

The implementation plan is `_claude/plans/master_plan.md`.

## Setup

Run `rake` to build everything, then run rubocop and the suite. As a non-root user it exits non-zero, because the 15 `rlimit_nproc: 0` examples, which no Go binary can start under, fail by design.

- `dist/find` is qfind: `rake qfind:build` (`go build -trimpath -o dist/find ./cmd/qfind`).
- GNU find 4.11.0 is built from source as the reference oracle at `findutils-4.11.0/find/find` (`rake upstream:build`). Run the suite against it with `FIND=$PWD/findutils-4.11.0/find/find bundle exec rspec`.
- Reference sources are cloned into `references/`: `rspec`, `go` (Go master), `fastwalk`.

## The spec suite

`spec/` black-boxes `dist/find`, and `spec/README.md` documents its harness and every GNU quirk it pins. qfind is done when `bundle exec rspec` matches GNU find: 0 failures and 547 pending, apart from the 15 `rlimit_nproc: 0` examples (5 contexts × 3), which no Go binary can start under. As a non-root user the final run therefore reads 15 failures, 547 pending; report those 15 and do not hack around them. (`spec/README.md`'s "0 failures and 547 pending" is GNU find's result.)

If the specs themselves ever need to change, first and foremost use the /rspec skill, and these rules from the original task apply:

1. These are blackbox tests. This means no stubs or mocks. EVERY spec should actually shell out to find/gfind
2. 1 expectation per example. No exceptions
3. Absolutely no comments allowed in the specs. Any comments in specs need to be organized into their own specs
4. Constantly reference the rspec source to ensure idiomatic use. It can be found in references/rspec

Use rspec features like shared_examples, shared_context etc to keep things DRY and behavior based, if needed

Context that cannot become a spec goes in `spec/README.md`.

## Git

Make every commit a clean, new commit. No amending and no force pushing.
```

- [ ] **Step 8: Save the fastwalk decision to memory**

Write `/home/dev/.claude/projects/-workspace-qfind-master/memory/fastwalk-unmodified.md`:
```markdown
---
name: fastwalk-unmodified
description: qfind must use fastwalk as an unmodified dependency and adapt to its model; the user vouches for it because fzf uses it
metadata:
  type: feedback
---

Build qfind's traversal on github.com/charlievieth/fastwalk as a plain, unmodified dependency, like fzf does.

**Why:** the user pointed out (2026-10-13) that fzf uses fastwalk and is one of the gold-standard CLI tools of the last decade; they reacted to concerns about fastwalk's missing post-order callback and loop handling.
**How to apply:** design around fastwalk's model (pre-order callbacks, ErrTraverseLink, second callback for read errors) instead of forking or patching it; see [[go-toolchain-env]].
```
Append to `MEMORY.md`: `- [fastwalk unmodified](fastwalk-unmodified.md) — qfind uses fastwalk as-is like fzf; adapt to its model, don't fork it`

- [ ] **Step 9: Commit the scaffolding**

The plan is already committed at `_claude/plans/master_plan.md`, so there is nothing to copy.
```bash
gofmt -l cmd internal; go vet ./...
git add CLAUDE.md Rakefile go.mod go.sum cmd internal
git commit -m "Scaffold qfind: Go module on fastwalk, dist/find build, task statement"
```

---

## Task 2: glibc-parity layer, run state, diagnostics and leading options

GNU find's observable text comes from glibc (`strerror`, `error()`, quoting, locale, `TZ`, `ARG_MAX`). This task builds that layer and the front end that runs before expression parsing:
- option defaults (`util.c set_option_defaults`);
- `-H`/`-L`/`-P`/`-D`/`-O`/`--` (`util.c process_leading_options`);
- `--help`/`--version` texts;
- `-D help`;
- `FIND_BLOCK_SIZE`;
- write errors on stdout.

**Files:**
- Create: `internal/libc/errno.go`, `internal/libc/quote.go`, `internal/libc/locale.go`, `internal/libc/rpmatch.go`, `internal/libc/tz.go`, `internal/libc/sysconf.go`, `internal/libc/ctime.go`
- Create: `internal/find/options.go`, `internal/find/leading.go`, `internal/find/run.go`, `internal/find/diag.go`, `internal/find/help.go`
- Modify: `internal/find/main.go`

**Interfaces:**
- Consumes: Task 1 stub.
- Produces (used by every later task):
  - `libc.Strerror(err error) string`, `libc.Errno(err error) (syscall.Errno, bool)`
  - `libc.Locale{CType, Messages, Time string; UTF8 bool}`, `libc.LocaleFromEnv(lookup func(string) (string, bool)) libc.Locale`, `(Locale).Quote(s string) string` (locale_quoting_style), `(Locale).Qmark(s string) string` (print_quoted on a tty), `(Locale).IsYes(line string) bool`
  - `libc.LoadTZ(lookup func(string) (string, bool)) *time.Location`, `libc.ArgMax() int64`, `libc.Ctime(t time.Time) string`
  - `find.Options` (port of `struct options`), `find.DebugFlags`, `find.SymlinkMode`
  - `*find.Run` with `errorf(err error, format string, a ...any)`, `fatalf(err error, format string, a ...any)` (never returns), `setFailure()`, `quote(s string) string`, `exit(status int)` (never returns), `stdout *Output` (Task 3 fills `Output`)

- [ ] **Step 1: Confirm the target specs fail**

Run: `bundle exec rspec spec/find/options/help_spec.rb spec/find/options/version_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '`
Expected: every example fails ("find: not implemented yet").

- [ ] **Step 2: `internal/libc/errno.go` — glibc strerror text**

```go
// Package libc reproduces the glibc behaviour GNU find exposes and Go
// does not: strerror text, gnulib quoting, locales, POSIX TZ strings,
// sysconf(_SC_ARG_MAX) and ctime(3).
package libc

import (
	"errors"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

// Errno extracts the errno wrapped in err (os.PathError, os.SyscallError…).
func Errno(err error) (syscall.Errno, bool) {
	var e syscall.Errno
	if errors.As(err, &e) {
		return e, true
	}
	return 0, false
}

// Strerror is glibc strerror(3) for err's errno; other errors keep their text.
func Strerror(err error) string {
	e, ok := Errno(err)
	if !ok {
		return err.Error()
	}
	if s, ok := strerrors[e]; ok {
		return s
	}
	return "Unknown error " + strconv.Itoa(int(e))
}

// strerrors is glibc 2.44's message table (generated with
// ruby -e 'SystemCallError.new(nil, n).message' for n in 1..133).
var strerrors = map[syscall.Errno]string{
	unix.EPERM: "Operation not permitted", unix.ENOENT: "No such file or directory",
	unix.ESRCH: "No such process", unix.EINTR: "Interrupted system call",
	unix.EIO: "Input/output error", unix.ENXIO: "No such device or address",
	unix.E2BIG: "Argument list too long", unix.ENOEXEC: "Exec format error",
	unix.EBADF: "Bad file descriptor", unix.ECHILD: "No child processes",
	unix.EAGAIN: "Resource temporarily unavailable", unix.ENOMEM: "Cannot allocate memory",
	unix.EACCES: "Permission denied", unix.EFAULT: "Bad address",
	unix.ENOTBLK: "Block device required", unix.EBUSY: "Device or resource busy",
	unix.EEXIST: "File exists", unix.EXDEV: "Invalid cross-device link",
	unix.ENODEV: "No such device", unix.ENOTDIR: "Not a directory",
	unix.EISDIR: "Is a directory", unix.EINVAL: "Invalid argument",
	unix.ENFILE: "Too many open files in system", unix.EMFILE: "Too many open files",
	unix.ENOTTY: "Inappropriate ioctl for device", unix.ETXTBSY: "Text file busy",
	unix.EFBIG: "File too large", unix.ENOSPC: "No space left on device",
	unix.ESPIPE: "Illegal seek", unix.EROFS: "Read-only file system",
	unix.EMLINK: "Too many links", unix.EPIPE: "Broken pipe",
	unix.EDOM: "Numerical argument out of domain", unix.ERANGE: "Numerical result out of range",
	unix.EDEADLK: "Resource deadlock avoided", unix.ENAMETOOLONG: "File name too long",
	unix.ENOLCK: "No locks available", unix.ENOSYS: "Function not implemented",
	unix.ENOTEMPTY: "Directory not empty", unix.ELOOP: "Too many levels of symbolic links",
	unix.ENOMSG: "No message of desired type", unix.EIDRM: "Identifier removed",
	unix.ECHRNG: "Channel number out of range", unix.EL2NSYNC: "Level 2 not synchronized",
	unix.EL3HLT: "Level 3 halted", unix.EL3RST: "Level 3 reset",
	unix.ELNRNG: "Link number out of range", unix.EUNATCH: "Protocol driver not attached",
	unix.ENOCSI: "No CSI structure available", unix.EL2HLT: "Level 2 halted",
	unix.EBADE: "Invalid exchange", unix.EBADR: "Invalid request descriptor",
	unix.EXFULL: "Exchange full", unix.ENOANO: "No anode",
	unix.EBADRQC: "Invalid request code", unix.EBADSLT: "Invalid slot",
	unix.EBFONT: "Bad font file format", unix.ENOSTR: "Device not a stream",
	unix.ENODATA: "No data available", unix.ETIME: "Timer expired",
	unix.ENOSR: "Out of streams resources", unix.ENONET: "Machine is not on the network",
	unix.ENOPKG: "Package not installed", unix.EREMOTE: "Object is remote",
	unix.ENOLINK: "Link has been severed", unix.EADV: "Advertise error",
	unix.ESRMNT: "Srmount error", unix.ECOMM: "Communication error on send",
	unix.EPROTO: "Protocol error", unix.EMULTIHOP: "Multihop attempted",
	unix.EDOTDOT: "RFS specific error", unix.EBADMSG: "Bad message",
	unix.EOVERFLOW: "Value too large for defined data type", unix.ENOTUNIQ: "Name not unique on network",
	unix.EBADFD: "File descriptor in bad state", unix.EREMCHG: "Remote address changed",
	unix.ELIBACC: "Can not access a needed shared library", unix.ELIBBAD: "Accessing a corrupted shared library",
	unix.ELIBSCN: ".lib section in a.out corrupted", unix.ELIBMAX: "Attempting to link in too many shared libraries",
	unix.ELIBEXEC: "Cannot exec a shared library directly", unix.EILSEQ: "Invalid or incomplete multibyte or wide character",
	unix.ERESTART: "Interrupted system call should be restarted", unix.ESTRPIPE: "Streams pipe error",
	unix.EUSERS: "Too many users", unix.ENOTSOCK: "Socket operation on non-socket",
	unix.EDESTADDRREQ: "Destination address required", unix.EMSGSIZE: "Message too long",
	unix.EPROTOTYPE: "Protocol wrong type for socket", unix.ENOPROTOOPT: "Protocol not available",
	unix.EPROTONOSUPPORT: "Protocol not supported", unix.ESOCKTNOSUPPORT: "Socket type not supported",
	unix.EOPNOTSUPP: "Operation not supported", unix.EPFNOSUPPORT: "Protocol family not supported",
	unix.EAFNOSUPPORT: "Address family not supported by protocol", unix.EADDRINUSE: "Address already in use",
	unix.EADDRNOTAVAIL: "Cannot assign requested address", unix.ENETDOWN: "Network is down",
	unix.ENETUNREACH: "Network is unreachable", unix.ENETRESET: "Network dropped connection on reset",
	unix.ECONNABORTED: "Software caused connection abort", unix.ECONNRESET: "Connection reset by peer",
	unix.ENOBUFS: "No buffer space available", unix.EISCONN: "Transport endpoint is already connected",
	unix.ENOTCONN: "Transport endpoint is not connected", unix.ESHUTDOWN: "Cannot send after transport endpoint shutdown",
	unix.ETOOMANYREFS: "Too many references: cannot splice", unix.ETIMEDOUT: "Connection timed out",
	unix.ECONNREFUSED: "Connection refused", unix.EHOSTDOWN: "Host is down",
	unix.EHOSTUNREACH: "No route to host", unix.EALREADY: "Operation already in progress",
	unix.EINPROGRESS: "Operation now in progress", unix.ESTALE: "Stale file handle",
	unix.EUCLEAN: "Structure needs cleaning", unix.ENOTNAM: "Not a XENIX named type file",
	unix.ENAVAIL: "No XENIX semaphores available", unix.EISNAM: "Is a named type file",
	unix.EREMOTEIO: "Remote I/O error", unix.EDQUOT: "Disk quota exceeded",
	unix.ENOMEDIUM: "No medium found", unix.EMEDIUMTYPE: "Wrong medium type",
	unix.ECANCELED: "Operation canceled", unix.ENOKEY: "Required key not available",
	unix.EKEYEXPIRED: "Key has expired", unix.EKEYREVOKED: "Key has been revoked",
	unix.EKEYREJECTED: "Key was rejected by service", unix.EOWNERDEAD: "Owner died",
	unix.ENOTRECOVERABLE: "State not recoverable", unix.ERFKILL: "Operation not possible due to RF-kill",
	unix.EHWPOISON: "Memory page has hardware error",
}
```
(`EDEADLOCK`, `ENOTSUP` and `EWOULDBLOCK` alias entries above; listing them again is a duplicate-key compile error.)

- [ ] **Step 3: `internal/libc/locale.go`, `rpmatch.go` — setlocale(LC_ALL, "") emulation**

```go
package libc

import "strings"

// Locale is the outcome of setlocale(LC_ALL, "") for the categories find
// reads: LC_CTYPE (charset, MB_CUR_MAX), LC_MESSAGES (yes/no) and LC_TIME.
type Locale struct {
	CType, Messages, Time string
	UTF8                  bool // LC_CTYPE codeset is UTF-8, so MB_CUR_MAX > 1
}

// LocaleFromEnv applies glibc precedence: LC_ALL, then LC_<CATEGORY>, then
// LANG; an unset or empty value falls through, and the result defaults to "C".
// No findutils translations are installed, so only these names matter.
func LocaleFromEnv(lookup func(string) (string, bool)) Locale {
	pick := func(category string) string {
		for _, name := range []string{"LC_ALL", category, "LANG"} {
			if v, ok := lookup(name); ok && v != "" {
				return v
			}
		}
		return "C"
	}
	l := Locale{CType: pick("LC_CTYPE"), Messages: pick("LC_MESSAGES"), Time: pick("LC_TIME")}
	l.UTF8 = isUTF8(l.CType)
	return l
}

// isUTF8 reports whether a locale name's codeset is UTF-8 ("C.UTF-8",
// "en_US.utf8", "de_DE.UTF-8@euro").
func isUTF8(name string) bool {
	if i := strings.IndexByte(name, '@'); i >= 0 {
		name = name[:i]
	}
	i := strings.IndexByte(name, '.')
	if i < 0 {
		return false
	}
	cs := strings.ToLower(strings.ReplaceAll(name[i+1:], "-", ""))
	return cs == "utf8"
}

func isCLocale(name string) bool {
	return name == "C" || name == "POSIX" || strings.HasPrefix(name, "C.")
}
```
`rpmatch.go`:
```go
package libc

// IsYes is gnulib yesno()'s `0 < rpmatch(line)`: glibc's YESEXPR for
// LC_MESSAGES is "^[yY]" in C/POSIX/C.UTF-8 and "^[+1yY]" in en_US and the
// other installed locales (spec/README.md: with en_US.UTF-8, "1" is yes).
func (l Locale) IsYes(line string) bool {
	if line == "" {
		return false
	}
	switch line[0] {
	case 'y', 'Y':
		return true
	case '+', '1':
		return !isCLocale(l.Messages)
	}
	return false
}
```

- [ ] **Step 4: `internal/libc/quote.go` — gnulib quotearg as find uses it**

```go
package libc

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Quote is quotearg_n_style(n, locale_quoting_style, s), find's
// err_quoting_style. Quotes are '…' unless LC_CTYPE is UTF-8, which gives
// U+2018/U+2019 (gnulib gettext_quote). Control characters get C escapes,
// '\\' and the closing quote are backslash-escaped, and non-printable bytes
// (every byte >= 0x80 in a single-byte locale) become \ooo.
func (l Locale) Quote(s string) string {
	left, right := "'", "'"
	if l.UTF8 {
		left, right = "\u2018", "\u2019"
	}
	var b strings.Builder
	b.WriteString(left)
	for i := 0; i < len(s); {
		c := s[i]
		if esc := cEscape(c); esc != "" {
			b.WriteString(esc)
			i++
			continue
		}
		if strings.HasPrefix(s[i:], right) {
			b.WriteByte('\\')
			b.WriteString(right)
			i += len(right)
			continue
		}
		if c >= 0x20 && c < 0x7f {
			b.WriteByte(c)
			i++
			continue
		}
		if l.UTF8 && c >= 0x80 {
			if r, n := utf8.DecodeRuneInString(s[i:]); r != utf8.RuneError && unicode.IsPrint(r) {
				b.WriteString(s[i : i+n])
				i += n
				continue
			}
		}
		b.WriteByte('\\')
		b.WriteString(octal3(c))
		i++
	}
	b.WriteString(right)
	return b.String()
}

func cEscape(c byte) string {
	switch c {
	case '\a':
		return `\a`
	case '\b':
		return `\b`
	case '\f':
		return `\f`
	case '\n':
		return `\n`
	case '\r':
		return `\r`
	case '\t':
		return `\t`
	case '\v':
		return `\v`
	case '\\':
		return `\\`
	}
	return ""
}

func octal3(c byte) string {
	o := strconv.FormatUint(uint64(c), 8)
	return strings.Repeat("0", 3-len(o)) + o
}

// Qmark is print_quoted() for a terminal destination: literal quoting
// followed by qmark_chars(), which turns every non-printable character
// (byte in a single-byte locale, character in UTF-8) into '?'.
func (l Locale) Qmark(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		c := s[i]
		if c >= 0x20 && c < 0x7f {
			b.WriteByte(c)
			i++
			continue
		}
		if l.UTF8 && c >= 0x80 {
			r, n := utf8.DecodeRuneInString(s[i:])
			if r != utf8.RuneError || n > 1 {
				if unicode.IsPrint(r) {
					b.WriteString(s[i : i+n])
				} else {
					b.WriteByte('?')
				}
				i += n
				continue
			}
		}
		b.WriteByte('?')
		i++
	}
	return b.String()
}
```

- [ ] **Step 5: `internal/libc/tz.go`, `sysconf.go`, `ctime.go`**

`tz.go`:
```go
package libc

import (
	"encoding/binary"
	"strings"
	"time"
)

// LoadTZ resolves $TZ like glibc tzset(): unset → /etc/localtime (Go's
// time.Local); empty → UTC; ":name" or "name" → zoneinfo file; otherwise a
// POSIX rule string such as "EST5" or "JST-9", which Go's time package
// silently turns into UTC.
func LoadTZ(lookup func(string) (string, bool)) *time.Location {
	tz, ok := lookup("TZ")
	if !ok {
		return time.Local
	}
	name := strings.TrimPrefix(tz, ":")
	if name == "" {
		return time.UTC
	}
	if loc, err := time.LoadLocation(name); err == nil {
		return loc
	}
	if loc, err := time.LoadLocationFromTZData(name, posixTZif(name)); err == nil {
		return loc
	}
	return time.UTC
}

// posixTZif wraps a POSIX TZ rule in a TZif v2 file with no transitions and
// one placeholder type. Go applies the footer rule (its internal tzset) to
// every instant after the last transition, which here is all of them.
func posixTZif(rule string) []byte {
	header := func(b []byte, version byte, typecnt, charcnt uint32) []byte {
		b = append(b, "TZif"...)
		b = append(b, version)
		b = append(b, make([]byte, 15)...)
		for _, n := range []uint32{0, 0, 0, 0, typecnt, charcnt} { // isut, isstd, leap, time, type, char
			b = binary.BigEndian.AppendUint32(b, n)
		}
		return b
	}
	block := func(b []byte) []byte {
		b = append(b, 0, 0, 0, 0, 0, 0) // ttinfo: utoff 0, isdst 0, desigidx 0
		return append(b, "UTC\x00"...)
	}
	var b []byte
	b = block(header(b, '2', 1, 4))
	b = block(header(b, '2', 1, 4))
	b = append(b, '\n')
	b = append(b, rule...)
	return append(b, '\n')
}
```
Validate by hand after building: `TZ=JST-9 ./dist/find …` is exercised by `printf_times_spec.rb` and `ls_spec.rb` in later tasks. In this task, check `libc.LoadTZ` through `-D time`, which prints `cur_day_start` in local time.

`sysconf.go`:
```go
package libc

import "golang.org/x/sys/unix"

// legacyArgMax is glibc's ARG_MAX floor (131072).
const legacyArgMax = 131072

// ArgMax is glibc sysconf(_SC_ARG_MAX): MAX(131072, RLIMIT_STACK.cur / 4).
// RLIM_INFINITY therefore yields a huge value, as in glibc.
func ArgMax() int64 {
	var rl unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_STACK, &rl); err != nil {
		return legacyArgMax
	}
	if q := rl.Cur / 4; q > legacyArgMax && q < 1<<62 {
		return int64(q)
	}
	if rl.Cur == unix.RLIM_INFINITY {
		return 1 << 62
	}
	return legacyArgMax
}
```
`ctime.go`:
```go
package libc

import (
	"fmt"
	"time"
)

// Ctime is ctime(3): asctime's "%.3s %.3s%3d %.2d:%.2d:%.2d %d\n".
func Ctime(t time.Time) string {
	return fmt.Sprintf("%.3s %.3s%3d %02d:%02d:%02d %d\n",
		t.Weekday().String(), t.Month().String(), t.Day(),
		t.Hour(), t.Minute(), t.Second(), t.Year())
}
```

- [ ] **Step 6: `internal/find/options.go` — `struct options` and `set_option_defaults`**

```go
package find

import (
	"time"

	"golang.org/x/sys/unix"

	"qfind/internal/regex"
)

// SymlinkMode is enum SymlinkOption (defs.h).
type SymlinkMode uint8

const (
	SymlinkNeverDeref    SymlinkMode = iota // -P (default)
	SymlinkDerefArgsOnly                    // -H
	SymlinkAlwaysDeref                      // -L, -follow
)

// DebugFlags are the -D bits (defs.h DebugXxx values).
type DebugFlags uint

const (
	DebugExpressionTree DebugFlags = 1 << iota // tree
	DebugStat                                  // stat
	DebugSearch                                // search
	DebugTreeOpt                               // opt (with tree)
	DebugHelp                                  // help
	DebugExec                                  // exec
	DebugSuccessRates                          // rates
	DebugTime                                  // time
	DebugAll = DebugExpressionTree | DebugStat | DebugSearch | DebugTreeOpt |
		DebugExec | DebugSuccessRates | DebugTime
)

// Timespec mirrors struct timespec (nanosecond timestamps compare exactly).
type Timespec struct{ Sec, Nsec int64 }

// Options is struct options (defs.h).
type Options struct {
	DoDirFirst          bool
	ExplicitDepth       bool
	MaxDepth, MinDepth  int // -1 when unset
	NoLeafCheck         bool
	Mount, Xdev         bool
	IgnoreReaddirRace   bool
	LiteralControlChars bool
	Warnings            bool
	PosixlyCorrect      bool
	StartTime           Timespec
	CurDayStart         Timespec
	FullDays            bool
	OutputBlockSize     int
	Debug               DebugFlags
	Symlinks            SymlinkMode
	RegexSyntax         regex.Syntax // created in Task 10; until then: type Syntax uint32 in internal/regex
	OptimisationLevel   int
	Files0From          string
	HasFiles0From       bool
	OkPromptStdin       bool
}

// setDefaults is util.c set_option_defaults.
func (r *Run) setDefaults() {
	o := &r.opt
	_, o.PosixlyCorrect = r.lookupEnv("POSIXLY_CORRECT")
	o.RegexSyntax = regex.SyntaxEmacs
	o.Warnings = isatty(0) && !o.PosixlyCorrect
	o.DoDirFirst = true
	o.MaxDepth, o.MinDepth = -1, -1
	now := time.Now()
	o.StartTime = Timespec{now.Unix(), int64(now.Nanosecond())}
	o.CurDayStart = Timespec{o.StartTime.Sec - 86400, o.StartTime.Nsec}
	o.OutputBlockSize = 1024
	if o.PosixlyCorrect {
		o.OutputBlockSize = 512
	}
	o.OptimisationLevel = 1
	if _, ok := r.lookupEnv("FIND_BLOCK_SIZE"); ok {
		r.fatalf(nil, "The environment variable FIND_BLOCK_SIZE is not supported, the only thing that affects the block size is the POSIXLY_CORRECT environment variable")
	}
	o.NoLeafCheck = false // LEAF_OPTIMISATION is on in the reference build
	r.setFollowState(SymlinkNeverDeref)
}

// setFollowState is parser.c set_follow_state.
func (r *Run) setFollowState(m SymlinkMode) {
	r.opt.Symlinks = m
	if m != SymlinkNeverDeref {
		r.opt.NoLeafCheck = true
	}
}

func isatty(fd int) bool {
	_, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	return err == nil
}
```
`internal/regex` is created properly in Task 10. Here, create a placeholder `internal/regex/syntax.go` with just the type and the default constant, and Task 10 extends it:
```go
// Package regex implements GNU regex syntaxes (Task 10 completes it).
package regex

// Syntax is a set of RE_* syntax bits (gnulib regex.h).
type Syntax uint32

// SyntaxEmacs is RE_SYNTAX_EMACS = RE_CHAR_CLASSES | RE_INTERVALS.
const SyntaxEmacs Syntax = 1<<2 | 1<<9 // RE_CHAR_CLASSES | RE_INTERVALS (regex.h: bits 2 and 9)
```
(Task 10 replaces the literal with named bits copied from `gl/lib/regex.h`.)

- [ ] **Step 7: `internal/find/run.go`, `diag.go` — run state, `error()`, exit**

```go
package find

import (
	"os"
	"sync"
	"sync/atomic"
	"time"

	"qfind/internal/libc"
)

// Run is the process-wide state: GNU's `options` plus the parts of `state`
// that are not per file (exit status, shared files, pending execs).
type Run struct {
	prog       string // argv[0], used like program_invocation_name
	opt        Options
	loc        libc.Locale
	tz         *time.Location
	env        []string
	out          *Output // Task 3
	stderrMu     sync.Mutex
	stderrFailed atomic.Bool // ferror(stderr): close_stdout then exits 1
	exitStatus   atomic.Int32
}

func newRun(args []string) *Run {
	prog := "find"
	if len(args) > 0 {
		prog = args[0]
	}
	r := &Run{prog: prog, env: os.Environ()}
	r.loc = libc.LocaleFromEnv(r.lookupEnv)
	r.tz = libc.LoadTZ(r.lookupEnv)
	return r
}

func (r *Run) lookupEnv(name string) (string, bool) { return os.LookupEnv(name) }

func (r *Run) setFailure() { r.exitStatus.Store(1) }

// quote is safely_quote_err_filename: locale_quoting_style.
func (r *Run) quote(s string) string { return r.loc.Quote(s) }
```
`diag.go`:
```go
package find

import (
	"fmt"
	"os"
	"strings"

	"qfind/internal/libc"
)

// errorf is glibc error(0, errnum, format, …): flush stdout, then print
// "<prog>: <message>[: <strerror>]\n" on stderr. It does not set the exit
// status; callers do, as GNU does.
func (r *Run) errorf(err error, format string, a ...any) {
	if r.out != nil {
		r.out.flushForDiagnostic()
	}
	var b strings.Builder
	b.WriteString(r.prog)
	b.WriteString(": ")
	fmt.Fprintf(&b, format, a...)
	if err != nil {
		b.WriteString(": ")
		b.WriteString(libc.Strerror(err))
	}
	b.WriteByte('\n')
	r.writeStderr(b.String())
}

// writeStderr is an unbuffered fputs to stderr: one write per record, and
// a failure is remembered like ferror(stderr).
func (r *Run) writeStderr(text string) error {
	r.stderrMu.Lock()
	defer r.stderrMu.Unlock()
	_, err := os.Stderr.WriteString(text)
	if err != nil {
		r.stderrFailed.Store(true)
	}
	return err
}

// debugf is fprintf(stderr, …) for -D output: no program-name prefix.
func (r *Run) debugf(format string, a ...any) {
	r.writeStderr(fmt.Sprintf(format, a...))
}

// cbyte is C's %c of a char: the raw byte. Go's %c would UTF-8-encode a
// byte >= 0x80 as a rune, which GNU's messages never do.
func cbyte(b byte) string { return string([]byte{b}) }

// fatalf is error(EXIT_FAILURE, errnum, …): report, then exit(1) through the
// same flush-and-close path as a normal exit. It claims the exit before it
// reports, so only the first fatal error is printed.
func (r *Run) fatalf(err error, format string, a ...any) {
	r.claimExit()
	r.errorf(err, format, a...)
	r.exitClaimed(1)
}

// usageFailure is usage(EXIT_FAILURE): no program prefix on the hint.
func (r *Run) usageFailure() {
	r.writeStderr(fmt.Sprintf("Try '%s --help' for more information.\n", r.prog))
	r.exit(1)
}

// exit is exit(status) with gnulib close_stdout() registered via atexit:
// shared output files are flushed, then stdout is closed; a stdout failure
// prints "write error" and forces status 1 (EPIPE included: GNU sets no
// ignore_EPIPE, and SIGPIPE normally kills the process first).
//
// exit is the only way out of the process; see claimExit.
func (r *Run) exit(status int) {
	r.claimExit()
	r.exitClaimed(status)
}

// claimExit makes the caller the one goroutine that exits the process (GNU,
// being single-threaded, dies once). The first caller wins. A later caller
// is on another goroutine, because nothing that runs after a claim dies
// again, so it blocks until the winner's os.Exit instead of printing a
// second message or exiting with another status. A write failure inside
// fatalAt's commit claims first, as GNU would have died at that write.
func (r *Run) claimExit() {
	if !r.exiting.CompareAndSwap(false, true) {
		select {}
	}
}

// exitClaimed is exit's work once the caller holds the claim.
func (r *Run) exitClaimed(status int) {
	if r.out != nil {
		r.out.flushFilesQuietly() // Task 7: stdio flushes every FILE at exit
		if err := r.out.closeAll(); err != nil {
			if err == errPrevFailed {
				r.errorf(nil, "write error")
			} else {
				r.errorf(err, "write error")
			}
			status = 1
		}
	}
	// close_stdout also runs close_stream(stderr): after any failed write to
	// stderr (2>/dev/full), it calls _exit(EXIT_FAILURE) without a message.
	if r.stderrFailed.Load() {
		status = 1
	}
	os.Exit(status)
}
```
Add `exiting atomic.Bool` to `Run`.

For this task, add a minimal `Output` in `internal/find/output.go` that buffers stdout through `bufio.NewWriterSize(os.Stdout, 64<<10)`. It exposes:
- `flushForDiagnostic()`;
- `closeAll() error`;
- `WriteString(s string)`;
- a no-op `flushFilesQuietly()`;
- `var errPrevFailed = errors.New("write error after earlier failure")`.

Task 3 replaces it, keeping these names, and adds `fatalAt` (Task 3, Step 10) once `Entry` and `commit` exist.

- [ ] **Step 8: `internal/find/leading.go` — `process_leading_options`, `-D`, `-O`**

```go
package find

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"syscall"
)

var debugAssoc = []struct {
	name string
	val  DebugFlags
	doc  string
}{
	{"exec", DebugExec, "Show diagnostic information relating to -exec, -execdir, -ok and -okdir"},
	{"opt", DebugExpressionTree | DebugTreeOpt, "Show diagnostic information relating to optimisation"},
	{"rates", DebugSuccessRates, "Indicate how often each predicate succeeded"},
	{"search", DebugSearch, "Navigate the directory tree verbosely"},
	{"stat", DebugStat, "Trace calls to stat(2) and lstat(2)"},
	{"time", DebugTime, "Show diagnostic information relating to time-of-day and timestamp comparisons"},
	{"tree", DebugExpressionTree, "Display the expression tree"},
	{"all", DebugAll, "Set all of the debug flags (but help)"},
	{"help", DebugHelp, "Explain the various -D options"},
}

// processLeadingOptions is util.c process_leading_options; it returns the
// index of the first argument after the leading options.
func (r *Run) processLeadingOptions(args []string) int {
	i := 1
	for ; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-H":
			r.setFollowState(SymlinkDerefArgsOnly)
		case a == "-L":
			r.setFollowState(SymlinkAlwaysDeref)
		case a == "-P":
			r.setFollowState(SymlinkNeverDeref)
		case a == "--":
			return i + 1
		case a == "-D":
			if i+1 >= len(args) {
				r.errorf(nil, "Missing argument after the -D option.")
				r.usageFailure()
			}
			r.processDebugOptions(args[i+1])
			i++
		case strings.HasPrefix(a, "-O"):
			r.processOptimisationOption(a[2:])
		default:
			return i
		}
	}
	return i
}

// processDebugOptions is util.c process_debug_options. The unknown-flag
// warning quotes the FIRST token of the list (strtok_r has cut it), which
// is a 4.11.0 bug the spec tolerates; reproduce it.
func (r *Run) processDebugOptions(arg string) {
	tokens := strings.FieldsFunc(arg, func(c rune) bool { return c == ',' })
	if len(tokens) == 0 {
		r.errorf(nil, "Empty argument to the -D option.")
		r.usageFailure()
	}
	for _, t := range tokens {
		found := false
		for _, d := range debugAssoc {
			if d.name == t {
				r.opt.Debug |= d.val
				found = true
				break
			}
		}
		if !found {
			r.errorf(nil, "Ignoring unrecognised debug flag %s", r.quote(tokens[0]))
		}
	}
	if r.opt.Debug&DebugHelp != 0 {
		r.showValidDebugOptions(true)
		r.exit(0)
	}
}

// showValidDebugOptions is util.c show_valid_debug_options.
func (r *Run) showValidDebugOptions(full bool) {
	var b strings.Builder
	b.WriteString("Valid arguments for -D:\n")
	for i, d := range debugAssoc {
		if full {
			fmt.Fprintf(&b, "%-10s %s\n", d.name, d.doc)
		} else if i == 0 {
			b.WriteString("      " + d.name)
		} else {
			b.WriteString(", " + d.name)
		}
	}
	r.out.WriteString(b.String())
}

// processOptimisationOption is util.c process_optimisation_option.
func (r *Run) processOptimisationOption(arg string) {
	if arg == "" {
		r.fatalf(nil, "The -O option must be immediately followed by a decimal integer")
	}
	if arg[0] < '0' || arg[0] > '9' {
		r.fatalf(nil, "Please specify a decimal number immediately after -O")
	}
	end := 0
	for end < len(arg) && arg[end] >= '0' && arg[end] <= '9' {
		end++
	}
	if end < len(arg) {
		r.fatalf(nil, "Invalid optimisation level %s", arg)
	}
	level, err := strconv.ParseUint(arg, 10, 64)
	if errors.Is(err, strconv.ErrRange) {
		r.fatalf(syscall.ERANGE, "Invalid optimisation level %s", arg)
	}
	if level > 65535 {
		r.fatalf(nil, "Optimisation level %d is too high.  If you want to find files very quickly, consider using GNU locate.", level)
	}
	r.opt.OptimisationLevel = int(level)
}
```

- [ ] **Step 9: `internal/find/help.go` — verbatim texts**

The usage text is exactly the 4.11.0 output captured from the oracle, including the trailing space after `-print`, the blank lines after `--version`, and the `-D` list without a trailing newline before "Use". Keep it in a raw string, and make sure no editor strips the trailing space.

```go
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

// usage is util.c usage(EXIT_SUCCESS).
func (r *Run) usage() {
	r.out.WriteString(fmt.Sprintf("Usage: %s [-H] [-L] [-P] [-Olevel] [-D debugopts] [path...] [expression]\n", r.prog))
	r.out.WriteString(usageBody)
	r.showValidDebugOptions(false)
	r.out.WriteString(fmt.Sprintf(usageTail, "find"))
	r.exit(0)
}

// version is parser.c parse_version (version_etc + the Features line).
func (r *Run) version() {
	r.out.WriteString(fmt.Sprintf(versionText, r.opt.OptimisationLevel))
	r.exit(0)
}
```
`"%s"` in `usageTail` is the program name used by `explain_how_to_report_bugs`. In the oracle output it reads `"find"` with argv0 `find`; use `r.prog` instead of the literal if `help_spec.rb` ever runs the program under another name.

- [ ] **Step 10: `internal/find/main.go` — the front-end sequence of ftsfind.c `main`**

```go
package find

import (
	"time"

	"qfind/internal/libc"
)

// Main runs find with argv and returns the exit status (ftsfind.c main).
func Main(args []string) int {
	r := newRun(args)
	r.out = newOutput(r)
	r.setDefaults()
	end := r.processLeadingOptions(args)
	if r.opt.Debug&DebugTime != 0 {
		r.writeStderr("cur_day_start = " + libcCtime(r))
	}
	// Task 3: build the expression tree (parses -help/-version, which exit).
	// Task 4: process start points, show success rates, cleanup.
	_ = end
	r.exit(int(r.exitStatus.Load()))
	return 0
}
```
with `func libcCtime(r *Run) string { return libc.Ctime(time.Unix(r.opt.CurDayStart.Sec, 0).In(r.tz)) }`. Until Task 3 lands, make `--help`/`-help` and `--version`/`-version` work by scanning `args[end:]` here for the first argument whose one-dash-stripped form is `-help` or `-version`, *only if* every argument before it looks like a path. Task 3 deletes this shim when the real parse table handles both.

- [ ] **Step 11: Build and run the gates**

```bash
bundle exec rake qfind:build
bundle exec rspec spec/find/options/help_spec.rb spec/find/options/version_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
bundle exec rspec spec/find/options/debug_spec.rb spec/find/options/optimisation_spec.rb -e 'without a level' -e 'not a number' -e 'negative level' -e 'trailing the level' -e 'above 65535' -e 'out of the numeric range' -e 'separate argument' -e 'without a list' -e 'with the list ""' -e 'with the list ","' -e 'with help' 2>&1 | grep -E '^[0-9]+ examples?, '
```
Expected:
- help/version: 0 failures, except the placement-warning example (`find . -name x --help` on a terminal) and the `-foo --help` unknown-predicate examples, which need Task 3.
- `-D`/`-O` rejections and `-D help`: 0 failures.

Check `/dev/full` by hand:
```bash
./dist/find --help >/dev/full; echo $?
```
Expected: `find: write error: No space left on device`, then `1`.

- [ ] **Step 12: Commit**

```bash
gofmt -l internal; go vet ./...
git add internal
git commit -m "qfind: glibc-parity layer, leading options, --help/--version, -D help"
```

---

## Task 3: Expression engine — predicate model, parser, tree builder, optimiser, output transactions

This task ports GNU's command-line → expression pipeline exactly:
- `parser.c`: parse table, `find_parser`/`found_parser` warnings, the option/punctuation/`-print` parsers;
- `tree.c`: `build_expression_tree`, `get_expr`, `scan_rest`, `opt_expr`, arm swaps, cost estimation, `-D tree/opt` printing;
- `util.c`: `apply_predicate`, `looks_like_expression`;
- `pred.c`: the logical predicates.

It also adds the stdout layer with per-file transactions, and evaluates starting points at depth 0 only. Descent arrives in Task 4.

**Files:**
- Create: `internal/find/predicate.go`, `parsetable.go`, `parser.go`, `parse_options.go`, `parse_actions.go`, `tree.go`, `optimise.go`, `debugtree.go`, `pred_logic.go`, `pred_actions.go`, `output.go` (replaces the Task 2 minimal one), `entry.go`, `traverse.go`
- Modify: `internal/find/main.go` (remove the Task 2 `--help` shim)

**Interfaces:**
- Consumes: Task 2 `Run`, `Options`, `errorf`/`fatalf`/`exit`, `libc.Locale`.
- Produces (exact names used by Tasks 4–15):
  - **Predicate model.** `type Predicate struct` with fields `fn PredFunc; id FuncID; name string; typ PredType; prec Prec; sideEffects, noDefaultPrint, needStat, needType, needInum bool; cost Cost; estSuccessRate float32; artificial bool; argText string; hasArgText bool; args predArgs; next, left, right *Predicate; entry *parserEntry`.
  - **Evaluation types.** `type PredFunc func(r *Run, e *Entry, p *Predicate) bool`; `type FuncID uint8` with constants `fnAmin … fnXtype` (one per `pred_*` in `tree.c costlookup`).
  - **Parser.** `type parser struct{ r *Run; argv []string; i int; … }` with:
    - `(*parser).collectArg() (string, bool)`
    - `(*parser).collectArgStatInfo() (string, unix.Stat_t, bool)` (Task 4 fills the xstat call)
    - `(*parser).insertPrimary(e *parserEntry, arg string, hasArg bool) *Predicate`
    - `(*parser).insertPrimaryWithPred(e *parserEntry, fn PredFunc, id FuncID, arg string, hasArg bool) *Predicate`
    - `(*parser).insertPrimaryNoArg(e *parserEntry) *Predicate`
    - `(*parser).shouldIssueWarnings() bool`
  - **Parse table.** `type parserEntry struct{ class argClass; name string; parse parseFunc; fn PredFunc; id FuncID }` and `type parseFunc func(ps *parser, e *parserEntry) bool`.
  - **Run hooks.** `(*Run).apply(e *Entry, p *Predicate) bool`; `(*Run).getInfo(e *Entry, p *Predicate) bool`.
  - **Output.**
    - `type cfile` emulates glibc stdio: `newCfile(*os.File)`, `write([]byte) error`, `flushBuf() error`, and `err` as ferror.
    - `type sink interface{ lock(); unlock(); write(b []byte) error; flush() error }`, implemented by `*Output` (stdout) and Task 7's `*sharedFile`.
    - `type Stream struct{ name string; kind streamKind; tty bool; sink sink }`. `sink` is nil for stderr.
    - `type failMode` (`failIgnored`, `failReported`, `failFatal`) and `type piece struct{ text string; stage int }`.
    - Queueing:
      - `(*Run).emit(e *Entry, s *Stream, text string)` (unchecked);
      - `(*Run).emitChecked(e *Entry, s *Stream, text string, fail failMode)`;
      - `(*Run).emitRendered(e *Entry, s *Stream, render func() []piece)` (Task 9's `-ls`);
      - `(*Run).emitQuoted(e *Entry, s *Stream, name, suffix string)` (`print_quoted`).
    - Committing and flushing:
      - `(*Run).commit(e *Entry)`;
      - `(*Run).flushStream(e *Entry, s *Stream, fail failMode)`;
      - `(*Run).flushStdout(e *Entry)`;
      - `(*Run).reportWriteError(held sink, s *Stream, err error)`.
    - `(*Run).fatalAt(e *Entry, err error, format string, a ...any)`.
  - **Entry.** `type Entry struct{ path, name, accPath string; depth int; haveStat, haveType bool; mode uint32; st unix.Stat_t; issuedStatErr, prune bool; txn txn; … }`, plus `(*Run).xstat(e *Entry)` and `(*Run).xstatName(path, shown string, depth int) (unix.Stat_t, error)`.

- [ ] **Step 1: Record the failing gates**

```bash
bundle exec rspec spec/find/operators spec/find/expressions_spec.rb spec/find/tests/true_spec.rb spec/find/tests/false_spec.rb spec/find/actions/print_spec.rb spec/find/actions/print0_spec.rb spec/find/positional_options/warn_spec.rb spec/find/global_options/placement_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
```
Expected: nearly every example fails.

- [ ] **Step 2: `predicate.go` — `struct predicate`, enums, `apply_predicate`**

```go
package find

import (
	"sync/atomic"

	"golang.org/x/sys/unix"
)

// PredType is enum predicate_type (defs.h).
type PredType uint8

const (
	NoType PredType = iota
	PrimaryType
	UniOp
	BiOp
	OpenParen
	CloseParen
)

// Prec is enum predicate_precedence (defs.h).
type Prec uint8

const (
	NoPrec Prec = iota
	CommaPrec
	OrPrec
	AndPrec
	NegatePrec
	MaxPrec
)

// Cost is enum EvaluationCost (defs.h). The order matters: the optimiser
// compares costs and buckets predicates by them.
type Cost uint8

const (
	NeedsNothing Cost = iota
	NeedsInodeNumber
	NeedsType
	NeedsStatInfo
	NeedsLinkName
	NeedsAccessInfo
	NeedsSyncDiskHit
	NeedsEventualExec
	NeedsImmediateExec
	NeedsUserInteraction
	NeedsUnknown
	numCosts
)

// FuncID names the pred_* function of a node. GNU compares pred_func
// pointers (pred_is, the cost table); Go func values are not comparable.
type FuncID uint8

const (
	fnNone FuncID = iota
	fnAmin
	fnAnd
	fnAnewer
	fnAtime
	fnCloseParen
	fnCmin
	fnCnewer
	fnComma
	fnContext
	fnCtime
	fnDelete
	fnEmpty
	fnExec
	fnExecdir
	fnExecutable
	fnFalse
	fnFls
	fnFprint
	fnFprint0
	fnFprintf
	fnFstype
	fnGid
	fnGroup
	fnIlname
	fnIname
	fnInum
	fnIpath
	fnLinks
	fnLname
	fnLs
	fnMmin
	fnMtime
	fnName
	fnNegate
	fnNewer
	fnNewerXY
	fnNogroup
	fnNouser
	fnOk
	fnOkdir
	fnOpenParen
	fnOr
	fnPath
	fnPerm
	fnPrint
	fnPrint0
	fnPrune
	fnQuit
	fnReadable
	fnRegex
	fnSamefile
	fnSize
	fnTrue
	fnType
	fnUid
	fnUsed
	fnUser
	fnWritable
	fnXtype
)

// PredFunc is a pred_* function.
type PredFunc func(r *Run, e *Entry, p *Predicate) bool

// Predicate is struct predicate (defs.h). next links command-line order;
// left/right form the evaluation tree.
type Predicate struct {
	fn             PredFunc
	id             FuncID
	name           string // p_name, as typed
	typ            PredType
	prec           Prec
	sideEffects    bool
	noDefaultPrint bool
	needStat       bool
	needType       bool
	needInum       bool
	cost           Cost
	estSuccessRate float32
	artificial     bool
	argText        string
	hasArgText     bool
	args           predArgs
	next           *Predicate
	left, right    *Predicate
	entry          *parserEntry
	visits         atomic.Uint64
	successes      atomic.Uint64
}

// predArgs is the args union of struct predicate. Tasks add fields as they
// port predicates (regex, exec, num, size, uid, gid, reftime, perm,
// samefile, types, format, scontext).
type predArgs struct {
	str string  // fstype, [i]lname, [i]name, [i]path
	out *Stream // printf_vec.stream for the print family
}

func (p *Predicate) is(id FuncID) bool { return p.id == id }

// apply is util.c apply_predicate.
func (r *Run) apply(e *Entry, p *Predicate) bool {
	if r.countRates {
		p.visits.Add(1)
	}
	if p.needStat || p.needType || p.needInum {
		if !r.getInfo(e, p) {
			return false
		}
	}
	if !p.fn(r, e, p) {
		return false
	}
	if r.countRates {
		p.successes.Add(1)
	}
	return true
}

var _ = unix.S_IFMT
```
Add these fields to `Run`:
- `countRates bool`: set `countRates = opt.Debug&DebugSuccessRates != 0` before evaluation.
- `startPoints []string`.
- `tree *Predicate`: the evaluation tree.
- `preds *Predicate`: the command-line predicate list head, kept for later scans such as Task 4's `needsSerial`.

- [ ] **Step 3: `pred_logic.go` — `pred_and/or/comma/negate/openparen/closeparen/true/false`**

```go
package find

// predAnd is pred.c pred_and.
func predAnd(r *Run, e *Entry, p *Predicate) bool {
	if p.left == nil || r.apply(e, p.left) {
		return r.apply(e, p.right)
	}
	return false
}

// predOr is pred.c pred_or.
func predOr(r *Run, e *Entry, p *Predicate) bool {
	if p.left == nil || !r.apply(e, p.left) {
		return r.apply(e, p.right)
	}
	return true
}

// predComma is pred.c pred_comma: both sides run, the right one decides.
func predComma(r *Run, e *Entry, p *Predicate) bool {
	if p.left != nil {
		r.apply(e, p.left)
	}
	return r.apply(e, p.right)
}

func predNegate(r *Run, e *Entry, p *Predicate) bool { return !r.apply(e, p.right) }

func predOpenparen(*Run, *Entry, *Predicate) bool  { return true }
func predCloseparen(*Run, *Entry, *Predicate) bool { return true }
func predTrue(*Run, *Entry, *Predicate) bool       { return true }
func predFalse(*Run, *Entry, *Predicate) bool      { return false }
```

- [ ] **Step 4: `parsetable.go` — `parse_table[]`, `find_parser`, `found_parser`**

Every primary is listed now. Primaries owned by later tasks point at `parseLater` until their task replaces the entry (each later task shows the replacement line). `parseLater` reports `unknown predicate`, so their specs fail cleanly rather than misbehave.

```go
package find

type argClass uint8

const (
	argOption argClass = iota + 1
	argPositionalOption
	argPunctuation
	argTest
	argAction
	argSpecialParse
	argNoop
)

type parseFunc func(ps *parser, e *parserEntry) bool

// parserEntry is struct parser_table (defs.h).
type parserEntry struct {
	class argClass
	name  string // parser_name, without the first '-'
	parse parseFunc
	fn    PredFunc
	id    FuncID
}

var (
	parseTable        []parserEntry
	parseEntryNewerXY parserEntry
)

// parseLater stands in for a primary whose port lands in a later task.
func parseLater(ps *parser, e *parserEntry) bool {
	ps.r.fatalf(nil, "unknown predicate `%s'", ps.argv[ps.i-1])
	return false
}

func init() {
	parseTable = []parserEntry{
		{argOption, "d", parseD, nil, fnNone},
		{argOption, "depth", parseDepth, nil, fnNone},
		{argOption, "files0-from", parseFiles0From, nil, fnNone},
		{argOption, "ignore_readdir_race", parseIgnoreRace, nil, fnNone},
		{argOption, "maxdepth", parseMaxdepth, nil, fnNone},
		{argOption, "mindepth", parseMindepth, nil, fnNone},
		{argOption, "mount", parseMount, nil, fnNone},
		{argOption, "noleaf", parseNoleaf, nil, fnNone},
		{argOption, "noignore_readdir_race", parseNoignoreRace, nil, fnNone},
		{argOption, "xdev", parseXdev, nil, fnNone},
		{argOption, "help", parseHelp, nil, fnNone},
		{argOption, "-help", parseHelp, nil, fnNone},
		{argOption, "version", parseVersion, nil, fnNone},
		{argOption, "-version", parseVersion, nil, fnNone},
		{argPositionalOption, "daystart", parseDaystart, nil, fnNone},
		{argPositionalOption, "follow", parseFollow, nil, fnNone},
		{argPositionalOption, "nowarn", parseNowarn, nil, fnNone},
		{argPositionalOption, "regextype", parseLater, nil, fnNone}, // Task 10
		{argPositionalOption, "warn", parseWarn, nil, fnNone},
		{argPunctuation, "!", parseNegate, predNegate, fnNegate},
		{argPunctuation, "not", parseNegate, predNegate, fnNegate},
		{argPunctuation, "(", parseOpenparen, predOpenparen, fnOpenParen},
		{argPunctuation, ")", parseCloseparen, predCloseparen, fnCloseParen},
		{argPunctuation, ",", parseComma, predComma, fnComma},
		{argPunctuation, "a", parseAnd, predAnd, fnAnd},
		{argPunctuation, "and", parseAnd, predAnd, fnAnd},
		{argPunctuation, "o", parseOr, predOr, fnOr},
		{argPunctuation, "or", parseOr, predOr, fnOr},
		{argTest, "amin", parseLater, nil, fnAmin},            // Task 12
		{argTest, "anewer", parseLater, nil, fnAnewer},        // Task 12
		{argTest, "atime", parseLater, nil, fnAtime},          // Task 12
		{argTest, "cmin", parseLater, nil, fnCmin},            // Task 12
		{argTest, "cnewer", parseLater, nil, fnCnewer},        // Task 12
		{argTest, "ctime", parseLater, nil, fnCtime},          // Task 12
		{argTest, "context", parseLater, nil, fnContext},      // Task 13
		{argTest, "empty", parseLater, nil, fnEmpty},          // Task 11
		{argTest, "executable", parseLater, nil, fnExecutable}, // Task 13
		{argTest, "false", parseFalse, predFalse, fnFalse},
		{argTest, "fstype", parseLater, nil, fnFstype},    // Task 13
		{argTest, "gid", parseLater, nil, fnGid},          // Task 11
		{argTest, "group", parseLater, nil, fnGroup},      // Task 11
		{argTest, "ilname", parseLater, nil, fnIlname},    // Task 5
		{argTest, "iname", parseLater, nil, fnIname},      // Task 5
		{argTest, "inum", parseLater, nil, fnInum},        // Task 11
		{argTest, "ipath", parseLater, nil, fnIpath},      // Task 5
		{argTest, "iregex", parseLater, nil, fnRegex},     // Task 10
		{argTest, "iwholename", parseLater, nil, fnIpath}, // Task 5
		{argTest, "links", parseLater, nil, fnLinks},      // Task 11
		{argTest, "lname", parseLater, nil, fnLname},      // Task 5
		{argTest, "mmin", parseLater, nil, fnMmin},        // Task 12
		{argTest, "mtime", parseLater, nil, fnMtime},      // Task 12
		{argTest, "name", parseLater, nil, fnName},        // Task 5
		{argTest, "newer", parseLater, nil, fnNewer},      // Task 12
		{argTest, "nogroup", parseLater, nil, fnNogroup},  // Task 11
		{argTest, "nouser", parseLater, nil, fnNouser},    // Task 11
		{argTest, "path", parseLater, nil, fnPath},        // Task 5
		{argTest, "perm", parseLater, nil, fnPerm},        // Task 13
		{argTest, "readable", parseLater, nil, fnReadable}, // Task 13
		{argTest, "regex", parseLater, nil, fnRegex},      // Task 10
		{argTest, "samefile", parseLater, nil, fnSamefile}, // Task 11
		{argTest, "size", parseLater, nil, fnSize},        // Task 11
		{argTest, "true", parseTrue, predTrue, fnTrue},
		{argTest, "type", parseLater, nil, fnType},        // Task 6
		{argTest, "uid", parseLater, nil, fnUid},          // Task 11
		{argTest, "used", parseLater, nil, fnUsed},        // Task 12
		{argTest, "user", parseLater, nil, fnUser},        // Task 11
		{argTest, "wholename", parseLater, nil, fnPath},   // Task 5
		{argTest, "writable", parseLater, nil, fnWritable}, // Task 13
		{argTest, "xtype", parseLater, nil, fnXtype},      // Task 6
		{argAction, "delete", parseLater, nil, fnDelete},  // Task 9
		{argAction, "exec", parseLater, nil, fnExec},      // Task 8
		{argAction, "execdir", parseLater, nil, fnExecdir}, // Task 8
		{argAction, "fls", parseLater, nil, fnFls},        // Task 9
		{argAction, "fprint", parseLater, nil, fnFprint},  // Task 7
		{argAction, "fprint0", parseLater, nil, fnFprint0}, // Task 7
		{argAction, "fprintf", parseLater, nil, fnFprintf}, // Task 7
		{argAction, "ls", parseLater, nil, fnLs},          // Task 9
		{argAction, "ok", parseLater, nil, fnOk},          // Task 8
		{argAction, "okdir", parseLater, nil, fnOkdir},    // Task 8
		{argAction, "print", parsePrint, predPrint, fnPrint},
		{argAction, "print0", parsePrint0, predPrint0, fnPrint0},
		{argAction, "printf", parseLater, nil, fnFprintf}, // Task 7
		{argAction, "prune", parsePrune, predPrune, fnPrune},
		{argAction, "quit", parseQuit, predQuit, fnQuit},
		{argNoop, "--noop", nil, predTrue, fnTrue},
	}
	parseEntryNewerXY = parserEntry{argSpecialParse, "newerXY", parseLater, nil, fnNewerXY} // Task 12
}

// lookupEntry finds a table entry by its parser_name.
func lookupEntry(name string) *parserEntry {
	for i := range parseTable {
		if parseTable[i].name == name {
			return &parseTable[i]
		}
	}
	return nil
}

// findParser is parser.c find_parser.
func (ps *parser) findParser(arg string) *parserEntry {
	if len(arg) == 8 && arg[:6] == "-newer" {
		return ps.foundParser(arg, &parseEntryNewerXY)
	}
	name := arg
	if name != "" && name[0] == '-' {
		name = name[1:]
	}
	e := lookupEntry(name)
	if e == nil {
		return nil
	}
	if name != arg && e.class == argPunctuation && len(name) == 1 &&
		(name == "!" || name == "," || name == "(" || name == ")") {
		ps.r.errorf(nil, "warning: operator '%s' (with leading dash '-') will no longer be accepted in future findutils releases!", arg)
	}
	return ps.foundParser(arg, e)
}

// foundParser is parser.c found_parser: the placement warning for global
// options after a non-option argument.
func (ps *parser) foundParser(arg string, e *parserEntry) *parserEntry {
	if e.class == argPositionalOption {
		return e
	}
	if e.class == argNoop {
		return nil
	}
	if e.class == argOption {
		if ps.hasFirstNonOption && ps.shouldIssueWarnings() {
			ps.r.errorf(nil, "warning: you have specified the global option %s after the argument %s, but global options are not positional, i.e., %s affects tests specified before it as well as those specified after it.  Please specify global options before other arguments.", arg, ps.firstNonOption, arg)
		}
		return e
	}
	if !ps.hasFirstNonOption {
		ps.firstNonOption, ps.hasFirstNonOption = arg, true
	}
	return e
}
```

- [ ] **Step 5: `parser.go` — `build_expression_tree` input loop and helpers**

```go
package find

import "golang.org/x/sys/unix"

// parser holds parser.c/tree.c file-level state for one command line.
type parser struct {
	r                 *Run
	argv              []string
	i                 int        // *arg_ptr
	preds             *Predicate // predicates
	last              *Predicate // last_pred
	firstNonOption    string
	hasFirstNonOption bool
}

func (ps *parser) shouldIssueWarnings() bool {
	return !ps.r.opt.PosixlyCorrect && ps.r.opt.Warnings
}

// collectArg is parser.c collect_arg.
func (ps *parser) collectArg() (string, bool) {
	if ps.i >= len(ps.argv) {
		return "", false
	}
	ps.i++
	return ps.argv[ps.i-1], true
}

// getNewPred is tree.c get_new_pred: append a defaulted node.
func (ps *parser) getNewPred(e *parserEntry) *Predicate {
	p := &Predicate{entry: e, needStat: true, needType: true, cost: NeedsUnknown,
		estSuccessRate: 1.0, argText: "ThisShouldBeSetToSomethingElse", hasArgText: true}
	if ps.preds == nil {
		ps.preds = p
	} else {
		ps.last.next = p
	}
	ps.last = p
	return p
}

func (ps *parser) getNewPredNoArg(e *parserEntry) *Predicate {
	p := ps.getNewPred(e)
	p.argText, p.hasArgText = "", false
	return p
}

// getNewPredChkOp is tree.c get_new_pred_chk_op: interpose an implicit -a
// after a primary or ')'.
func (ps *parser) getNewPredChkOp(e *parserEntry, arg string, hasArg bool) *Predicate {
	if ps.last != nil {
		switch ps.last.typ {
		case NoType:
			ps.r.fatalf(nil, "oops -- invalid default insertion of and!  This is probably a bug.")
		case PrimaryType, CloseParen:
			and := ps.getNewPredNoArg(lookupEntry("and"))
			and.fn, and.id, and.name = predAnd, fnAnd, "-a"
			and.typ, and.prec = BiOp, AndPrec
			and.needStat, and.needType, and.needInum = false, false, false
		}
	}
	p := ps.getNewPred(e)
	p.argText, p.hasArgText = arg, hasArg
	p.entry = e
	return p
}

// insertPrimaryWithPred is util.c insert_primary_withpred.
func (ps *parser) insertPrimaryWithPred(e *parserEntry, fn PredFunc, id FuncID, arg string, hasArg bool) *Predicate {
	p := ps.getNewPredChkOp(e, arg, hasArg)
	p.fn, p.id, p.name = fn, id, e.name
	p.typ, p.prec = PrimaryType, NoPrec
	return p
}

func (ps *parser) insertPrimary(e *parserEntry, arg string, hasArg bool) *Predicate {
	return ps.insertPrimaryWithPred(e, e.fn, e.id, arg, hasArg)
}

func (ps *parser) insertPrimaryNoArg(e *parserEntry) *Predicate {
	return ps.insertPrimary(e, "", false)
}

// looksLikeExpression is util.c looks_like_expression.
func looksLikeExpression(arg string, leading bool) bool {
	if arg == "" {
		return false
	}
	switch arg[0] {
	case '-':
		return len(arg) > 1
	case ')', ',':
		return len(arg) == 1 && !leading
	case '!', '(':
		return len(arg) == 1
	}
	return false
}

// defaultPrints is util.c default_prints.
func defaultPrints(p *Predicate) bool {
	for ; p != nil; p = p.next {
		if p.noDefaultPrint {
			return false
		}
	}
	return true
}

// checkOptionCombinations is parser.c check_option_combinations.
func (r *Run) checkOptionCombinations(p *Predicate) {
	var del, prune bool
	for ; p != nil; p = p.next {
		del = del || p.is(fnDelete)
		prune = prune || p.is(fnPrune)
	}
	if del && prune && !r.opt.ExplicitDepth {
		r.fatalf(nil, "The -delete action automatically turns on -depth, but -prune does nothing when -depth is in effect.  If you want to carry on anyway, just explicitly use the -depth option.")
	}
}

// buildExpressionTree is tree.c build_expression_tree.
func (r *Run) buildExpressionTree(args []string, end int) *Predicate {
	ps := &parser{r: r, argv: args}
	i := end
	for i < len(args) && !looksLikeExpression(args[i], true) {
		r.startPoints = append(r.startPoints, args[i])
		i++
	}
	ps.i = i
	parseOpenparen(ps, lookupEntry("("))
	ps.last.name, ps.preds.artificial = "(", true

	for ps.i < len(args) {
		arg := args[ps.i]
		if !looksLikeExpression(arg, false) {
			r.errorf(nil, "paths must precede expression: `%s'", arg)
			if unix.Access(arg, unix.F_OK) == nil {
				r.errorf(nil, "possible unquoted pattern after predicate `%s'?", ps.last.name)
			}
			r.exit(1)
		}
		e := ps.findParser(arg)
		if e == nil {
			r.fatalf(nil, "unknown predicate `%s'", arg)
		}
		if e.class != argSpecialParse {
			ps.i++
		}
		old := ps.i
		if !e.parse(ps, e) {
			if ps.i < len(args) {
				if e.class == argSpecialParse && ps.i == old {
					r.fatalf(nil, "invalid predicate `%s'", arg)
				}
				r.fatalf(nil, "invalid argument `%s' to `%s'", args[ps.i], arg)
			}
			r.fatalf(nil, "missing argument to `%s'", arg)
		}
		ps.last.name = arg
		if ps.i != old {
			ps.last.argText, ps.last.hasArgText = args[old], true
		} else {
			ps.last.argText, ps.last.hasArgText = "", false
		}
	}

	switch {
	case ps.preds.next == nil:
		ps.preds, ps.last = nil, nil
		parsePrint(ps, lookupEntry("print"))
		ps.last.name = "-print"
	case !defaultPrints(ps.preds.next):
		ps.preds = ps.preds.next
	default:
		parseCloseparen(ps, lookupEntry(")"))
		ps.last.name, ps.last.artificial = ")", true
		parsePrint(ps, lookupEntry("print"))
		ps.last.name, ps.last.artificial = "-print", true
	}

	if r.opt.Debug&(DebugExpressionTree|DebugTreeOpt) != 0 {
		r.debugf("Predicate List:\n")
		r.printList(ps.preds)
	}
	r.preds = ps.preds
	r.checkOptionCombinations(ps.preds)

	cur := ps.preds
	tree := r.getExpr(&cur, NoPrec, nil)
	calculateDerivedRates(tree)
	if cur != nil {
		if cur.is(fnCloseParen) {
			r.fatalf(nil, "you have too many ')'")
		}
		if cur.name != "" {
			r.fatalf(nil, "unexpected extra predicate '%s'", cur.name)
		}
		r.fatalf(nil, "unexpected extra predicate")
	}
	if r.opt.Debug&(DebugExpressionTree|DebugTreeOpt) != 0 {
		r.debugf("Eval Tree:\n")
		r.printTree(tree, 0)
	}
	estimateCosts(tree)
	r.optExpr(&tree)
	if r.opt.OptimisationLevel > 1 {
		r.doArmSwaps(tree)
	}
	if r.opt.Debug&(DebugExpressionTree|DebugTreeOpt) != 0 {
		r.debugf("Optimized Eval Tree:\n")
		r.printTree(tree, 0)
		r.debugf("Optimized command line:\n")
		r.printOptlist(tree)
		r.debugf("\n")
	}
	return tree
}
```
`r.debugf` (Task 2's `diag.go`) writes to stderr with no prefix, as GNU's `fprintf(stderr, …)` does.

- [ ] **Step 6: `parse_options.go` and `parse_actions.go` — options, punctuation, `-true/-false/-print/-print0/-prune/-quit`**

```go
package find

import (
	"strconv"
	"strings"
	"syscall"
	"time"
)

// parseTrue is parser.c parse_true.
func parseTrue(ps *parser, e *parserEntry) bool {
	p := ps.insertPrimaryNoArg(e)
	p.needStat, p.needType = false, false
	p.estSuccessRate = 1.0
	return true
}

// insertFalse is parser.c insert_false (also used by -fstype substitution).
func (ps *parser) insertFalse() bool {
	e := ps.findParser("false")
	p := ps.insertPrimaryNoArg(e)
	p.needStat, p.needType = false, false
	p.sideEffects, p.noDefaultPrint = false, false
	p.estSuccessRate = 0
	return true
}

func parseFalse(ps *parser, e *parserEntry) bool { return ps.insertFalse() }

// parseNoop is parser.c parse_noop: a -true node later renamed to the option.
func parseNoop(ps *parser, e *parserEntry) bool {
	for i := range parseTable {
		if parseTable[i].class == argNoop {
			return parseTrue(ps, &parseTable[i])
		}
	}
	return false
}

func parseD(ps *parser, e *parserEntry) bool {
	if ps.shouldIssueWarnings() {
		ps.r.errorf(nil, "warning: the -d option is deprecated; please use -depth instead, because the latter is a POSIX-compliant feature.")
	}
	return parseDepth(ps, e)
}

func parseDepth(ps *parser, e *parserEntry) bool {
	ps.r.opt.DoDirFirst, ps.r.opt.ExplicitDepth = false, true
	return parseNoop(ps, e)
}

// parseDaystart is parser.c parse_daystart: local midnight today, once.
func parseDaystart(ps *parser, e *parserEntry) bool {
	o := &ps.r.opt
	if !o.FullDays {
		o.CurDayStart.Sec += 86400
		o.CurDayStart.Nsec = 0
		t := time.Unix(o.CurDayStart.Sec, 0).In(ps.r.tz)
		o.CurDayStart.Sec -= int64(t.Second() + t.Minute()*60 + t.Hour()*3600)
		o.FullDays = true
	}
	return true // inserts no predicate: the caller renames the previous one
}

func parseFollow(ps *parser, e *parserEntry) bool {
	ps.r.setFollowState(SymlinkAlwaysDeref)
	return parseNoop(ps, e)
}
func parseWarn(ps *parser, e *parserEntry) bool   { ps.r.opt.Warnings = true; return parseNoop(ps, e) }
func parseNowarn(ps *parser, e *parserEntry) bool { ps.r.opt.Warnings = false; return parseNoop(ps, e) }
func parseIgnoreRace(ps *parser, e *parserEntry) bool {
	ps.r.opt.IgnoreReaddirRace = true
	return parseNoop(ps, e)
}
func parseNoignoreRace(ps *parser, e *parserEntry) bool {
	ps.r.opt.IgnoreReaddirRace = false
	return parseNoop(ps, e)
}
func parseMount(ps *parser, e *parserEntry) bool  { ps.r.opt.Mount = true; return parseNoop(ps, e) }
func parseXdev(ps *parser, e *parserEntry) bool   { ps.r.opt.Xdev = true; return parseNoop(ps, e) }
func parseNoleaf(ps *parser, e *parserEntry) bool { ps.r.opt.NoLeafCheck = true; return parseNoop(ps, e) }
func parseHelp(ps *parser, e *parserEntry) bool   { ps.r.usage(); return true }
func parseVersion(ps *parser, e *parserEntry) bool {
	ps.r.version()
	return true
}

// parseFiles0From is parser.c parse_files0_from: no predicate is inserted.
func parseFiles0From(ps *parser, e *parserEntry) bool {
	name, ok := ps.collectArg()
	if ok {
		ps.r.opt.Files0From, ps.r.opt.HasFiles0From = name, true
	}
	return ok
}

func parseMaxdepth(ps *parser, e *parserEntry) bool { return ps.insertDepthspec(e, &ps.r.opt.MaxDepth) }
func parseMindepth(ps *parser, e *parserEntry) bool { return ps.insertDepthspec(e, &ps.r.opt.MinDepth) }

// insertDepthspec is parser.c insert_depthspec with lib/safe-atoi.c.
func (ps *parser) insertDepthspec(e *parserEntry, limit *int) bool {
	option := ps.argv[ps.i-1]
	s, ok := ps.collectArg()
	if !ok {
		return false
	}
	n := len(s) - len(strings.TrimLeft(s, "0123456789"))
	if n > 0 && n == len(s) {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil || v > 1<<31-1 {
			ps.r.fatalf(syscall.ERANGE, "%s", s) // safe_atoi: unquoted, no option name
		}
		*limit = int(v)
		return parseNoop(ps, e)
	}
	ps.r.fatalf(nil, "Expected a positive decimal integer argument to %s, but got %s", option, ps.r.quote(s))
	return false
}

func parseAnd(ps *parser, e *parserEntry) bool {
	p := ps.getNewPredNoArg(e)
	p.fn, p.id, p.typ, p.prec = predAnd, fnAnd, BiOp, AndPrec
	p.needStat, p.needType = false, false
	return true
}

func parseOr(ps *parser, e *parserEntry) bool {
	p := ps.getNewPredNoArg(e)
	p.fn, p.id, p.typ, p.prec = predOr, fnOr, BiOp, OrPrec
	p.needStat, p.needType = false, false
	return true
}

func parseComma(ps *parser, e *parserEntry) bool {
	p := ps.getNewPredNoArg(e)
	p.fn, p.id, p.typ, p.prec = predComma, fnComma, BiOp, CommaPrec
	p.needStat, p.needType = false, false
	p.estSuccessRate = 1.0
	return true
}

func parseNegate(ps *parser, e *parserEntry) bool {
	p := ps.getNewPredChkOp(e, "", false)
	p.fn, p.id, p.typ, p.prec = predNegate, fnNegate, UniOp, NegatePrec
	p.needStat, p.needType = false, false
	return true
}

func parseOpenparen(ps *parser, e *parserEntry) bool {
	p := ps.getNewPredChkOp(e, "", false)
	p.fn, p.id, p.typ, p.prec = predOpenparen, fnOpenParen, OpenParen, NoPrec
	p.needStat, p.needType = false, false
	return true
}

func parseCloseparen(ps *parser, e *parserEntry) bool {
	p := ps.getNewPredNoArg(e)
	p.fn, p.id, p.typ, p.prec = predCloseparen, fnCloseParen, CloseParen, NoPrec
	p.needStat, p.needType = false, false
	return true
}
```
`parse_actions.go`:
```go
package find

// parsePrint is parser.c parse_print.
func parsePrint(ps *parser, e *parserEntry) bool {
	p := ps.insertPrimaryNoArg(e)
	p.sideEffects, p.noDefaultPrint = true, true
	p.needStat, p.needType = false, false
	p.args.out = ps.r.out.stdoutStream
	return true
}

// parsePrint0 is parser.c parse_print0 → insert_fprint(entry, NULL).
func parsePrint0(ps *parser, e *parserEntry) bool {
	p := ps.insertPrimary(e, "", false)
	p.args.out = ps.r.out.stdoutStream
	p.sideEffects, p.noDefaultPrint = true, true
	p.needStat, p.needType = false, false
	p.estSuccessRate = 1.0
	return true
}

// parsePrune is parser.c parse_prune.
func parsePrune(ps *parser, e *parserEntry) bool {
	p := ps.insertPrimaryNoArg(e)
	if !ps.r.opt.DoDirFirst {
		p.needStat, p.needType = false, false
	}
	p.sideEffects, p.noDefaultPrint = true, false
	return true
}

// parseQuit is parser.c parse_quit.
func parseQuit(ps *parser, e *parserEntry) bool {
	p := ps.insertPrimaryNoArg(e)
	p.needStat, p.needType = false, false
	p.sideEffects, p.noDefaultPrint = true, false
	p.estSuccessRate = 1.0
	return true
}
```

- [ ] **Step 7: `tree.go` — `get_expr`, `scan_rest`, `set_new_parent`, `merge_pred`**

```go
package find

// getExpr is tree.c get_expr.
func (r *Run) getExpr(input **Predicate, prevPrec Prec, prevPred *Predicate) *Predicate {
	var next *Predicate
	this := *input
	if this == nil {
		r.fatalf(nil, "invalid expression")
	}
	switch this.typ {
	case NoType:
		r.fatalf(nil, "invalid expression")
	case BiOp:
		r.fatalf(nil, "invalid expression; you have used a binary operator '%s' with nothing before it.", this.name)
	case CloseParen:
		if prevPred == nil {
			r.fatalf(nil, "invalid expression: expected expression before closing parentheses '%s'.", this.name)
		}
		if (prevPred.typ == UniOp || prevPred.typ == BiOp) && !this.artificial {
			r.fatalf(nil, "expected an expression between '%s' and ')'", prevPred.name)
		} else if this.artificial {
			r.fatalf(nil, "expected an expression after '%s'", prevPred.name)
		}
		r.fatalf(nil, "invalid expression; you have too many ')'")
	case PrimaryType:
		next = this
		*input = this.next
	case UniOp:
		next = this
		*input = this.next
		next.right = r.getExpr(input, NegatePrec, next)
	case OpenParen:
		if this.next == nil || this.next.artificial {
			r.fatalf(nil, "invalid expression; expected to find a ')' but didn't see one. Perhaps you need an extra predicate after '%s'", this.name)
		}
		prevPred = this
		*input = this.next
		if (*input).typ == CloseParen {
			if prevPred.artificial {
				r.fatalf(nil, "invalid expression: expected expression before closing parentheses '%s'.", (*input).name)
			}
			r.fatalf(nil, "invalid expression; empty parentheses are not allowed.")
		}
		next = r.getExpr(input, NoPrec, prevPred)
		if *input == nil || (*input).typ != CloseParen {
			r.fatalf(nil, "invalid expression; I was expecting to find a ')' somewhere but did not see one.")
		}
		*input = (*input).next
	default:
		r.fatalf(nil, "oops -- invalid expression type! This is probably a bug.")
	}
	if *input == nil {
		return next
	}
	if (*input).prec > prevPrec {
		next = r.scanRest(input, next, prevPrec)
		if next == nil {
			r.fatalf(nil, "invalid expression")
		}
	}
	return next
}

// scanRest is tree.c scan_rest.
func (r *Run) scanRest(input **Predicate, head *Predicate, prevPrec Prec) *Predicate {
	if *input == nil || (*input).typ == CloseParen {
		return nil
	}
	tree := head
	for *input != nil && (*input).prec > prevPrec {
		switch (*input).typ {
		case NoType, PrimaryType, UniOp, OpenParen:
			r.fatalf(nil, "invalid expression")
		case BiOp:
			prev := *input
			(*input).left = tree
			tree = *input
			*input = (*input).next
			tree.right = r.getExpr(input, tree.prec, prev)
		case CloseParen:
			return tree
		default:
			r.fatalf(nil, "oops -- invalid expression type (%d)!  This is probably a bug.", (*input).typ)
		}
	}
	return tree
}

// setNewParent is tree.c set_new_parent.
func setNewParent(curr *Predicate, high Prec, prevp **Predicate) *Predicate {
	np := &Predicate{typ: BiOp, prec: high, cost: NeedsNothing}
	switch high {
	case CommaPrec:
		np.fn, np.id, np.name, np.estSuccessRate = predComma, fnComma, ",", 1.0
	case OrPrec:
		np.fn, np.id, np.name, np.estSuccessRate = predOr, fnOr, "-o", constrainRate(curr.estSuccessRate)
	case AndPrec:
		np.fn, np.id, np.name, np.estSuccessRate = predAnd, fnAnd, "-a", constrainRate(curr.estSuccessRate)
	}
	np.right = curr
	*prevp = np
	return np
}

// mergePred is tree.c merge_pred.
func mergePred(beg, end *Predicate, lastp **Predicate) {
	end.left = *lastp
	*lastp = beg
}

func constrainRate(rate float32) float32 {
	switch {
	case rate > 1:
		return 1
	case rate < 0:
		return 0
	}
	return rate
}
```

- [ ] **Step 8: `optimise.go` — costs, rates, `opt_expr`, arm swaps (tree.c lines 289–1137, verbatim)**

```go
package find

// costLookup is tree.c costlookup[].
var costLookup = map[FuncID]Cost{
	fnAmin: NeedsStatInfo, fnAnd: NeedsNothing, fnAnewer: NeedsStatInfo, fnAtime: NeedsStatInfo,
	fnCloseParen: NeedsNothing, fnCmin: NeedsStatInfo, fnCnewer: NeedsStatInfo, fnComma: NeedsNothing,
	fnContext: NeedsAccessInfo, fnCtime: NeedsStatInfo, fnDelete: NeedsSyncDiskHit, fnEmpty: NeedsStatInfo,
	fnExec: NeedsEventualExec, fnExecdir: NeedsEventualExec, fnExecutable: NeedsAccessInfo, fnFalse: NeedsNothing,
	fnFprint: NeedsNothing, fnFprint0: NeedsNothing, fnFprintf: NeedsNothing, fnFstype: NeedsStatInfo,
	fnGid: NeedsStatInfo, fnGroup: NeedsStatInfo, fnIlname: NeedsLinkName, fnIname: NeedsNothing,
	fnInum: NeedsInodeNumber, fnIpath: NeedsNothing, fnLinks: NeedsStatInfo, fnLname: NeedsLinkName,
	fnLs: NeedsStatInfo, fnFls: NeedsStatInfo, fnMmin: NeedsStatInfo, fnMtime: NeedsStatInfo,
	fnName: NeedsNothing, fnNegate: NeedsNothing, fnNewer: NeedsStatInfo, fnNewerXY: NeedsStatInfo,
	fnNogroup: NeedsStatInfo, fnNouser: NeedsStatInfo, fnOk: NeedsUserInteraction, fnOkdir: NeedsUserInteraction,
	fnOpenParen: NeedsNothing, fnOr: NeedsNothing, fnPath: NeedsNothing, fnPerm: NeedsStatInfo,
	fnPrint: NeedsNothing, fnPrint0: NeedsNothing, fnPrune: NeedsNothing, fnQuit: NeedsNothing,
	fnReadable: NeedsAccessInfo, fnRegex: NeedsNothing, fnSamefile: NeedsStatInfo, fnSize: NeedsStatInfo,
	fnTrue: NeedsNothing, fnType: NeedsType, fnUid: NeedsStatInfo, fnUsed: NeedsStatInfo,
	fnUser: NeedsStatInfo, fnWritable: NeedsAccessInfo, fnXtype: NeedsType,
}

// getPredCost is tree.c get_pred_cost.
func getPredCost(p *Predicate) Cost {
	data := NeedsNothing
	switch {
	case p.needStat:
		data = NeedsStatInfo
	case p.needInum:
		data = NeedsInodeNumber
	case p.needType:
		data = NeedsType
	}
	inherent := NeedsUnknown
	switch {
	case p.is(fnExec) || p.is(fnExecdir) || p.is(fnOk) || p.is(fnOkdir): // predicate_uses_exec
		inherent = NeedsImmediateExec // Task 8 adds: NeedsEventualExec when p.args.exec.multiple
	case p.is(fnFprintf):
		inherent = p.cost // computed by the -printf parser (Task 7)
	default:
		if c, ok := costLookup[p.id]; ok {
			inherent = c
		}
	}
	return max(inherent, data)
}

// estimateCosts is tree.c estimate_costs.
func estimateCosts(t *Predicate) {
	if t != nil {
		estimateCosts(t.right)
		estimateCosts(t.left)
		t.cost = getPredCost(t)
	}
}

func getRate(p *Predicate) float32 {
	if p == nil {
		return 1.0
	}
	return p.estSuccessRate
}

// calculateDerivedRates is tree.c calculate_derived_rates.
func calculateDerivedRates(p *Predicate) float32 {
	if p.right != nil {
		calculateDerivedRates(p.right)
	}
	if p.left != nil {
		calculateDerivedRates(p.left)
	}
	switch p.typ {
	case UniOp:
		p.estSuccessRate = 1.0 - p.right.estSuccessRate
	case BiOp:
		var rate float32
		switch {
		case p.is(fnAnd):
			rate = getRate(p.right) * getRate(p.left)
		case p.is(fnComma):
			rate = 1.0
		case p.is(fnOr):
			rate = getRate(p.right) + getRate(p.left)
		}
		p.estSuccessRate = constrainRate(rate)
	case OpenParen, CloseParen:
		p.estSuccessRate = 1.0
	}
	return p.estSuccessRate
}

type predlist struct{ head, tail *Predicate }

// predlistInsert is tree.c predlist_insert (prepends).
func predlistInsert(list *predlist, curr *Predicate, pprev **Predicate) {
	*pprev = curr.left
	curr.left = list.head
	list.head = curr
	if list.tail == nil {
		list.tail = list.head
	}
}

// predCostCompare is tree.c pred_cost_compare.
func predCostCompare(p1, p2 *Predicate, wantFailure bool) int {
	if p1.cost == p2.cost {
		switch {
		case p1.estSuccessRate == p2.estSuccessRate:
			return 0
		case wantFailure:
			if p1.estSuccessRate < p2.estSuccessRate {
				return -1
			}
			return 1
		default:
			if p1.estSuccessRate < p2.estSuccessRate {
				return 1
			}
			return -1
		}
	}
	if p1.cost < p2.cost {
		return -1
	}
	return 1
}

// predlistMergeSort is tree.c predlist_merge_sort (an insertion sort).
func (r *Run) predlistMergeSort(list *predlist, last **Predicate) {
	if list.head == nil {
		return
	}
	if r.opt.Debug&DebugTreeOpt != 0 {
		r.debugf("%s:\n", "predlist before merge sort")
		r.printTree(list.head, 2)
	}
	calculateDerivedRates(list.head)
	var nl predlist
	for list.head != nil {
		q := list.head
		list.head = list.head.left
		q.left = nil
		var p *Predicate
		for p = nl.head; p != nil; p = p.left {
			wantFailure := p.prec != OrPrec
			if predCostCompare(p.right, q.right, wantFailure) >= 0 {
				break
			}
		}
		if p != nil {
			q.left = p.left
			if q.left == nil {
				nl.tail = q
			}
			p.left = q
		} else {
			q.left = nl.head
			nl.head = q
			if nl.tail == nil {
				nl.tail = q
			}
		}
	}
	if r.opt.Debug&DebugTreeOpt != 0 {
		r.debugf("%s:\n", "predlist after merge sort")
		r.printTree(nl.head, 2)
	}
	calculateDerivedRates(nl.head)
	mergePred(nl.head, nl.tail, last)
	*list = predlist{}
}

func (r *Run) mergeLists(lists *[numCosts]predlist, nameList, regexList *predlist, last **Predicate) {
	r.predlistMergeSort(nameList, last)
	r.predlistMergeSort(regexList, last)
	for i := range lists {
		r.predlistMergeSort(&lists[i], last)
	}
}

// predicateIsCostFree is tree.c predicate_is_cost_free.
func (r *Run) predicateIsCostFree(p *Predicate) bool {
	if p.is(fnName) || p.is(fnPath) || p.is(fnIname) || p.is(fnIpath) {
		return true
	}
	if r.opt.OptimisationLevel > 0 {
		if p.is(fnAnd) || p.is(fnNegate) || p.is(fnComma) || p.is(fnOr) {
			return false
		}
		return p.cost == NeedsNothing
	}
	return false
}

// optExpr is tree.c opt_expr.
func (r *Run) optExpr(treep **Predicate) bool {
	if treep == nil || *treep == nil {
		return false
	}
	var regexList, nameList predlist
	var cbo [numCosts]predlist
	hasSideEffects := false

	prevp := treep
	prevPrec := AndPrec
	curr := *prevp
	for curr.left != nil {
		prevp = &curr.left
		prevPrec = curr.prec
		curr = curr.left
	}
	if curr.typ != BiOp {
		setNewParent(curr, prevPrec, prevp)
	}
	if r.opt.Debug&(DebugExpressionTree|DebugTreeOpt) != 0 {
		r.debugf("Normalized Eval Tree:\n")
		r.printTree(*treep, 0)
	}

	prevp = treep
	biopPrec := NoPrec
	if *prevp != nil && (*prevp).typ == BiOp {
		biopPrec = (*prevp).prec
	}
	for {
		curr = *prevp
		if curr == nil {
			break
		}
		if curr.typ == BiOp && curr.prec != biopPrec {
			curr = setNewParent(curr, biopPrec, prevp)
		}
		rp := curr.right
		switch rp.typ {
		case NoType, PrimaryType:
			if biopPrec == CommaPrec || rp.sideEffects {
				break
			}
			if r.predicateIsCostFree(rp) {
				if r.opt.Debug&DebugTreeOpt != 0 {
					r.debugf("-O%d: promoting cheap predicate ", r.opt.OptimisationLevel)
					r.printPredicate(rp)
					r.debugf(" into name_list\n")
				}
				predlistInsert(&nameList, curr, prevp)
				continue
			}
			if rp.is(fnRegex) {
				predlistInsert(&regexList, curr, prevp)
				continue
			}
			reorder := (r.opt.OptimisationLevel > 1 &&
				(rp.cost == NeedsType || rp.cost == NeedsInodeNumber) && !rp.needStat) ||
				r.opt.OptimisationLevel > 2
			if reorder {
				if r.opt.Debug&DebugTreeOpt != 0 {
					r.debugf("-O%d: categorising predicate ", r.opt.OptimisationLevel)
					r.printPredicate(rp)
					r.debugf(" by cost (%s)\n", costName(rp.cost))
				}
				predlistInsert(&cbo[rp.cost], curr, prevp)
				continue
			}
		case UniOp:
			rp.sideEffects = r.optExpr(&rp.right)
		case BiOp:
			curr.right.sideEffects = r.optExpr(&curr.right)
		default:
			r.fatalf(nil, "oops -- invalid expression type!  This is probably a bug.")
		}
		if curr.right.sideEffects {
			r.mergeLists(&cbo, &nameList, &regexList, prevp)
			hasSideEffects = true
		}
		prevp = &curr.left
	}
	r.mergeLists(&cbo, &nameList, &regexList, prevp)
	return hasSideEffects
}
```
**Port check.** In C, `break` inside the `switch` falls to the side-effects test, and `continue` skips `prevp = &curr->pred_left` because `predlist_insert` already unlinked `curr`. The Go `break` inside `case NoType, PrimaryType:` does the same. `estimateCosts` must run before `optExpr` (see `buildExpressionTree`).

```go
func subtreeHasSideEffects(p *Predicate) bool {
	return p != nil && (p.sideEffects || subtreeHasSideEffects(p.left) || subtreeHasSideEffects(p.right))
}

func worstCost(p *Predicate) Cost {
	if p == nil {
		return 0
	}
	return max(worstCost(p.left), worstCost(p.right), p.cost)
}

// considerArmSwap is tree.c consider_arm_swap.
func (r *Run) considerArmSwap(p *Predicate) bool {
	reason := ""
	var pl, pr **Predicate
	switch {
	case p.typ != BiOp:
		reason = "Not a binary operation"
	case p.left == nil || p.right == nil:
		reason = "Doesn't have two arms"
	case p.left.right == nil:
		reason = "Left arm has no child on RHS"
	}
	if reason == "" {
		pr, pl = &p.right, &p.left.right
		if subtreeHasSideEffects(*pl) {
			reason = "Left subtree has side-effects"
		} else if subtreeHasSideEffects(*pr) {
			reason = "Right subtree has side-effects"
		}
	}
	var lc, rc Cost
	if reason == "" {
		lc, rc = worstCost(*pl), worstCost(*pr)
		if lc < rc {
			reason = "efficient as-is"
		}
	}
	if reason == "" {
		want := true
		if lc == rc {
			sl, sr := (*pl).estSuccessRate, (*pr).estSuccessRate
			if r.opt.Debug&DebugTreeOpt != 0 {
				r.debugf("Success rates: l=%f, r=%f\n", sl, sr)
			}
			switch {
			case p.is(fnOr):
				want = sr < sl
				if !want {
					reason = "Operation is OR; right success rate >= left"
				}
			case p.is(fnAnd):
				want = sr > sl
				if !want {
					reason = "Operation is AND; right success rate <= left"
				}
			default:
				want, reason = false, "Not 'AND' or 'OR'"
			}
		}
		if want {
			if r.opt.Debug&DebugTreeOpt != 0 {
				r.debugf("Performing arm swap on:\n")
				r.printTree(p, 0)
			}
			tmp := p.left.right
			p.left.right = p.right
			p.right = tmp
			return true
		}
	}
	if r.opt.Debug&DebugTreeOpt != 0 {
		r.debugf("Not an arm swap candidate (%s):\n", reason)
		r.printTree(p, 0)
	}
	return false
}

// doArmSwaps is tree.c do_arm_swaps.
func (r *Run) doArmSwaps(p *Predicate) bool {
	if p == nil {
		return false
	}
	for {
		swapped := r.considerArmSwap(p) || r.doArmSwaps(p.left) || r.doArmSwaps(p.right)
		if !swapped {
			return false
		}
	}
}
```
(`do_arm_swaps` returns `swapped` after the loop, which is always false. The Go form above returns false the same way.)

- [ ] **Step 9: `debugtree.go` — `print_tree`, `print_list`, `print_optlist`, names, success rates**

```go
package find

import (
	"fmt"
	"strings"
)

var costNames = [...]string{"Nothing", "InodeNumber", "Type", "StatInfo", "LinkName",
	"AccessInfo", "SyncDiskHit", "EventualExec", "ImmediateExec", "UserInteraction", "Unknown"}

func costName(c Cost) string {
	if int(c) < len(costNames) {
		return costNames[c]
	}
	return "unknown"
}

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

// printTree is tree.c print_tree.
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

// printList is pred.c print_list.
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
```
C `%.4g` and `%#.4g` match Go's `fmt` (`%#g` keeps trailing zeros) for every finite value. Floats are passed as float64, just as C promotes them.

- [ ] **Step 10: `cfile.go` and `output.go` — glibc stdio emulation, streams and per-entry transactions**

GNU's output diagnostics depend on which stdio call sees a failed `write(2)`:
- `checked_fprintf` reports per call;
- `-ls` dies at a numbered stage;
- `cleanup()` reports `'standard output'` only when its own `fflush` fails;
- `close_stdout` says `write error` without errno after an earlier failure.

On a terminal, glibc line-buffers stdout, so results appear as they are found. qfind therefore emulates a glibc `FILE` instead of using `bufio`:
- a buffer of `min(BUFSIZ, st_blksize)` bytes;
- `_IO_new_file_xsputn`'s fill, flush, block-write, buffer-the-remainder sequence;
- a failed flush that empties the buffer and sets the error flag;
- line buffering on a terminal.

`cfile.go`:
```go
package find

import (
	"bytes"
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// cfile emulates a glibc stdio FILE open for writing (libio/fileops.c).
// It is not safe for concurrent use; the sink that owns it holds a lock.
type cfile struct {
	f       *os.File
	buf     []byte
	size    int   // _IO_file_doallocate: BUFSIZ, or st_blksize when smaller
	lineBuf bool  // a terminal: _IO_LINE_BUF
	err     error // first failed write(2): ferror()
}

func newCfile(f *os.File) *cfile {
	c := &cfile{f: f, size: 8192}
	var st unix.Stat_t
	if unix.Fstat(int(f.Fd()), &st) == nil {
		if st.Mode&unix.S_IFMT == unix.S_IFCHR && isatty(int(f.Fd())) {
			c.lineBuf = true
		}
		if st.Blksize > 0 && st.Blksize < 8192 {
			c.size = int(st.Blksize)
		}
	}
	c.buf = make([]byte, 0, c.size)
	return c
}

// write is _IO_new_file_xsputn, the output path of fwrite, fputs and
// fprintf. It returns an error when this call's write(2) failed: exactly
// when the C call returns EOF or a short count.
func (c *cfile) write(p []byte) error {
	toDo := len(p)
	space := c.size - len(c.buf)
	count, mustFlush := space, false
	if c.lineBuf && space >= toDo {
		if i := bytes.LastIndexByte(p, '\n'); i >= 0 {
			count, mustFlush = i+1, true
		}
	}
	count = min(count, toDo)
	c.buf = append(c.buf, p[:count]...)
	p, toDo = p[count:], toDo-count
	if toDo == 0 && !mustFlush {
		return nil
	}
	if err := c.flushBuf(); err != nil {
		return err // the rest of this call's data is dropped, as in glibc
	}
	doWrite := toDo
	if c.size >= 128 {
		doWrite -= toDo % c.size // write whole blocks directly
	}
	if doWrite > 0 {
		if err := c.sysWrite(p[:doWrite]); err != nil {
			return err
		}
		p = p[doWrite:]
	}
	c.buf = append(c.buf, p...) // _IO_default_xsputn: the remainder fits
	return nil
}

// flushBuf is _IO_do_write of the buffer (fflush, _IO_OVERFLOW): the
// buffer is emptied whether or not the write succeeds.
func (c *cfile) flushBuf() error {
	if len(c.buf) == 0 {
		return nil
	}
	err := c.sysWrite(c.buf)
	c.buf = c.buf[:0]
	return err
}

// sysWrite is _IO_new_file_write: write everything or fail, setting the
// error flag. os.File.Write loops over short writes and retries EINTR; a
// broken pipe on fd 1 or 2 kills the process with SIGPIPE, as in C.
func (c *cfile) sysWrite(p []byte) error {
	_, err := c.f.Write(p)
	if err == nil {
		return nil
	}
	if c.err == nil {
		c.err = err
	}
	var pe *os.PathError
	if errors.As(err, &pe) {
		err = pe.Err
	}
	return err
}
```

`output.go`:
```go
package find

import (
	"errors"
	"os"
	"sync"

	"golang.org/x/sys/unix"

	"qfind/internal/libc"
)

type streamKind uint8

const (
	streamStdout streamKind = iota
	streamStderr
	streamFile
)

// sink is a locked stdio stream: stdout, or a shared output file (Task 7).
// write and flush return this call's error, like the C call's return
// value; callers hold lock() around them.
type sink interface {
	lock()
	unlock()
	write(b []byte) error
	flush() error
}

// Stream is format_val's destination: FILE*, filename and dest_is_tty.
type Stream struct {
	name string // dest->filename: "standard output", "standard error", or the path as given
	kind streamKind
	tty  bool
	sink sink // nil for stderr, which is unbuffered and written at once
}

// Output owns stdout and, from Task 7, the shared output files.
type Output struct {
	r            *Run
	mu           sync.Mutex // guards f
	f            *cfile
	stdoutStream *Stream
	stderrStream *Stream
}

func newOutput(r *Run) *Output {
	o := &Output{r: r, f: newCfile(os.Stdout)}
	o.stdoutStream = &Stream{name: "standard output", kind: streamStdout, tty: isatty(1), sink: o}
	o.stderrStream = &Stream{name: "standard error", kind: streamStderr, tty: isatty(2)}
	return o
}

func (o *Output) lock()                { o.mu.Lock() }
func (o *Output) unlock()              { o.mu.Unlock() }
func (o *Output) write(b []byte) error { return o.f.write(b) }
func (o *Output) flush() error         { return o.f.flushBuf() }

// failMode is what a failed write of a queued call does, following how
// GNU checks the stdio call that produced it.
type failMode uint8

const (
	failIgnored  failMode = iota // print_quoted, fputs, putc: unchecked (-print, -print0, -fprint)
	failReported                 // checked_fprintf/fwrite/fflush: diagnose, exit status 1 (-printf)
	failFatal                    // list_file: "Failed to write output (at stage N)" (-ls)
)

// piece is one stdio call of a rendered record; stage is list_file's
// failed_at number for that call.
type piece struct {
	text  string
	stage int
}

// pending is one queued stdio call, or (render) one -ls record whose calls
// are produced at commit time: listfile.c's column widths are shared by
// every entry and must grow in output order.
type pending struct {
	s      *Stream
	text   string
	fail   failMode
	render func() []piece
	done   bool
}

// txn is one entry's pending output. Committing it atomically keeps an
// entry's records together, as GNU's one-entry-at-a-time evaluation does.
type txn struct{ items []pending }

// emit queues one unchecked call (print_quoted, fputs) for s on e.
func (r *Run) emit(e *Entry, s *Stream, text string) { r.emitChecked(e, s, text, failIgnored) }

// emitChecked queues one call whose failure is handled per fail. stderr
// is unbuffered in C, so its text is written immediately, in program order
// with diagnostics.
func (r *Run) emitChecked(e *Entry, s *Stream, text string, fail failMode) {
	if s.sink == nil {
		if err := r.writeStderr(text); err != nil && fail == failReported {
			r.reportWriteError(nil, s, err)
		}
		return
	}
	e.txn.items = append(e.txn.items, pending{s: s, text: text, fail: fail})
}

// emitRendered queues a record whose calls are produced at commit time.
func (r *Run) emitRendered(e *Entry, s *Stream, render func() []piece) {
	if s.sink == nil {
		for _, pc := range render() {
			if err := r.writeStderr(pc.text); err != nil {
				r.fatalf(err, "Failed to write output (at stage %d)", pc.stage)
			}
		}
		return
	}
	e.txn.items = append(e.txn.items, pending{s: s, fail: failFatal, render: render})
}

// emitQuoted is lib/printquoted.c print_quoted(fp, …, "%s<suffix>", name).
func (r *Run) emitQuoted(e *Entry, s *Stream, name, suffix string) {
	if s.tty {
		name = r.loc.Qmark(name)
	}
	r.emit(e, s, name+suffix)
}

// commit hands e's queued calls to their streams. Each sink receives all
// of e's calls for it, in order, under one lock, so no other entry's
// record lands between two of e's; the order between sinks is not
// observable.
func (r *Run) commit(e *Entry) {
	items := e.txn.items
	for i := range items {
		if items[i].done {
			continue
		}
		sk := items[i].s.sink
		sk.lock()
		for j := i; j < len(items); j++ {
			it := &items[j]
			if it.done || it.s.sink != sk {
				continue
			}
			it.done = true
			if it.render != nil {
				for _, pc := range it.render() {
					if err := sk.write([]byte(pc.text)); err != nil {
						sk.unlock() // exit() flushes every stream: never die holding one
						r.fatalf(err, "Failed to write output (at stage %d)", pc.stage)
					}
				}
				continue
			}
			if err := sk.write([]byte(it.text)); err != nil && it.fail == failReported {
				r.reportWriteError(sk, it.s, err)
			}
		}
		sk.unlock()
	}
	e.txn.items = items[:0]
}

// reportWriteError is nonfatal_nontarget_file_error(errno, dest->filename)
// for a checked call, issued while sink held (or none) is locked. Like
// error(), it flushes stdout first; stdout's lock may be the one held.
func (r *Run) reportWriteError(held sink, s *Stream, err error) {
	o := r.out
	if held != sink(o) {
		o.mu.Lock()
		defer o.mu.Unlock()
	}
	o.f.flushBuf()
	r.writeStderr(r.prog + ": " + r.quote(s.name) + ": " + libc.Strerror(err) + "\n")
	r.setFailure()
}

// flushStream is fflush(fp): unchecked at launch() and the -ok prompt
// (stdout), checked_fflush at \c (the format's own stream).
func (r *Run) flushStream(e *Entry, s *Stream, fail failMode) {
	if e != nil {
		r.commit(e)
	}
	if s.sink == nil {
		return
	}
	s.sink.lock()
	defer s.sink.unlock()
	if err := s.sink.flush(); err != nil && fail == failReported {
		r.reportWriteError(s.sink, s, err)
	}
}

// flushStdout is the unchecked fflush(stdout) before a child or a prompt.
func (r *Run) flushStdout(e *Entry) { r.flushStream(e, r.out.stdoutStream, failIgnored) }

// fatalAt is fatalf for an error raised while evaluating e. GNU's stdout
// buffer already holds e's earlier output and exit() flushes it, so commit
// e's transaction first.
func (r *Run) fatalAt(e *Entry, err error, format string, a ...any) {
	if e != nil {
		r.commit(e)
	}
	r.fatalf(err, format, a...)
}

// flushForDiagnostic is the fflush(stdout) inside glibc error().
func (o *Output) flushForDiagnostic() {
	o.mu.Lock()
	o.f.flushBuf()
	o.mu.Unlock()
}

// flushFilesQuietly is exit()'s flush of every other FILE; Task 7 fills it.
func (o *Output) flushFilesQuietly() {}

// WriteString writes usage, version and -D help text (no entry).
func (o *Output) WriteString(s string) {
	o.mu.Lock()
	o.f.write([]byte(s))
	o.mu.Unlock()
}

var errPrevFailed = errors.New("write error after earlier failure")

// cleanupFlush is util.c cleanup()'s fflush(stdout): it fails only when
// this flush's write fails; a buffer emptied by an earlier failure flushes
// fine, and close_stdout reports that one.
func (r *Run) cleanupFlush() {
	o := r.out
	o.mu.Lock()
	err := o.f.flushBuf()
	o.mu.Unlock()
	if err != nil {
		r.errorf(err, "%s", r.quote("standard output"))
		r.setFailure()
	}
}

// closeAll is close_stdout's close_stream(stdout): fclose flushes and
// closes fd 1. An earlier failure gives "write error" without errno, a
// failure now gives its errno, and EBADF with nothing pending (stdout
// closed but never written) is no error.
func (o *Output) closeAll() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	pending := len(o.f.buf) > 0
	prevFail := o.f.err != nil
	err := o.f.flushBuf()
	if cerr := unix.Close(1); err == nil {
		err = cerr
	}
	switch {
	case err == nil && prevFail:
		return errPrevFailed
	case err != nil && (prevFail || pending || err != unix.EBADF):
		return err
	}
	return nil
}
```
`diag.go`'s `exit` (Task 2) already prints `write error` without an errno suffix when `closeAll` returns `errPrevFailed`.

`pred_actions.go`:
```go
package find

// predPrint is pred.c pred_print.
func predPrint(r *Run, e *Entry, p *Predicate) bool {
	r.emitQuoted(e, p.args.out, e.path, "\n")
	return true
}

// predPrint0 is pred.c pred_fprint0: never quoted.
func predPrint0(r *Run, e *Entry, p *Predicate) bool {
	r.emit(e, p.args.out, e.path+"\x00")
	return true
}

// predPrune is pred.c pred_prune.
func predPrune(r *Run, e *Entry, p *Predicate) bool {
	if r.opt.DoDirFirst && e.haveStat && e.st.Mode&unixS_IFMT == unixS_IFDIR {
		e.prune = true
	}
	return true
}

// predQuit is pred.c pred_quit: cleanup, then exit with the current status.
// Evaluation is serialised when -quit is present (Task 4), so nothing else
// can print after this point.
func predQuit(r *Run, e *Entry, p *Predicate) bool {
	r.commit(e)
	r.cleanup()
	r.exit(int(r.exitStatus.Load()))
	return true
}
```
(`unixS_IFMT`/`unixS_IFDIR` are `const unixS_IFMT = unix.S_IFMT` etc. in `entry.go`.)

- [ ] **Step 11: `entry.go` and `traverse.go` — per-file state; starting points at depth 0**

```go
package find

import "golang.org/x/sys/unix"

const (
	unixS_IFMT  = unix.S_IFMT
	unixS_IFDIR = unix.S_IFDIR
	unixS_IFLNK = unix.S_IFLNK
	unixS_IFREG = unix.S_IFREG
)

// Entry is GNU's per-file `state` plus the fts path fields.
type Entry struct {
	path          string // fts_path, as printed and quoted in messages
	sysPath       string // the path every system call uses: fastwalk's, always shorter than PATH_MAX (Task 15)
	name          string // fts_name: the directory entry's name; the argument at depth 0
	accPath       string // fts_accpath = state.rel_pathname: name at depth >= 1 (full path under -L), the argument at 0
	depth         int    // fts_level
	haveStat      bool
	haveType      bool
	mode          uint32 // state.type (S_IFMT bits, full st_mode once stat'ed)
	st            unix.Stat_t
	issuedStatErr bool // already_issued_stat_error_msg
	prune         bool // stop_at_current_level
	txn           txn
}

// getInfo is util.c get_info.
func (r *Run) getInfo(e *Entry, p *Predicate) bool {
	todo := false
	switch {
	case p.needStat && !e.haveStat:
		todo = true
	case p.needType && !e.haveType:
		todo = true
	case p.needInum:
		todo = !e.haveStat // fastwalk exposes no d_ino, so stat for the inode
	}
	if todo {
		return r.getStatinfo(e)
	}
	return true
}

// getStatinfo is util.c get_statinfo.
func (r *Run) getStatinfo(e *Entry) bool {
	if !e.haveStat {
		st, err := r.xstat(e)
		if err != nil {
			if !r.opt.IgnoreReaddirRace || !isENOENT(err) {
				r.nonfatalTargetFileError(e, err)
			}
			return false
		}
		if st.Mode == 0 {
			r.errorf(nil, "WARNING: file %s appears to have mode 0000", r.quote(e.accPath)) // GNU quotes `name`
			r.setFailure()
		}
		e.st = st
	}
	e.haveStat, e.haveType = true, true
	e.mode = e.st.Mode
	return true
}

// xstat is options.xstat for an entry: the syscall uses the full path
// (qfind has no fts cwd), while -D stat shows fts_accpath as GNU does.
func (r *Run) xstat(e *Entry) (unix.Stat_t, error) {
	return r.xstatName(e.sysPath, e.accPath, e.depth)
}

// xstatName is options.xstat — optionp_stat, optionh_stat or optionl_stat,
// wrapped by debug_stat under -D stat (util.c). depth is state.curdepth:
// 0 for command-line arguments such as -newer's reference file. Only these
// stats print debug_stat; the traversal's own stats (fts's) never do.
func (r *Run) xstatName(path, shown string, depth int) (unix.Stat_t, error) {
	var st unix.Stat_t
	if r.opt.Debug&DebugStat != 0 {
		r.debugf("debug_stat (%s)\n", shown)
	}
	follow := r.opt.Symlinks == SymlinkAlwaysDeref ||
		(r.opt.Symlinks == SymlinkDerefArgsOnly && depth == 0)
	if !follow {
		return st, unix.Lstat(path, &st)
	}
	err := unix.Stat(path, &st)
	if err == unix.ENOENT || err == unix.ENOTDIR { // fallback_stat
		if r.opt.Debug&DebugStat != 0 {
			r.debugf("fallback_stat(): stat(%s) failed; falling back on lstat()\n", shown)
		}
		return st, unix.Lstat(path, &st)
	}
	return st, err
}

// nonfatalTargetFileError is util.c report_file_err(0, errno, true, path):
// one message per entry.
func (r *Run) nonfatalTargetFileError(e *Entry, err error) {
	if !e.issuedStatErr {
		r.errorf(err, "%s", r.quote(e.path))
		r.setFailure()
	}
	e.issuedStatErr = true
}

func isENOENT(err error) bool { return err == unix.ENOENT }
```
`traverse.go` (Task 4 replaces `findRoot` with the fastwalk walker):
```go
package find

import "golang.org/x/sys/unix"

// processAllStartPoints is ftsfind.c process_all_startpoints (argv form;
// -files0-from arrives in Task 4).
func (r *Run) processAllStartPoints() bool {
	if len(r.startPoints) == 0 {
		return r.findRoot(".")
	}
	for _, name := range r.startPoints {
		if name == "" {
			r.errorf(unix.ENOENT, "%s", r.quote(name))
			r.setFailure()
			continue
		}
		if !r.findRoot(name) {
			return false
		}
	}
	return true
}

// findRoot evaluates a starting point at depth 0 (descent: Task 4).
func (r *Run) findRoot(arg string) bool {
	e := &Entry{path: arg, sysPath: arg, name: arg, accPath: arg}
	st, err := r.xstat(e)
	if err != nil {
		r.nonfatalTargetFileError(e, err)
		return true
	}
	e.st, e.haveStat, e.haveType, e.mode = st, true, true, st.Mode
	if r.opt.MinDepth <= 0 {
		r.visit(e)
	}
	return true
}

// visit is ftsfind.c visit.
func (r *Run) visit(e *Entry) {
	r.apply(e, r.tree)
	r.commit(e)
}

// cleanup is util.c cleanup (Tasks 7–8 add exec batches and shared files).
func (r *Run) cleanup() {
	r.cleanupFlush()
}
```
`main.go` now runs:
1. `r.tree = r.buildExpressionTree(args, end)`;
2. `r.countRates = r.opt.Debug&DebugSuccessRates != 0`;
3. `if r.processAllStartPoints() { r.showSuccessRates(r.tree); r.cleanup() }`;
4. `r.exit(int(r.exitStatus.Load()))`.

Delete the Task 2 help shim.

- [ ] **Step 12: Build and run the gates**

```bash
bundle exec rake qfind:build
bundle exec rspec spec/find/operators spec/find/expressions_spec.rb spec/find/positional_options/warn_spec.rb spec/find/global_options/placement_spec.rb spec/find/options spec/find/tests/true_spec.rb spec/find/tests/false_spec.rb spec/find/actions/print_spec.rb spec/find/actions/print0_spec.rb --format progress 2>&1 | grep -E '^[0-9]+ examples?, '
```
Expected:
- Every "a rejected command line" example passes.
- Every example whose fixture is a single file or starting point passes.
- The remaining failures need descent (Task 4) or a later primary named in the failure (`-name`, `-type`, `-printf`, `-exec`, …).

List them with `ruby .scratch/failures.rb`. Spot-check by hand:
```bash
./dist/find -D opt -O3 . -size 0 -type f -name file 2>&1 >/dev/null | grep -c 'promoting cheap'   # 0 until Tasks 5–11 parse those primaries
./dist/find file -printf x 2>&1; ./dist/find . -print ')' ; ./dist/find . -a
```

- [ ] **Step 13: Review Focus 4 differential check**

`.scratch/diff/pipes.sh`:
```bash
#!/bin/bash
set -u
export LC_ALL=C   # the suite's locale; the login shell exports en_US.UTF-8
cd "$(mktemp -d -p /workspace/qfind.master/.scratch)"
mkdir -p d && touch d/{1..2000}
for f in /workspace/qfind.master/findutils-4.11.0/find/find /workspace/qfind.master/dist/find; do
  echo "== $f"
  (exec -a find "$f" d -print > /dev/full); echo "exit=$?"   # subshell: a bare exec would replace this script
  exec -a find "$f" d -print | head -1 >/dev/null; echo "pipestatus=${PIPESTATUS[0]}"   # no subshell: PIPESTATUS must see find itself
done
```
Expected: both binaries print the same two lines for `/dev/full` (`find: 'standard output': No space left on device`, then `find: write error`), with `exit=1`, and the same pipe status (141).

- [ ] **Step 14: Commit**

```bash
gofmt -l internal; go vet ./...
git add internal
git commit -m "qfind: parser, expression tree, optimiser and output transactions (port of parser.c/tree.c)"
```

---

## Task 4: Traversal on fastwalk — starting points, fts semantics, symlinks, cycles, `-depth`, `-files0-from`

This task replaces `ftsfind.c`'s fts loop with fastwalk while reproducing every `consider_visiting()` branch.

**How it maps onto fastwalk:**
- qfind classifies each starting point itself (`fts_open`/`fts_stat` for level 0) and visits it in pre-order before calling `fastwalk.Walk`. The depth-0 callback is therefore a no-op, except for reporting a read error on the root.
- Each callback synthesises the fts state (`FTS_D`, `FTS_DC`, `FTS_DNR`, `FTS_ERR`, `FTS_F`, `FTS_NS`, `FTS_NSOK`, `FTS_SL`, `FTS_SLNONE`) and runs a faithful port of `consider_visiting()`.
- Stats fts makes itself print nothing under `-D stat`; only `options.xstat` stats do.
- Directory cycles are detected on the active-ancestor (dev, ino) chain.
- `-L` descends through symlinks by returning `fastwalk.ErrTraverseLink`, with `fts_accpath` equal to the full path (`FTS_NOCHDIR`).
- `-depth` evaluates files inline and defers directories.
- `consider_visiting` runs under one lock when `needsSerial()` holds.

**Files:**
- Modify: `internal/find/traverse.go` (replace `findRoot`), `internal/find/entry.go` (fts info, `startLen`, `cancelled`), `internal/find/run.go` (`serial`, `evalMu`, `workers`, `prevDepth`, `formatFlushes`), `internal/find/main.go` (`needsSerial`)
- Create: `internal/find/visit.go`, `internal/find/cycles.go`, `internal/find/depth.go`, `internal/find/files0.go`, `internal/find/workers.go`, `internal/find/paths.go`
- Create (git-ignored): `.scratch/diff/many.sh`, `.scratch/diff/huge.sh`, `.scratch/diff/deep.sh`

**Interfaces:**
- Consumes:
  - Task 3: `Entry`, `apply`, `visit`, `commit`, `getStatinfo`, `xstat`, `nonfatalTargetFileError`, `predQuit`, `cleanup`.
  - `fastwalk.Walk(conf *fastwalk.Config, root string, fn fs.WalkDirFunc) error`, `fastwalk.DirEntry` (`Type()`, `Name()`, `Depth()`), `fastwalk.SkipDir`, `fastwalk.ErrTraverseLink`.
- Produces:
  - `type ftsInfo uint8` with constants `ftsD, ftsDC, ftsDNR, ftsDP, ftsERR, ftsF, ftsNS, ftsNSOK, ftsSL, ftsSLNONE` (names for `-D search`).
  - `(*Run).findRoot(arg string) bool`
  - `(*Run).afterRoot()` (Task 8 flushes `-execdir +` batches here)
  - `Entry.dirPath()` (the directory containing the entry, for `-execdir`), `Entry.startLen`
  - `r.serial bool`, `r.evalMu sync.Mutex`, `r.workers int`, `r.formatFlushes bool`

- [ ] **Step 1: Record the failing gates**

```bash
bundle exec rspec spec/find/traversal_spec.rb spec/find/starting_points_spec.rb spec/find/global_options spec/find/options/symbolic_links_spec.rb spec/find/positional_options/follow_spec.rb spec/find/actions/prune_spec.rb spec/find/actions/quit_spec.rb spec/find/operators spec/find/tests/true_spec.rb spec/find/tests/false_spec.rb --format progress 2>&1 | grep -E '^[0-9]+ examples?, '
```

- [ ] **Step 2: `paths.go` — fts path spelling over fastwalk paths**

```go
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
```

- [ ] **Step 3: `cycles.go` — `FTS_TIGHT_CYCLE_CHECK` over active ancestors**

```go
package find

import "sync"

// dirNode is one directory on the active path: (dev, ino) plus its parent.
// fastwalk gives no "directory finished" event, so nodes live until the
// start point is done (memory O(directories), never O(files)).
type dirNode struct {
	dev, ino uint64
	parent   *dirNode
	entry    *Entry // the directory itself: read errors report it, -depth visits it
}

type dirTree struct{ m sync.Map } // fastwalk dir path → *dirNode

func (t *dirTree) add(key string, n *dirNode) { t.m.Store(key, n) }

func (t *dirTree) get(key string) *dirNode {
	if v, ok := t.m.Load(key); ok {
		return v.(*dirNode)
	}
	return nil
}

// isCycle reports whether (dev, ino) is an active ancestor (fts FTS_DC).
// Only ancestors count: noleaf_spec requires a directory reached through
// two non-ancestor paths to be walked twice.
func isCycle(parent *dirNode, dev, ino uint64) bool {
	for n := parent; n != nil; n = n.parent {
		if n.dev == dev && n.ino == ino {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: `workers.go` — fd-aware parallelism**

```go
package find

import (
	"github.com/charlievieth/fastwalk"
	"golang.org/x/sys/unix"
)

// numWorkers caps fastwalk's workers so that each worker's open directory
// fd, plus stdio, the runtime and one child at a time, fits RLIMIT_NOFILE
// (execdir_spec runs with rlimit_nofile 30). QFIND_WORKERS overrides it
// for benchmarking.
func (r *Run) numWorkers() int {
	if v, ok := r.lookupEnv("QFIND_WORKERS"); ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	n := fastwalk.DefaultNumWorkers()
	var rl unix.Rlimit
	if unix.Getrlimit(unix.RLIMIT_NOFILE, &rl) == nil && rl.Cur != unix.RLIM_INFINITY {
		n = min(n, max(1, (int(rl.Cur)-16)/2))
	}
	return n
}
```
(Add the `strconv` import.)

- [ ] **Step 5: `entry.go` additions — fts info and helpers**

```go
// ftsInfo is the fts_info value consider_visiting switches on.
type ftsInfo uint8

const (
	ftsD ftsInfo = iota
	ftsDC
	ftsDNR
	ftsDP
	ftsERR
	ftsF
	ftsNS
	ftsNSOK
	ftsSL
	ftsSLNONE
)

var ftsInfoNames = [...]string{"FTS_D", "FTS_DC", "FTS_DNR", "FTS_DP", "FTS_ERR",
	"FTS_F", "FTS_NS", "FTS_NSOK", "FTS_SL", "FTS_SLNONE"}

// setType records a d_type-derived type without a stat (FTS_NSOK).
func (e *Entry) setType(m fs.FileMode) {
	switch {
	case m.IsDir():
		e.mode = unix.S_IFDIR
	case m&fs.ModeSymlink != 0:
		e.mode = unix.S_IFLNK
	case m&fs.ModeNamedPipe != 0:
		e.mode = unix.S_IFIFO
	case m&fs.ModeSocket != 0:
		e.mode = unix.S_IFSOCK
	case m&fs.ModeCharDevice != 0:
		e.mode = unix.S_IFCHR
	case m&fs.ModeDevice != 0:
		e.mode = unix.S_IFBLK
	default:
		e.mode = unix.S_IFREG
	}
	e.haveType = true
}

func (e *Entry) setStat(st *unix.Stat_t) {
	e.st, e.haveStat, e.haveType, e.mode = *st, true, true, st.Mode
}

func (e *Entry) isDir() bool { return e.mode&unix.S_IFMT == unix.S_IFDIR }

// dirPath is the working directory -execdir uses for the entry: its parent
// directory below a start point (record_exec_dir's directory relative to
// fts's cwd), gnulib mdir_name(argument) for the start point itself. It is
// a system-call path: below a start point it comes from sysPath, which
// fastwalk builds as parent + "/" + name.
func (e *Entry) dirPath() string {
	if e.depth > 0 {
		d := strings.TrimRight(e.sysPath[:len(e.sysPath)-len(e.name)], "/")
		if d == "" {
			return "/"
		}
		return d
	}
	return mdirName(e.path)
}

// mdirName is gnulib mdir_name: the directory part without trailing
// slashes, "." when there is none, "/" for the root ("/" → "/",
// "tmp/top/" → "tmp", "a" → ".", "/a" → "/", "a//b" → "a").
func mdirName(p string) string {
	end := len(p)
	for end > 1 && p[end-1] == '/' {
		end--
	}
	q := p[:end]
	i := strings.LastIndexByte(q, '/')
	switch {
	case i < 0:
		return "."
	case i == 0:
		return "/"
	}
	for i > 1 && q[i-1] == '/' {
		i--
	}
	return q[:i]
}
```
Add two fields to `Entry` (Task 3's `entry.go`):
- `startLen int`, the length of the starting point argument (`state.starting_path_length`, used by `%H` and `%P` in Task 7). Children copy it from the root.
- `cancelled atomic.Bool`, set when fts would never return the directory's `FTS_DP`: on `FTS_ERR`, and on an ENOENT read race under `-ignore_readdir_race`. A cancelled deferred directory is not visited post-order.

Add to `Run` (`run.go`):
- `serial bool`
- `evalMu sync.Mutex`
- `workers int`
- `prevDepth atomic.Int64` (ftsfind.c `prev_depth`; `Main` stores `math.MinInt32`)
- `formatFlushes bool` (set by Task 7 when a format contains `\c`)

- [ ] **Step 6: `traverse.go` — `find()` for one start point**

```go
package find

import (
	"github.com/charlievieth/fastwalk"
	"golang.org/x/sys/unix"
)

// walker is the state of one fastwalk.Walk below a start point (Task 15
// adds walks re-rooted at /proc/self/fd/N for very deep directories).
type walker struct {
	r         *Run
	rp        rootPaths
	root      *Entry // the directory this Walk starts at
	baseDepth int    // fts_level of root: 0 for a start point
	rootDev   uint64 // the start point's device (-xdev, -mount)
	dirs      *dirTree
	defers    *deferredDirs
}

// findRoot is ftsfind.c find(): classify and visit the start point, then
// let fastwalk walk below it. A start point that is not a directory to
// descend never reaches fastwalk (-files0-from may supply 100,000 of them).
func (r *Run) findRoot(arg string) bool {
	w := &walker{r: r, rp: newRootPaths(arg), dirs: &dirTree{}, defers: &deferredDirs{}}
	root := &Entry{path: arg, sysPath: arg, name: arg, accPath: arg, startLen: len(arg)}
	w.root = root
	info, err := r.rootInfo(root)
	if w.considerRoot(root, info, err) {
		w.dirs.add(w.rp.fw, &dirNode{dev: root.st.Dev, ino: root.st.Ino, entry: root})
		w.rootDev = root.st.Dev
		w.walk()
	}
	w.runDeferred()
	r.afterRoot()
	return true
}

// walk runs fastwalk over w.rp's root. The callbacks never return an error
// that aborts the walk, so an error here is Walk's own os.Stat of the
// root, which lost a race with a rename: report it as a read error.
func (w *walker) walk() {
	conf := &fastwalk.Config{NumWorkers: w.r.workers, Sort: fastwalk.SortNone}
	if w.r.opt.MaxDepth > 0 {
		conf.MaxDepth = w.r.opt.MaxDepth - w.baseDepth // fastwalk counts from its own root
	}
	if err := fastwalk.Walk(conf, w.rp.fw, w.callback); err != nil {
		w.readError(w.root, err)
	}
}

// rootInfo is fts_open/fts_stat for a level-0 entry: lstat under -P;
// stat with FTS_COMFOLLOW under -H/-L, where only ENOENT falls back to
// lstat, as FTS_SLNONE (fts_stat, not util.c fallback_stat: ENOTDIR,
// EACCES and ELOOP are FTS_NS). fts's stats never print debug_stat.
func (r *Run) rootInfo(e *Entry) (ftsInfo, error) {
	var st unix.Stat_t
	if r.opt.Symlinks == SymlinkNeverDeref {
		if err := unix.Lstat(e.sysPath, &st); err != nil {
			return ftsNS, err
		}
	} else if err := unix.Stat(e.sysPath, &st); err != nil {
		if err != unix.ENOENT || unix.Lstat(e.sysPath, &st) != nil {
			return ftsNS, err
		}
		e.setStat(&st)
		return ftsSLNONE, nil
	}
	e.setStat(&st)
	switch st.Mode & unix.S_IFMT {
	case unix.S_IFDIR:
		return ftsD, nil
	case unix.S_IFLNK:
		return ftsSL, nil
	}
	return ftsF, nil
}

// afterRoot runs per-start-point completions (Task 8: pending -execdir +).
func (r *Run) afterRoot() {}
```

- [ ] **Step 7: `visit.go` — the fastwalk callback and `consider_visiting`**

```go
package find

import (
	"io/fs"

	"github.com/charlievieth/fastwalk"
	"golang.org/x/sys/unix"

	"qfind/internal/libc"
)

// callback is the fs.WalkDirFunc handed to fastwalk.Walk. It runs on
// several goroutines at once. fastwalk's contract shapes every return:
//   - a directory callback runs when the directory is dequeued, before it
//     is read; it returns nil (read it) or SkipDir, never another error;
//   - a non-directory callback returns nil, or ErrTraverseLink for a
//     followed symlink to a directory to descend; anything else would
//     abandon the parent directory;
//   - a second callback with err != nil reports that a directory could not
//     be read; it returns nil, because any error aborts the whole walk.
func (w *walker) callback(path string, d fs.DirEntry, err error) error {
	de := d.(fastwalk.DirEntry)
	depth := de.Depth()
	if err != nil {
		w.readError(w.dirEntry(path, depth), err)
		return nil
	}
	if depth == 0 {
		return nil // the root was considered before Walk
	}
	r := w.r
	gpath := w.rp.gnu(path)
	acc := de.Name()
	if r.opt.Symlinks == SymlinkAlwaysDeref {
		acc = gpath // FTS_LOGICAL implies FTS_NOCHDIR: fts_accpath is fts_path
	}
	e := &Entry{path: gpath, sysPath: path, name: de.Name(), accPath: acc, depth: w.baseDepth + depth, startLen: w.root.startLen}
	parent := w.dirs.get(w.rp.parentKey(path, depth))
	typ := de.Type()
	switch {
	case typ.IsDir():
		return w.considerDir(e, path, parent, false)
	case typ&fs.ModeSymlink != 0 && r.opt.Symlinks == SymlinkAlwaysDeref:
		return w.considerFollowedLink(e, path, parent)
	case r.opt.Mount:
		return w.considerMounted(e)
	}
	e.setType(typ)
	w.consider(e, ftsNSOK, nil)
	return nil
}

// dirEntry returns the Entry of a directory fastwalk has read or tried to.
func (w *walker) dirEntry(path string, depth int) *Entry {
	if depth == 0 {
		return w.root
	}
	return w.dirs.get(path).entry
}

// consider is consider_visiting, under the evaluation lock when
// evaluation is serialised: GNU considers one entry at a time, so with
// -quit, -exec & co. or -D, no diagnostic or visit of another entry may
// interleave with this one.
func (w *walker) consider(e *Entry, info ftsInfo, err error) bool {
	if w.r.serial {
		w.r.evalMu.Lock()
		defer w.r.evalMu.Unlock()
	}
	return w.considerVisiting(e, info, err)
}

// considerDir handles an entry fts stats itself: a directory (or, under
// -L, a symlink to one). Its stat decides everything: the entry may have
// stopped being a directory since readdir, and fastwalk opens what it is
// told to read without O_DIRECTORY or O_NONBLOCK (a FIFO would block a
// worker), so only a confirmed directory is ever descended into.
func (w *walker) considerDir(e *Entry, key string, parent *dirNode, viaLink bool) error {
	r := w.r
	var st unix.Stat_t
	statFn := unix.Lstat
	if r.opt.Symlinks == SymlinkAlwaysDeref {
		statFn = unix.Stat
	}
	if err := statFn(e.sysPath, &st); err != nil {
		w.consider(e, ftsNS, err)
		return skip(viaLink)
	}
	e.setStat(&st)
	if r.opt.Mount && st.Dev != w.rootDev {
		return skip(viaLink) // FTS_MOUNT: never returned
	}
	switch st.Mode & unix.S_IFMT {
	case unix.S_IFDIR:
	case unix.S_IFLNK:
		w.consider(e, ftsSL, nil)
		return skip(viaLink)
	default:
		w.consider(e, ftsF, nil)
		return skip(viaLink)
	}
	if isCycle(parent, st.Dev, st.Ino) {
		w.consider(e, ftsDC, nil)
		return skip(viaLink)
	}
	w.dirs.add(key, &dirNode{dev: st.Dev, ino: st.Ino, parent: parent, entry: e})
	descend := w.consider(e, ftsD, nil)
	if r.opt.Xdev && st.Dev != w.rootDev {
		descend = false // FTS_XDEV: visited, never entered
	}
	if !descend {
		return skip(viaLink)
	}
	if viaLink {
		return fastwalk.ErrTraverseLink
	}
	return nil
}

func skip(viaLink bool) error {
	if viaLink {
		return nil // fastwalk does not traverse a link unless asked
	}
	return fastwalk.SkipDir
}

// considerFollowedLink handles a symlink under -L, which fts stats
// (fts_stat with follow): ENOENT means a dangling link (FTS_SLNONE), other
// errors are FTS_NS, and a directory target is considered as a directory.
func (w *walker) considerFollowedLink(e *Entry, key string, parent *dirNode) error {
	r := w.r
	var st unix.Stat_t
	err := unix.Stat(e.sysPath, &st)
	switch {
	case err == nil && st.Mode&unix.S_IFMT == unix.S_IFDIR:
		return w.considerDir(e, key, parent, true)
	case err == nil:
		e.setStat(&st)
		if r.opt.Mount && st.Dev != w.rootDev {
			return nil
		}
		w.consider(e, ftsF, nil)
	case err == unix.ENOENT && unix.Lstat(e.sysPath, &st) == nil:
		e.setStat(&st)
		w.consider(e, ftsSLNONE, nil)
	default:
		w.consider(e, ftsNS, err)
	}
	return nil
}

// considerMounted is FTS_MOUNT's stat of every entry: entries on another
// device are never returned; a failed stat is (FTS_NS).
func (w *walker) considerMounted(e *Entry) error {
	var st unix.Stat_t
	if err := unix.Lstat(e.sysPath, &st); err != nil {
		w.consider(e, ftsNS, err)
		return nil
	}
	e.setStat(&st)
	if st.Dev != w.rootDev {
		return nil
	}
	info := ftsF
	if st.Mode&unix.S_IFMT == unix.S_IFLNK {
		info = ftsSL
	}
	w.consider(e, info, nil)
	return nil
}

// readError maps fastwalk's report that it could not read a directory to
// the fts state GNU would see:
//   - open(2) failed: FTS_DNR;
//   - getdents(2) failed: FTS_ERR (fts reports it in place of FTS_DP);
//   - lstat of a DT_UNKNOWN child failed: FTS_NS for that child. fastwalk
//     abandons the rest of the directory here, an accepted divergence.
func (w *walker) readError(dir *Entry, err error) {
	var pe *fs.PathError
	var se *os.SyscallError
	errno, ok := libc.Errno(err)
	if !ok {
		errno = unix.EIO
	}
	switch {
	case errors.As(err, &pe) && pe.Op == "lstat":
		name := pe.Path[strings.LastIndexByte(pe.Path, '/')+1:]
		child := &Entry{path: w.rp.gnu(pe.Path), sysPath: pe.Path, name: name, accPath: name,
			depth: dir.depth + 1, startLen: w.root.startLen}
		if w.r.opt.Symlinks == SymlinkAlwaysDeref {
			child.accPath = child.path
		}
		w.consider(child, ftsNS, errno)
	case errors.As(err, &se):
		w.consider(dir, ftsERR, errno)
	default:
		w.consider(dir, ftsDNR, errno)
	}
}

// considerVisiting is ftsfind.c consider_visiting for one entry; call it
// through consider. It returns whether a directory may be descended into.
func (w *walker) considerVisiting(e *Entry, info ftsInfo, err error) bool {
	r := w.r
	w.narrateEarly(e, info)
	switch info {
	case ftsERR:
		e.cancelled.Store(true) // fts returns FTS_ERR in place of FTS_DP
		e.issuedStatErr = false // a new fts_read iteration
		r.nonfatalTargetFileError(e, err)
		return false
	case ftsDNR:
		if errno, _ := libc.Errno(err); errno == unix.ENOENT && r.opt.IgnoreReaddirRace {
			e.cancelled.Store(true) // GNU returns before the post-order visit
			return false
		}
		e.issuedStatErr = false
		r.nonfatalTargetFileError(e, err)
		// Without -depth the directory was visited in pre-order. With it, the
		// deferred visit of e is GNU's visit of this FTS_DNR entry.
		return false
	case ftsDC:
		r.errorf(nil, "File system loop detected; the following directory is part of the cycle: %s", r.quote(e.path))
		r.setFailure()
		return false
	case ftsSLNONE:
		if w.symlinkLoop(e) {
			r.nonfatalTargetFileError(e, unix.ELOOP)
			return false
		}
	case ftsNS:
		if e.depth == 0 {
			r.nonfatalTargetFileError(e, err)
			return false
		}
		if w.symlinkLoop(e) {
			r.nonfatalTargetFileError(e, unix.ELOOP)
			return false
		}
		if errno, _ := libc.Errno(err); errno == unix.ENOENT && r.opt.IgnoreReaddirRace {
			return false
		}
		r.nonfatalTargetFileError(e, err)
		// Visit without stat or type: fts zeroes the stat buffer, so a d_type
		// must not leak through (-type d would wrongly match).
		e.haveStat, e.haveType, e.mode = false, false, 0
	}
	if e.haveStat && e.st.Mode == 0 {
		r.errorf(nil, "WARNING: file %s appears to have mode 0000", r.quote(e.path))
	}
	isDir := e.isDir() || info == ftsD
	descend := isDir
	ignore := false
	if r.opt.MaxDepth >= 0 && e.depth >= r.opt.MaxDepth {
		descend = false
		ignore = e.depth > r.opt.MaxDepth
	}
	postOrder := info == ftsD && !r.opt.DoDirFirst
	if e.depth < r.opt.MinDepth {
		ignore = true
	}
	if r.opt.Debug&DebugSearch != 0 {
		r.debugf("consider_visiting (late): %s: fts_info=%-6s, isdir=%d ignore=%d have_stat=%d have_type=%d \n",
			r.quote(e.path), ftsInfoNames[info], b2i(isDir), b2i(ignore || postOrder), b2i(e.haveStat), b2i(e.haveType))
	}
	switch {
	case ignore:
	case postOrder:
		w.defers.add(e) // visited as FTS_DP after the walk (depth.go)
	default:
		r.visit(e)
		if e.prune {
			descend = false
		}
	}
	return descend
}

// narrateEarly is consider_visiting's first -D search line. prev_depth is
// GNU's static: the level of the previously considered entry.
func (w *walker) narrateEarly(e *Entry, info ftsInfo) {
	r := w.r
	prev := r.prevDepth.Swap(int64(e.depth))
	if r.opt.Debug&DebugSearch != 0 {
		r.debugf("consider_visiting (early): %s: fts_info=%-6s, fts_level=%2d, prev_depth=%d fts_path=%s, fts_accpath=%s\n",
			r.quote(e.path), ftsInfoNames[info], e.depth, prev, r.quote(e.path), r.quote(e.accPath))
	}
}

// symlinkLoop is ftsfind.c symlink_loop: options.xstat (so it shows under
// -D stat) fails with ELOOP.
func (w *walker) symlinkLoop(e *Entry) bool {
	_, err := w.r.xstat(e)
	return err == unix.ELOOP
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// considerRoot considers a start point and reports whether to walk below it.
func (w *walker) considerRoot(root *Entry, info ftsInfo, err error) bool {
	return w.consider(root, info, err) && info == ftsD
}
```
`visit` (Task 3) takes no lock itself. The callers hold `evalMu` when `r.serial`: `consider` here, and `visitPostOrder` in `depth.go`. Imports for `visit.go` are `errors`, `io/fs`, `os`, `strings`, fastwalk, `x/sys/unix` and `qfind/internal/libc`.

In `main.go`, after building the tree, set `r.workers = r.numWorkers()`, `r.prevDepth.Store(math.MinInt32)` and `r.serial = r.needsSerial()`:
```go
// needsSerial reports whether GNU's one-entry-at-a-time evaluation is
// observable across entries for this command line: exec-family actions
// share stdin and flush stdout before each child, -quit must stop
// everything at once, \c flushes mid-record, and -D narrates global state.
func (r *Run) needsSerial() bool {
	if r.opt.Debug != 0 || r.formatFlushes {
		return true
	}
	for p := r.preds; p != nil; p = p.next {
		switch p.id {
		case fnQuit, fnExec, fnExecdir, fnOk, fnOkdir:
			return true
		}
	}
	return false
}
```
`buildExpressionTree` stores the final predicate list head in `r.preds` just before `checkOptionCombinations`.

- [ ] **Step 8: `depth.go` — `-depth` post-order**

```go
package find

import (
	"slices"
	"sync"
)

// deferredDirs holds directories whose FTS_DP visit must wait until their
// subtree has been walked. Files are never deferred: for them pre-order
// and post-order coincide.
type deferredDirs struct {
	mu      sync.Mutex
	byDepth map[int][]*Entry
}

func (d *deferredDirs) add(e *Entry) {
	d.mu.Lock()
	if d.byDepth == nil {
		d.byDepth = map[int][]*Entry{}
	}
	d.byDepth[e.depth] = append(d.byDepth[e.depth], e)
	d.mu.Unlock()
}

// runDeferred visits deferred directories deepest level first, after the
// walk of the starting point has finished (fastwalk has no "directory
// done" event). Every descendant of a directory is either a file, visited
// during the walk, or a directory at a deeper level, visited earlier here;
// so this is a valid post-order. Directories at one level are never
// ancestors of one another, so a level is evaluated in parallel.
func (w *walker) runDeferred() {
	depths := make([]int, 0, len(w.defers.byDepth))
	for d := range w.defers.byDepth {
		depths = append(depths, d)
	}
	slices.Sort(depths)
	slices.Reverse(depths)
	for _, d := range depths {
		level := w.defers.byDepth[d]
		sem := make(chan struct{}, w.r.workers)
		var wg sync.WaitGroup
		for _, e := range level {
			sem <- struct{}{}
			wg.Go(func() {
				defer func() { <-sem }()
				w.visitPostOrder(e)
			})
		}
		wg.Wait()
	}
	w.defers.byDepth = nil
}

// visitPostOrder is consider_visiting for a directory's FTS_DP entry (or
// FTS_DNR, see considerVisiting) under -depth: narrate, then visit with
// the stat taken in pre-order, as fts does.
func (w *walker) visitPostOrder(e *Entry) {
	r := w.r
	if r.serial {
		r.evalMu.Lock()
		defer r.evalMu.Unlock()
	}
	if e.cancelled.Load() {
		return
	}
	w.narrateEarly(e, ftsDP)
	if r.opt.Debug&DebugSearch != 0 {
		r.debugf("consider_visiting (late): %s: fts_info=%-6s, isdir=%d ignore=%d have_stat=%d have_type=%d \n",
			r.quote(e.path), ftsInfoNames[ftsDP], 1, 0, b2i(e.haveStat), b2i(e.haveType))
	}
	r.visit(e)
}
```
Without `-depth`, GNU also narrates each directory's `FTS_DP` entry under `-D search` (as ignored). qfind has no completion event to hang that line on, so it omits it. This is a `-D` narration difference only, recorded with the accepted divergences.

- [ ] **Step 9: `files0.go` — `-files0-from` (process_all_startpoints)**

```go
package find

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// processAllStartPoints is ftsfind.c process_all_startpoints.
func (r *Run) processAllStartPoints() bool {
	if !r.opt.HasFiles0From {
		if len(r.startPoints) == 0 {
			return r.findRoot(".")
		}
		ok := true
		for _, name := range r.startPoints {
			if name == "" {
				r.errorf(unix.ENOENT, "%s", r.quote(name))
				r.setFailure()
				ok = false
				continue
			}
			if !r.findRoot(name) {
				return false
			}
		}
		return ok
	}
	if len(r.startPoints) > 0 {
		r.errorf(nil, "extra operand %s", r.quote(r.startPoints[0]))
		r.fatalf(nil, "file operands cannot be combined with -files0-from")
	}
	in, quoted := os.Stdin, r.quote("(standard input)")
	if r.opt.Files0From == "-" {
		if r.opt.OkPromptStdin {
			r.fatalf(nil, "when -ok or -okdir is in use, the option -files0-from cannot also read from standard input")
		}
	} else {
		quoted = r.quote(r.opt.Files0From)
		f, err := os.Open(r.opt.Files0From)
		if err != nil {
			r.fatalf(err, "cannot open %s for reading", quoted)
		}
		if r.opt.OkPromptStdin && sameInode(f, os.Stdin) {
			r.fatalf(nil, "when -ok or -okdir is in use, the option -files0-from cannot also read from standard input, but the file (%s) used for -files0-from appears to refer to the same file as the standard input", quoted)
		}
		in = f
	}
	br := bufio.NewReaderSize(in, 64<<10)
	ok := true
	for n := 1; ; n++ {
		item, err := br.ReadString(0)
		if err != nil && !errors.Is(err, io.EOF) {
			r.errorf(err, "%s: read error", quoted)
			r.setFailure()
			return false
		}
		if item == "" && errors.Is(err, io.EOF) {
			break
		}
		item = strings.TrimSuffix(item, "\x00")
		if item == "" {
			r.errorf(nil, "%s:%d: %s", quoted, n, "invalid zero-length file name")
			r.setFailure()
			ok = false
		} else if !r.findRoot(item) {
			return false
		}
		if errors.Is(err, io.EOF) {
			break
		}
	}
	// GNU fcloses the stream, stdin included, so commands run by cleanup()
	// (pending -exec … +) start with fd 0 closed after -files0-from -.
	if ok && in.Close() != nil {
		r.fatalf(nil, "error reading %s", quoted)
	}
	return ok
}

func sameInode(a, b *os.File) bool {
	var sa, sb unix.Stat_t
	return unix.Fstat(int(a.Fd()), &sa) == nil && unix.Fstat(int(b.Fd()), &sb) == nil &&
		sa.Dev == sb.Dev && sa.Ino == sb.Ino
}
```
`main.go`: `if r.processAllStartPoints() { r.showSuccessRates(r.tree); r.cleanup() }`. A false return skips cleanup, exactly as in GNU, and that also drops pending batches.

- [ ] **Step 10: Build and run the gates**

```bash
bundle exec rake qfind:build
bundle exec rspec spec/find/traversal_spec.rb spec/find/starting_points_spec.rb spec/find/global_options spec/find/options/symbolic_links_spec.rb spec/find/positional_options/follow_spec.rb spec/find/actions/prune_spec.rb spec/find/actions/quit_spec.rb spec/find/operators spec/find/tests/true_spec.rb spec/find/tests/false_spec.rb --format progress 2>&1 | grep -E '^[0-9]+ examples?, '
ruby .scratch/failures.rb .scratch/run.json
```
Expected: every failure names a primary owned by a later task (`-name`, `-type`, `-printf`, `-exec`, `-execdir`, `-ls`, `-delete`, `-regex`, `-newer`, `-mtime`, `-size`, `-perm`, …). Check each one in `rspec --format documentation`. The traversal-only examples must all pass, including:
- unreadable directories;
- `/` with `-maxdepth 1`;
- `-L` loops;
- `-mount`/`-xdev`;
- `-files0-from` (100000 items);
- the 1000-run readdir race;
- the `.in_order` chains.

- [ ] **Step 11: Differential checks for Review Focus 1–3**

`.scratch/diff/many.sh`: build a tree of 200,000 files across 2,000 directories, then compare both binaries:
```bash
#!/bin/bash
set -eu
D=/workspace/qfind.master/.scratch/diff/manytree
[ -d "$D" ] || { mkdir -p "$D"; for i in $(seq 1 2000); do mkdir "$D/d$i"; (cd "$D/d$i" && touch $(seq -f f%g 1 100)); done; }
G=/workspace/qfind.master/findutils-4.11.0/find/find Q=/workspace/qfind.master/dist/find
for args in "-print" "-printf %p:%d\n"; do
  cmp <(exec -a find $G $D $args | sort) <(exec -a find $Q $D $args | sort) && echo "same: $args"
done
cmp <(exec -a find $G $D -print0 | sort -z) <(exec -a find $Q $D -print0 | sort -z) && echo "same: -print0"
# Each binary writes its own -fprint file, one run after the other, so each
# file holds only what that binary wrote.
(exec -a find $G $D -fprint $D.g.out -print) | sort > $D.g.stdout
(exec -a find $Q $D -fprint $D.q.out -print) | sort > $D.q.stdout
cmp $D.g.stdout $D.q.stdout && echo "same: -fprint stdout"
cmp <(sort $D.g.out) <(sort $D.q.out) && echo "same: -fprint file"
```
It must print five `same:` lines; otherwise `cmp` names the first byte that differs.

`.scratch/diff/huge.sh` (peak RSS must stay below 64 MiB for both runs, since files are never deferred):
```bash
#!/bin/bash
set -eu
D=/workspace/qfind.master/.scratch/diff/hugedir Q=/workspace/qfind.master/dist/find
mkdir -p "$D" && cd "$D" && seq -f 'f%g' 1 1000000 | xargs touch && cd ..
/usr/bin/time -f 'print: %M KiB' "$Q" "$D" -print | wc -l    # 1000001
/usr/bin/time -f 'delete: %M KiB' "$Q" "$D" -depth -delete
test ! -e "$D" && echo deleted
```

`.scratch/diff/deep.sh`: a 600-level chain of 9-character directory names (paths of about 6,000 bytes), then compare `qfind chain | wc -l` and stderr with GNU. Until Task 15 adds the fd-relative fallback, record the difference as expected (`File name too long`) in the task report.

- [ ] **Step 12: Commit**

```bash
gofmt -l internal; go vet ./...
git add internal
git commit -m "qfind: fastwalk traversal with fts semantics, cycles, -depth and -files0-from"
```

---

## Task 5: Name and path tests on a glibc-compatible fnmatch

GNU find calls **glibc's own `fnmatch`**: `REPLACE_FNMATCH=0` in `config.status`, and gnulib's `fnmatch.o` is not built. Go's `path.Match` differs in every important way: `*` and `?` stop at `/`, `[^` is handled differently, there is no casefold and no POSIX classes, and a stray `[` is an error. This task ports glibc's matcher (gnulib's `gl/lib/fnmatch_loop.c` is the same code) for the flags find uses, then the eight primaries built on it.

**Files:**
- Create: `internal/fnmatch/fnmatch.go`, `internal/fnmatch/class.go`
- Create: `internal/find/pred_names.go`, `internal/find/parse_names.go`
- Modify: `internal/find/parsetable.go` (8 entries), `internal/find/run.go` (`fnOpts()`)

**Interfaces:**
- Produces:
  - `fnmatch.Options{CaseFold, UTF8, PosixlyCorrect bool}`
  - `fnmatch.Match(pattern, name string, o fnmatch.Options) bool`
  - `(*Run).fnOpts(casefold bool) fnmatch.Options`
  - `baseName(path string) string` (gnulib `base_name`, used by `%f` in Task 7 and `-execdir` in Task 8)
  - `stripTrailingSlashes(s string) string`
  - `estimatePatternMatchRate(pattern string, isRegex bool) float32`
  - `(*Run).matchesStartPoint(glob string, fold bool) bool`

- [ ] **Step 1: Failing gates**

```bash
bundle exec rspec spec/find/tests/{name,iname,path,wholename,ipath,iwholename,lname,ilname}_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
```

- [ ] **Step 2: `internal/fnmatch/fnmatch.go` — port of `FCT` (fnmatch_loop.c)**

```go
// Package fnmatch is glibc fnmatch(3) for the flags GNU find uses:
// FNM_CASEFOLD or nothing (never FNM_PATHNAME, FNM_PERIOD, FNM_NOESCAPE or
// FNM_EXTMATCH). Ranges follow the nrules == 0 collation path (character
// values), which is exact for the C locale.
package fnmatch

import (
	"unicode"
	"unicode/utf8"
)

// Options are fnmatch's flags plus the process state glibc consults.
type Options struct {
	CaseFold       bool // FNM_CASEFOLD
	UTF8           bool // MB_CUR_MAX > 1: match characters, not bytes
	PosixlyCorrect bool // POSIXLY_CORRECT set: '^' no longer negates
}

// Match is fnmatch(pattern, name, flags) == 0.
func Match(pattern, name string, o Options) bool {
	if o.UTF8 {
		if !utf8.ValidString(pattern) || !utf8.ValidString(name) {
			return false // mbsrtowcs fails; fnmatch returns -1
		}
		return match([]rune(pattern), []rune(name), o)
	}
	return match([]byte(pattern), []byte(name), o)
}

type char interface{ ~byte | ~rune }

func fold[T char](c T, o Options) T {
	if !o.CaseFold {
		return c
	}
	if o.UTF8 {
		return T(unicode.ToLower(rune(c)))
	}
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

// match is FCT: c = FOLD(*p++) then the switch on c.
func match[T char](p, n []T, o Options) bool {
	for len(p) > 0 {
		c := fold(p[0], o)
		p = p[1:]
		switch c {
		case '?':
			if len(n) == 0 {
				return false
			}
		case '\\':
			if len(p) == 0 {
				return false
			}
			c = fold(p[0], o)
			p = p[1:]
			if len(n) == 0 || fold(n[0], o) != c {
				return false
			}
		case '*':
			for len(p) > 0 && (p[0] == '?' || p[0] == '*') {
				if p[0] == '?' {
					if len(n) == 0 {
						return false
					}
					n = n[1:]
				}
				p = p[1:]
			}
			if len(p) == 0 {
				return true
			}
			for i := range n {
				if match(p, n[i:], o) {
					return true
				}
			}
			return false
		case '[':
			if len(n) == 0 {
				return false
			}
			rest, ok, literal := bracket(p, n[0], o)
			if literal { // unterminated: '[' is an ordinary character
				if fold(n[0], o) != '[' {
					return false
				}
				break
			}
			if !ok {
				return false
			}
			p = rest
		default:
			if len(n) == 0 || c != fold(n[0], o) {
				return false
			}
		}
		n = n[1:]
	}
	return len(n) == 0
}

// bracket is FCT's `case L_('['):` for character ch. It returns the
// pattern after ']' and whether ch matched; literal reports an
// unterminated expression, where FCT goes back to p_init and matches '['
// as a normal character.
func bracket[T char](pat []T, ch T, o Options) (rest []T, matched, literal bool) {
	p := pat
	next := func() T {
		if len(p) == 0 {
			return 0
		}
		c := p[0]
		p = p[1:]
		return c
	}
	peek := func(i int) T {
		if i < len(p) {
			return p[i]
		}
		return 0
	}
	not := peek(0) == '!' || (!o.PosixlyCorrect && peek(0) == '^')
	if not {
		next()
	}
	fn := fold(ch, o)
	c := next()
	for {
		member := true
		switch {
		case c == '\\':
			if peek(0) == 0 {
				return nil, false, false
			}
			c = fold(next(), o)
		case c == '[' && peek(0) == ':':
			name, size, isName := scanClass(p[1:])
			if isName {
				if !validClass(name) {
					return nil, false, false
				}
				p = p[1+size:]
				if inClass(name, rune(ch), o.UTF8) {
					return skipRest(p, not)
				}
				c, member = next(), false
			}
		case c == '[' && (peek(0) == '=' || peek(0) == '.'):
			delim := peek(0)
			if peek(1) != 0 && peek(2) == delim && peek(3) == ']' {
				single := peek(1)
				p = p[4:]
				if single == ch {
					return skipRest(p, not)
				}
				c, member = next(), false
			}
		case c == 0:
			return pat, false, true
		default:
			c = fold(c, o)
		}
		if member {
			isRange := peek(0) == '-' && peek(1) != 0 && peek(1) != ']'
			if !isRange && c == fn {
				return skipRest(p, not)
			}
			cold := c
			c = next()
			if c == '-' && peek(0) != ']' {
				cend := next()
				if cend == '\\' {
					cend = next()
				}
				if cend == 0 {
					return nil, false, false
				}
				if cold <= fn && fn <= fold(cend, o) {
					return skipRest(p, not)
				}
				c = next()
			}
		}
		if c == ']' {
			break
		}
		if c == 0 {
			return pat, false, true
		}
	}
	if !not {
		return nil, false, false
	}
	return p, true, false
}

// skipRest is FCT's `matched:` label: skip what remains of [...].
func skipRest[T char](p []T, not bool) ([]T, bool, bool) {
	for {
		if len(p) == 0 {
			return nil, false, false // "[... (unterminated) loses"
		}
		c := p[0]
		p = p[1:]
		switch {
		case c == ']':
			if not {
				return nil, false, false
			}
			return p, true, false
		case c == '\\':
			if len(p) == 0 {
				return nil, false, false
			}
			p = p[1:]
		case c == '[' && len(p) > 0 && p[0] == ':':
			if _, size, isName := scanClass(p[1:]); isName {
				p = p[1+size:]
			}
		case c == '[' && len(p) > 0 && (p[0] == '=' || p[0] == '.'):
			delim := p[0]
			i := 1
			for i+1 < len(p) && !(p[i] == delim && p[i+1] == ']') {
				i++
			}
			if i+1 >= len(p) {
				return nil, false, false
			}
			p = p[i+2:]
		}
	}
}

// scanClass reads "name:]" after "[:"; isName is false when a character
// outside [a-y] (glibc's `c < 'a' || c >= 'z'`) appears first, in which
// case '[' is an ordinary member.
func scanClass[T char](p []T) (name string, size int, isName bool) {
	for i := 0; i < len(p) && i < 256; i++ {
		if p[i] == ':' && i+1 < len(p) && p[i+1] == ']' {
			return string(runesOf(p[:i])), i + 3, true
		}
		if p[i] < 'a' || p[i] >= 'z' {
			return "", 0, false
		}
	}
	return "", 0, false
}

func runesOf[T char](s []T) []rune {
	r := make([]rune, len(s))
	for i, c := range s {
		r[i] = rune(c)
	}
	return r
}
```
**Port check.** The `+3` in `scanClass` counts the leading `:` (consumed by the caller via `p[1:]`) plus the closing `:]`. Re-read `fnmatch_loop.c` lines 228–700 side by side when implementing. Every `return FNM_NOMATCH` becomes `nil, false, false`, and the `c == '\0'` branch becomes `literal`.

`class.go`:
```go
package fnmatch

import "unicode"

var classes = map[string]bool{"alnum": true, "alpha": true, "blank": true, "cntrl": true,
	"digit": true, "graph": true, "lower": true, "print": true, "punct": true,
	"space": true, "upper": true, "xdigit": true}

func validClass(name string) bool { return classes[name] }

// inClass is _ISCTYPE / iswctype: ASCII ctype in single-byte locales,
// Unicode categories in UTF-8 ones.
func inClass(name string, r rune, utf8 bool) bool {
	if !utf8 && r >= 0x80 {
		return false
	}
	switch name {
	case "alnum":
		return unicode.IsLetter(r) || unicode.IsDigit(r)
	case "alpha":
		return unicode.IsLetter(r)
	case "blank":
		return r == ' ' || r == '\t'
	case "cntrl":
		return unicode.IsControl(r)
	case "digit":
		return r >= '0' && r <= '9'
	case "graph":
		return unicode.IsGraphic(r) && !unicode.IsSpace(r)
	case "lower":
		return unicode.IsLower(r)
	case "print":
		return unicode.IsPrint(r)
	case "punct":
		return unicode.IsPunct(r) || (r < 0x80 && unicode.IsSymbol(r))
	case "space":
		return unicode.IsSpace(r)
	case "upper":
		return unicode.IsUpper(r)
	case "xdigit":
		return (r >= '0' && r <= '9') || (r|0x20 >= 'a' && r|0x20 <= 'f')
	}
	return false
}
```

- [ ] **Step 3: `parse_names.go` — parse_name/iname/path/wholename/ipath/iwholename/lname/ilname**

Port these exactly as shown in `findutils-4.11.0/find/parser.c`:
- `check_name_arg`, `parse_name`, `parse_iname`;
- `is_feasible_path_argument` and `insert_path_check`, whose warning is printed **unconditionally** unless POSIXLY_CORRECT;
- `parse_path`, `parse_wholename`, `parse_ipath`, `parse_iwholename`;
- `parse_lname`, `parse_ilname`, which keep the default `need_stat` and `need_type`;
- `estimate_pattern_match_rate`.

```go
package find

import "strings"

func (ps *parser) checkNameArg(pred, alt, arg string) {
	if ps.shouldIssueWarnings() && strings.Contains(arg, "/") && arg != "/" {
		r := ps.r
		r.errorf(nil, "warning: %s matches against basenames only, but the given pattern contains a directory separator (%s), thus the expression will evaluate to false all the time.  Did you mean %s?",
			r.quote(pred), r.quote("/"), r.quote(alt))
	}
}

func (ps *parser) insertName(e *parserEntry, pred, alt string) bool {
	name, ok := ps.collectArg()
	if !ok {
		return false
	}
	ps.checkNameArg(pred, alt, name)
	p := ps.insertPrimary(e, name, true)
	p.needStat, p.needType = false, false
	p.args.str = name
	p.estSuccessRate = estimatePatternMatchRate(name, false)
	return true
}

func parseName(ps *parser, e *parserEntry) bool  { return ps.insertName(e, "-name", "-wholename") }
func parseIname(ps *parser, e *parserEntry) bool { return ps.insertName(e, "-iname", "-iwholename") }

func (ps *parser) insertPathCheck(e *parserEntry, predName string, fold bool) bool {
	name, ok := ps.collectArg()
	if !ok {
		return false
	}
	fn, id := PredFunc(predPath), fnPath
	if fold {
		fn, id = predIpath, fnIpath
	}
	p := ps.insertPrimaryWithPred(e, fn, id, name, true)
	p.needStat, p.needType = false, false
	p.args.str = name
	p.estSuccessRate = estimatePatternMatchRate(name, false)
	if !ps.r.opt.PosixlyCorrect && !ps.r.isFeasiblePathArgument(name, fold) {
		ps.r.errorf(nil, "warning: -%s %s will not match anything because it ends with /.", predName, name)
		p.estSuccessRate = 1.0e-8
	}
	return true
}

func parsePath(ps *parser, e *parserEntry) bool       { return ps.insertPathCheck(e, "path", false) }
func parseWholename(ps *parser, e *parserEntry) bool  { return ps.insertPathCheck(e, "wholename", false) }
func parseIpath(ps *parser, e *parserEntry) bool      { return ps.insertPathCheck(e, "ipath", true) }
func parseIwholename(ps *parser, e *parserEntry) bool { return ps.insertPathCheck(e, "iwholename", true) }

func (ps *parser) insertLname(e *parserEntry) bool {
	name, ok := ps.collectArg()
	if !ok {
		return false
	}
	p := ps.insertPrimary(e, name, true)
	p.args.str = name
	p.estSuccessRate = 0.1 * estimatePatternMatchRate(name, false)
	return true
}

func parseLname(ps *parser, e *parserEntry) bool  { return ps.insertLname(e) }
func parseIlname(ps *parser, e *parserEntry) bool { return ps.insertLname(e) }

// isFeasiblePathArgument is parser.c is_feasible_path_argument.
func (r *Run) isFeasiblePathArgument(arg string, fold bool) bool {
	if i := strings.LastIndexByte(arg, '/'); i >= 0 && i == len(arg)-1 {
		return r.matchesStartPoint(arg, fold)
	}
	return true
}

// matchesStartPoint is tree.c matches_start_point.
func (r *Run) matchesStartPoint(glob string, fold bool) bool {
	if len(r.startPoints) == 0 {
		return fnmatch.Match(glob, ".", r.fnOpts(fold))
	}
	for _, s := range r.startPoints {
		if fnmatch.Match(glob, s, r.fnOpts(fold)) {
			return true
		}
	}
	return false
}

// estimatePatternMatchRate is parser.c estimate_pattern_match_rate.
func estimatePatternMatchRate(pattern string, isRegex bool) float32 {
	if strings.ContainsAny(pattern, "*?[") || (isRegex && strings.Contains(pattern, ".")) {
		return 0.8
	}
	return 0.1
}
```
`buildExpressionTree` must collect `r.startPoints` *before* the parse loop, as Task 3's code already does, because `matches_start_point` is consulted while parsing. With `-files0-from`, GNU's `start_points` holds no names, so `.` is used.

- [ ] **Step 4: `pred_names.go`**

```go
package find

import (
	"strings"

	"golang.org/x/sys/unix"

	"qfind/internal/fnmatch"
)

func (r *Run) fnOpts(fold bool) fnmatch.Options {
	return fnmatch.Options{CaseFold: fold, UTF8: r.loc.UTF8, PosixlyCorrect: r.opt.PosixlyCorrect}
}

// baseName is gnulib base_name: the last component, keeping one trailing
// slash ("dir/" → "dir/", "a//" → "a/"), and "/" for an all-slash path.
func baseName(path string) string {
	i := len(path)
	for i > 0 && path[i-1] == '/' {
		i--
	}
	if i == 0 {
		if path == "" {
			return ""
		}
		return "/"
	}
	start := strings.LastIndexByte(path[:i], '/') + 1
	if i < len(path) {
		return path[start : i+1]
	}
	return path[start:i]
}

// stripTrailingSlashes is gnulib strip_trailing_slashes ("/" stays "/").
func stripTrailingSlashes(s string) string {
	t := strings.TrimRight(s, "/")
	if t == "" && s != "" {
		return "/"
	}
	return t
}

// predNameCommon is pred.c pred_name_common.
func (r *Run) predNameCommon(path, pattern string, fold bool) bool {
	return fnmatch.Match(pattern, stripTrailingSlashes(baseName(path)), r.fnOpts(fold))
}

func predName(r *Run, e *Entry, p *Predicate) bool  { return r.predNameCommon(e.path, p.args.str, false) }
func predIname(r *Run, e *Entry, p *Predicate) bool { return r.predNameCommon(e.path, p.args.str, true) }

func predPath(r *Run, e *Entry, p *Predicate) bool {
	return fnmatch.Match(p.args.str, e.path, r.fnOpts(false))
}

func predIpath(r *Run, e *Entry, p *Predicate) bool {
	return fnmatch.Match(p.args.str, e.path, r.fnOpts(true))
}

// matchLname is pred.c match_lname: only when the entry's stat is a link.
func (r *Run) matchLname(e *Entry, p *Predicate, fold bool) bool {
	if e.st.Mode&unix.S_IFMT != unix.S_IFLNK {
		return false
	}
	target, err := readlink(e.sysPath)
	if err != nil {
		r.nonfatalTargetFileError(e, err)
		r.setFailure()
		return false
	}
	return fnmatch.Match(p.args.str, target, r.fnOpts(fold))
}

func predLname(r *Run, e *Entry, p *Predicate) bool  { return r.matchLname(e, p, false) }
func predIlname(r *Run, e *Entry, p *Predicate) bool { return r.matchLname(e, p, true) }

// readlink is gnulib areadlinkat with a growing buffer.
func readlink(path string) (string, error) {
	for size := 256; ; size *= 2 {
		buf := make([]byte, size)
		n, err := unix.Readlink(path, buf)
		if err != nil {
			return "", err
		}
		if n < size {
			return string(buf[:n]), nil
		}
	}
}
```
Check `baseName` against `printf_names_spec.rb` (the `%f` rows of the `printf-h` table) when Task 7 uses it. For example: `./` → `./`; `.//` → `./`; `foo//` → `foo/`; `///` → `/`.

- [ ] **Step 5: Wire the table**

Replace these 8 lines in `parsetable.go`:
```go
		{argTest, "ilname", parseIlname, predIlname, fnIlname},
		{argTest, "iname", parseIname, predIname, fnIname},
		{argTest, "ipath", parseIpath, predIpath, fnIpath},
		{argTest, "iwholename", parseIwholename, predIpath, fnIpath},
		{argTest, "lname", parseLname, predLname, fnLname},
		{argTest, "name", parseName, predName, fnName},
		{argTest, "path", parsePath, predPath, fnPath},
		{argTest, "wholename", parseWholename, predPath, fnPath},
```

- [ ] **Step 6: Gates**

```bash
bundle exec rake qfind:build
bundle exec rspec spec/find/tests/{name,iname,path,wholename,ipath,iwholename,lname,ilname}_spec.rb spec/find/options/optimisation_spec.rb spec/find/operators spec/find/expressions_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
```
Expected:
- The 8 pattern files pass, except examples that also use `-printf`, `-exec` or `-type` (later tasks).
- The `-O` "promoting cheap predicate -name file" examples now pass.

Then run the whole suite to confirm no regressions in earlier gates: `bundle exec rspec --format progress | grep -E '^[0-9]+ examples?, '`.

- [ ] **Step 7: Commit**

```bash
gofmt -l internal; go vet ./...
git add internal
git commit -m "qfind: glibc-compatible fnmatch and the -name/-path/-lname families"
```

---

## Task 6: File type tests `-type` and `-xtype`

**Files:**
- Create: `internal/find/pred_types.go`
- Modify: `internal/find/parsetable.go` (2 entries), `internal/find/predicate.go` (`predArgs.types`)

**Interfaces:**
- Produces:
  - `type fileType uint8` with `ftypeBlk, ftypeChr, ftypeDir, ftypeReg, ftypeLnk, ftypeFifo, ftypeSock, ftypeDoor, ftypeCount` (defs.h `enum file_type`)
  - `predArgs.types [ftypeCount]bool`
  - `modeToFiletype(mode uint32) fileType`
  - `typeLetter(mode uint32) byte` (`f d l s b c p D U`, used by `%y`/`%Y` in Task 7)
  - `(*Run).followingLinks(e *Entry) bool` (util.c `following_links`)

- [ ] **Step 1: Failing gates**

```bash
bundle exec rspec spec/find/tests/type_spec.rb spec/find/tests/xtype_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
```

- [ ] **Step 2: `pred_types.go` — `insert_type`, `pred_type`, `pred_xtype` (parser.c 2533, pred.c 925/1080)**

```go
package find

import "golang.org/x/sys/unix"

type fileType uint8

const (
	ftypeBlk fileType = iota
	ftypeChr
	ftypeDir
	ftypeReg
	ftypeLnk
	ftypeFifo
	ftypeSock
	ftypeDoor
	ftypeCount
)

// insertType is parser.c insert_type.
func (ps *parser) insertType(e *parserEntry, fn PredFunc, id FuncID) bool {
	r := ps.r
	name := "-type"
	if id == fnXtype {
		name = "-xtype"
	}
	arg, ok := ps.collectArg()
	if !ok {
		return false
	}
	if arg == "" {
		r.fatalf(nil, "Arguments to %s should contain at least one letter", name)
	}
	p := ps.insertPrimaryWithPred(e, fn, id, arg, true)
	p.estSuccessRate = 0
	if id == fnXtype {
		p.needStat, p.needType = true, false
	} else {
		p.needStat, p.needType = false, true // struct dirent is enough
	}
	for i := 0; i < len(arg); {
		var cell fileType
		var rate float32
		switch arg[i] {
		case 'b':
			cell, rate = ftypeBlk, 0.000888
		case 'c':
			cell, rate = ftypeChr, 0.000443
		case 'd':
			cell, rate = ftypeDir, 0.0922
		case 'f':
			cell, rate = ftypeReg, 0.875
		case 'l':
			cell, rate = ftypeLnk, 0.0311
		case 'p':
			cell, rate = ftypeFifo, 7.554e-6
		case 's':
			cell, rate = ftypeSock, 1.59e-5
		case 'D':
			r.fatalf(nil, "%s %s is not supported because Solaris doors are not supported on the platform find was compiled on.", name, cbyte(arg[i]))
		default:
			r.fatalf(nil, "Unknown argument to %s: %s", name, cbyte(arg[i]))
		}
		if p.args.types[cell] {
			r.fatalf(nil, "Duplicate file type '%s' in the argument list to %s.", cbyte(arg[i]), name)
		}
		p.estSuccessRate += rate
		p.args.types[cell] = true
		i++
		if i < len(arg) {
			if arg[i] != ',' {
				r.fatalf(nil, "Must separate multiple arguments to %s using: ','", name)
			}
			i++
			if i == len(arg) {
				r.fatalf(nil, "Last file type in list argument to %s is missing, i.e., list is ending on: ','", name)
			}
		}
	}
	return true
}

func parseType(ps *parser, e *parserEntry) bool  { return ps.insertType(e, predType, fnType) }
func parseXtype(ps *parser, e *parserEntry) bool { return ps.insertType(e, predXtype, fnXtype) }

func modeToFiletype(mode uint32) (fileType, bool) {
	switch mode & unix.S_IFMT {
	case unix.S_IFREG:
		return ftypeReg, true
	case unix.S_IFDIR:
		return ftypeDir, true
	case unix.S_IFLNK:
		return ftypeLnk, true
	case unix.S_IFBLK:
		return ftypeBlk, true
	case unix.S_IFCHR:
		return ftypeChr, true
	case unix.S_IFSOCK:
		return ftypeSock, true
	case unix.S_IFIFO:
		return ftypeFifo, true
	}
	return ftypeCount, false
}

// typeLetter is print.c mode_to_filetype for %y/%Y.
func typeLetter(mode uint32) byte {
	switch mode & unix.S_IFMT {
	case unix.S_IFREG:
		return 'f'
	case unix.S_IFDIR:
		return 'd'
	case unix.S_IFLNK:
		return 'l'
	case unix.S_IFSOCK:
		return 's'
	case unix.S_IFBLK:
		return 'b'
	case unix.S_IFCHR:
		return 'c'
	case unix.S_IFIFO:
		return 'p'
	}
	return 'U'
}

// predType is pred.c pred_type: the stat mode when known, else d_type.
func predType(r *Run, e *Entry, p *Predicate) bool {
	if e.mode == 0 {
		return false
	}
	mode := e.mode
	if e.haveStat {
		mode = e.st.Mode
	}
	t, ok := modeToFiletype(mode)
	return ok && p.args.types[t]
}

// followingLinks is util.c following_links.
func (r *Run) followingLinks(e *Entry) bool {
	switch r.opt.Symlinks {
	case SymlinkAlwaysDeref:
		return true
	case SymlinkDerefArgsOnly:
		return e.depth == 0
	}
	return false
}

// predXtype is pred.c pred_xtype: the opposite stat flavour, called
// directly (optionl_stat or optionp_stat, never debug_stat), falling back
// to the entry's own type on a broken link (ENOENT or ELOOP).
func predXtype(r *Run, e *Entry, p *Predicate) bool {
	var st unix.Stat_t
	follows := !r.followingLinks(e)
	var err error
	if follows {
		err = unix.Stat(e.sysPath, &st)
		if err == unix.ENOENT || err == unix.ENOTDIR { // fallback_stat
			if r.opt.Debug&DebugStat != 0 {
				r.debugf("fallback_stat(): stat(%s) failed; falling back on lstat()\n", e.accPath)
			}
			err = unix.Lstat(e.sysPath, &st)
		}
	} else {
		err = unix.Lstat(e.sysPath, &st)
	}
	if err != nil {
		if follows && (err == unix.ELOOP || err == unix.ENOENT) {
			return predType(r, e, p)
		}
		r.errorf(err, "%s", r.quote(e.path))
		r.setFailure()
		return false
	}
	t, ok := modeToFiletype(st.Mode)
	return ok && p.args.types[t]
}
```
Add `types [ftypeCount]bool` to `predArgs`. `-xtype` calls `optionl_stat`/`optionp_stat` directly, so it prints no `debug_stat` line, only `fallback_stat`'s.

- [ ] **Step 3: Wire, build, gate**

```go
		{argTest, "type", parseType, predType, fnType},
		{argTest, "xtype", parseXtype, predXtype, fnXtype},
```
```bash
bundle exec rake qfind:build
bundle exec rspec spec/find/tests/type_spec.rb spec/find/tests/xtype_spec.rb spec/find/options/optimisation_spec.rb spec/find/global_options spec/find/options/symbolic_links_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
```
Expected:
- 0 failures in `type_spec.rb`.
- In `xtype_spec.rb`, only the `xtype.exp` `-printf` example fails, until Task 7.
- The `-O2: categorising predicate -type f by cost (Type)` example passes.

- [ ] **Step 4: Commit**

```bash
git add internal && git commit -m "qfind: -type and -xtype"
```

---

## Task 7: Output files and the `-printf` family (`-printf`, `-fprintf`, `-fprint`, `-fprint0`)

This task ports the following:
- `parser.c`: `open_output_file`, `parse_printf`, `parse_fprintf`, `parse_fprint`, `parse_fprint0`, `insert_fprint`;
- `sharefile.c`;
- `print.c` in full: `make_segment`, `insert_fprintf`, `do_fprintf`, `format_date`, `do_time_format`, `ctime_format`, `file_sparseness`, `pred_fprintf`.

It also adds the support those directives need:
- `internal/filemode` (gnulib `filemode.c`);
- `internal/idcache` (uid/gid names);
- `internal/mounts` (`%F`, reused by `-fstype` in Task 13);
- a C `printf` emulation for `%s %d %o %g` with C flag semantics;
- a glibc C-locale `strftime`.

**Files:**
- Create: `internal/find/printf.go` (compiler), `internal/find/printf_render.go` (directives), `internal/find/printf_time.go`, `internal/find/cprintf.go`, `internal/find/sharefile.go`, `internal/find/parse_print.go`
- Create: `internal/filemode/filemode.go`, `internal/idcache/idcache.go`, `internal/mounts/mounts.go`, `internal/libc/strftime.go`
- Modify: `internal/find/output.go` (`flushFilesQuietly`; the `shareRegistry` field), `internal/find/parsetable.go` (5 entries), `internal/find/predicate.go` (`predArgs.format`)

**Interfaces:**
- Consumes: Task 3's `emit`, `emitQuoted`, `commit`, `flushStream` and `sink`; Task 4's `Entry.startLen`; Task 5's `baseName` and `readlink`; Task 6's `typeLetter` and `followingLinks`; `libc.Locale`; `r.tz`.
- Produces:
  - `(*parser).openOutputFile(path string) *Stream`, `*sharedFile` implementing `sink`
  - `(*Run).closeSharedFiles()` (called by `cleanup`)
  - `filemode.String(mode uint32) string` (10 chars)
  - `idcache.UserName(uid uint32) (string, bool)`, `idcache.GroupName(gid uint32) (string, bool)`, `idcache.LookupUser(name string) (uint32, bool)`, `idcache.LookupGroup(name string) (uint32, bool)`, `idcache.UserExists(uid uint32) bool`, `idcache.GroupExists(gid uint32) bool`
  - `mounts.FSType(dev uint64) (string, bool)` ("unknown" if none; false if the list is unreadable), `mounts.HasType(name string) bool`, `mounts.SELinuxEnabled() bool`
  - `(*Run).fsType(e *Entry) string`, `(*Run).getfilecon(e *Entry) (string, error)`, `(*Run).renderDirective`, `predFprintf`
  - `libc.Strftime(format string, t time.Time) string`
  - `parseCspec(s string) cspec`, `cfmtS(c cspec, s string) string`, `cfmtD(c cspec, v int64) string`, `cfmtO(c cspec, v uint64) string`, `cfmtG(c cspec, v float64) string`
  - `(*Run).formatDate(ts Timespec, kind byte) string`, `(*Run).ctimeFormat(ts Timespec) string`

- [ ] **Step 1: Failing gates**

```bash
bundle exec rspec spec/find/actions/{printf,printf_escapes,printf_names,printf_attributes,printf_times,fprint,fprint0,fprintf}_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
```

- [ ] **Step 2: `sharefile.go` and `output.go` — output files shared by (dev, ino)**

```go
package find

import (
	"os"
	"sync"

	"golang.org/x/sys/unix"
)

// sharedFile is one FILE* from sharefile.c: every alias of the same
// (dev, ino) shares this stdio stream. It is a sink (output.go).
type sharedFile struct {
	mu   sync.Mutex
	name string // the first name it was opened under (error messages)
	f    *os.File
	c    *cfile
}

func (sf *sharedFile) lock()                { sf.mu.Lock() }
func (sf *sharedFile) unlock()              { sf.mu.Unlock() }
func (sf *sharedFile) write(b []byte) error { return sf.c.write(b) }
func (sf *sharedFile) flush() error         { return sf.c.flushBuf() }

type devIno struct{ dev, ino uint64 }

type shareRegistry struct {
	mu    sync.Mutex
	files map[devIno]*sharedFile
	order []*sharedFile
}

// openOutputFile is parser.c open_output_file. The exact strings
// "/dev/stdout" and "/dev/stderr" are the real streams, never reopened.
func (ps *parser) openOutputFile(path string) *Stream {
	r := ps.r
	switch path {
	case "/dev/stderr":
		return r.out.stderrStream
	case "/dev/stdout":
		return r.out.stdoutStream
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		r.fatalNontargetFileError(err, path)
	}
	var st unix.Stat_t
	_ = unix.Fstat(int(f.Fd()), &st)
	key := devIno{st.Dev, st.Ino}
	reg := &r.shared
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if reg.files == nil {
		reg.files = map[devIno]*sharedFile{}
	}
	sf, ok := reg.files[key]
	if ok {
		f.Close()
	} else {
		sf = &sharedFile{name: path, f: f, c: newCfile(f)}
		reg.files[key] = sf
		reg.order = append(reg.order, sf)
	}
	return &Stream{name: path, kind: streamFile, tty: isatty(int(sf.f.Fd())), sink: sf}
}

// fatalNontargetFileError is util.c fatal_nontarget_file_error.
func (r *Run) fatalNontargetFileError(err error, name string) {
	r.errorf(err, "%s", r.quote(name))
	r.exit(1)
}

// closeSharedFiles is sharefile_destroy: flush and close every file; a
// failure is fatal, reported under the first-opened name.
func (r *Run) closeSharedFiles() {
	reg := &r.shared
	reg.mu.Lock()
	files := reg.order
	reg.order, reg.files = nil, nil
	reg.mu.Unlock()
	for _, sf := range files {
		sf.lock()
		// entry_free: fclose fails only if flushing what is buffered now, or
		// close(2), fails; an earlier failure that emptied the buffer does not.
		err := sf.c.flushBuf()
		if cerr := sf.f.Close(); err == nil {
			err = cerr
		}
		sf.unlock()
		if err != nil {
			r.fatalNontargetFileError(err, sf.name)
		}
	}
}
```
Changes elsewhere:
- Add `shared shareRegistry` to `Run`.
- In `output.go`, fill `flushFilesQuietly`. exit() flushes every stdio `FILE` and ignores errors here; only `fclose` in cleanup reports them:
```go
func (o *Output) flushFilesQuietly() {
	reg := &o.r.shared
	reg.mu.Lock()
	files := append([]*sharedFile(nil), reg.order...)
	reg.mu.Unlock()
	for _, sf := range files {
		sf.lock()
		sf.flush()
		sf.unlock()
	}
}
```
- `cleanup()` becomes:
  1. pending execs (Task 8);
  2. `r.closeSharedFiles()`;
  3. `r.cleanupFlush()`.

- [ ] **Step 3: `cprintf.go` — C printf for the four conversions find emits**

```go
package find

import (
	"math"
	"strconv"
	"strings"
)

// cspec is the "%[-+ #][width][.prec]" part of a segment, parsed as C
// does (a leading 0 in the width is the 0 flag).
type cspec struct {
	minus, plus, space, hash, zero bool
	width                          int
	prec                           int // -1 when absent
}

func parseCspec(s string) cspec { // s = flags/width/prec text after '%'
	c := cspec{prec: -1}
	i := 0
	for ; i < len(s) && strings.IndexByte("-+ #", s[i]) >= 0; i++ {
		switch s[i] {
		case '-':
			c.minus = true
		case '+':
			c.plus = true
		case ' ':
			c.space = true
		case '#':
			c.hash = true
		}
	}
	if i < len(s) && s[i] == '0' {
		c.zero = true
	}
	j := i
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	c.width, _ = strconv.Atoi(s[i:j])
	if j < len(s) && s[j] == '.' {
		c.prec, _ = strconv.Atoi(s[j+1:])
	}
	return c
}

func pad(c cspec, body string, zeroOK bool) string {
	if len(body) >= c.width {
		return body
	}
	n := c.width - len(body)
	switch {
	case c.minus:
		return body + strings.Repeat(" ", n)
	case c.zero && zeroOK:
		sign := ""
		if body != "" && (body[0] == '-' || body[0] == '+' || body[0] == ' ') {
			sign, body = body[:1], body[1:]
		}
		return sign + strings.Repeat("0", n) + body
	}
	return strings.Repeat(" ", n) + body
}

// cfmtS is %s: precision truncates bytes, only '-' and width matter.
func cfmtS(c cspec, s string) string {
	if c.prec >= 0 && len(s) > c.prec {
		s = s[:c.prec]
	}
	c.zero = false
	return pad(c, s, false)
}

// cfmtD is %d with C semantics (precision = minimum digits, disables 0).
func cfmtD(c cspec, v int64) string {
	digits := strconv.FormatUint(uint64(abs64(v)), 10)
	if c.prec >= 0 {
		if c.prec == 0 && v == 0 {
			digits = ""
		}
		for len(digits) < c.prec {
			digits = "0" + digits
		}
	}
	sign := ""
	switch {
	case v < 0:
		sign = "-"
	case c.plus:
		sign = "+"
	case c.space:
		sign = " "
	}
	return pad(c, sign+digits, c.prec < 0)
}

// cfmtO is %o: '#' forces a leading 0; '+' and ' ' do nothing.
func cfmtO(c cspec, v uint64) string {
	digits := strconv.FormatUint(v, 8)
	if c.prec >= 0 {
		if c.prec == 0 && v == 0 {
			digits = ""
		}
		for len(digits) < c.prec {
			digits = "0" + digits
		}
	}
	if c.hash && !strings.HasPrefix(digits, "0") {
		digits = "0" + digits
	}
	return pad(c, digits, c.prec < 0)
}

// cfmtG is %g: precision P (default 6, 0 means 1), %e style when the
// exponent X < -4 or X >= P, trailing zeros removed unless '#'.
func cfmtG(c cspec, v float64) string {
	sign := ""
	switch {
	case math.Signbit(v):
		sign, v = "-", -v
	case c.plus:
		sign = "+"
	case c.space:
		sign = " "
	}
	if math.IsInf(v, 0) {
		return pad(cspec{minus: c.minus, width: c.width}, sign+"inf", false)
	}
	p := c.prec
	if p < 0 {
		p = 6
	} else if p == 0 {
		p = 1
	}
	var body string
	x := 0
	if v != 0 {
		x = int(math.Floor(math.Log10(v)))
		if r := strconv.FormatFloat(v, 'e', p-1, 64); r != "" { // rounding may bump X
			x, _ = strconv.Atoi(r[strings.IndexByte(r, 'e')+1:])
		}
	}
	if x < -4 || x >= p {
		body = strconv.FormatFloat(v, 'e', p-1, 64)
		mant, exp, _ := strings.Cut(body, "e")
		if !c.hash {
			mant = trimZeros(mant)
		}
		e, _ := strconv.Atoi(exp)
		es := "+"
		if e < 0 {
			es, e = "-", -e
		}
		body = mant + "e" + es + twoDigits(e)
	} else {
		body = strconv.FormatFloat(v, 'f', p-1-x, 64)
		if !c.hash {
			body = trimZeros(body)
		} else if !strings.Contains(body, ".") {
			body += "."
		}
	}
	return pad(c, sign+body, true)
}

func trimZeros(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

func twoDigits(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
```
Check this against `printf_attributes_spec.rb`'s tables, for example `%#S` → `1.00000`, `%05S` → `00001`, `%+05d` → `+0001`, `%-05d` → `1    `, `%.3d` → `001`, `%#m` → `0640`, `%06m` → `000640`, `%012M` → `  -rw-r-----`.

- [ ] **Step 4: `printf.go` — `insert_fprintf` and `make_segment`**

```go
package find

import "strings"

type segKind uint8

const (
	segPlain segKind = iota
	segStop
	segFormat
)

// segment is struct segment (print.c): literal text, or a directive with
// its literal prefix, C spec and conversion.
type segment struct {
	kind   segKind
	text   string // plain/stop text, or the directive's literal prefix
	spec   cspec
	raw    string // prefix + "%" + spec text, as GNU writes for %% (text_len bytes)
	conv   byte   // 's', 'd', 'o', 'g' or '%'
	fc     byte   // format_char[0]
	fc2    byte   // format_char[1] for A, B, C, T
}

type formatVal struct {
	out      *Stream
	segments []segment
}

// insertFprintf is print.c insert_fprintf.
func (ps *parser) insertFprintf(out *Stream, e *parserEntry, format string) bool {
	r := ps.r
	p := ps.insertPrimaryWithPred(e, predFprintf, fnFprintf, format, true)
	p.sideEffects, p.noDefaultPrint = true, true
	p.needType, p.needStat = false, false
	p.cost = NeedsNothing
	fv := &formatVal{out: out}
	p.args.format = fv
	var lit strings.Builder // text since the last segment
	b := []byte(format)
	for i := 0; i < len(b); i++ {
		switch {
		case b[i] == '\\' && i+1 < len(b) && b[i+1] == 'c':
			r.formatFlushes = true // a mid-evaluation flush: evaluation goes serial (Task 4)
			fv.segments = append(fv.segments, segment{kind: segStop, text: lit.String()})
			if p.needStat && p.cost < NeedsStatInfo {
				p.cost = NeedsStatInfo
			}
			return true
		case b[i] == '\\':
			if i+1 == len(b) {
				r.errorf(nil, "warning: escape `\\' followed by nothing at all")
				lit.WriteByte('\\')
				continue
			}
			c := b[i+1]
			switch {
			case c >= '0' && c <= '7':
				n, j := 0, i+1
				for k := 0; k < 3 && j < len(b) && b[j] >= '0' && b[j] <= '7'; k, j = k+1, j+1 {
					n = 8*n + int(b[j]-'0')
				}
				lit.WriteByte(byte(n))
				i = j - 1
			case strings.IndexByte("abfnrtv\\", c) >= 0:
				lit.WriteByte("\a\b\f\n\r\t\v\\"[strings.IndexByte("abfnrtv\\", c)])
				i++
			default:
				r.errorf(nil, "warning: unrecognized escape `\\%s'", cbyte(c))
				lit.WriteByte('\\')
				lit.WriteByte(c)
				i++
			}
			fv.segments = append(fv.segments, segment{kind: segPlain, text: lit.String()})
			lit.Reset()
		case b[i] == '%':
			if i+1 == len(b) {
				r.fatalf(nil, "error: %s at end of format string", string(b[i:]))
			}
			start := i
			n := 1
			if b[i+1] != '%' {
				n = formatFlagsLength(b[i:])
			}
			i += n
			var dc byte
			if i < len(b) {
				dc = b[i]
			}
			size := specifierLength(dc)
			if size > 0 && i+size-1 < len(b) && (size == 1 || b[i+1] != 0) {
				seg := segment{kind: segFormat, text: lit.String(), raw: lit.String() + string(b[start:i]),
					spec: parseCspec(string(b[start+1 : i])), fc: dc}
				if size == 2 {
					seg.fc2 = b[i+1]
				}
				makeSegment(&seg, p)
				fv.segments = append(fv.segments, seg)
				lit.Reset()
				i += size - 1
				continue
			}
			if dc == '{' || dc == '[' || dc == '(' || dc == 0 {
				r.fatalf(nil, "error: the format directive `%%%s' is reserved for future use", cbyte(dc))
			}
			if size == 2 {
				r.errorf(nil, "warning: format directive `%%%s' should be followed by another character", cbyte(dc))
			} else {
				r.errorf(nil, "warning: unrecognized format directive `%%%s'", cbyte(dc))
			}
			lit.Write(b[start : i+1])
			fv.segments = append(fv.segments, segment{kind: segPlain, text: lit.String()})
			lit.Reset()
		default:
			lit.WriteByte(b[i])
		}
	}
	if lit.Len() > 0 {
		fv.segments = append(fv.segments, segment{kind: segPlain, text: lit.String()})
	}
	return true
}

// formatFlagsLength is print.c get_format_flags_length.
func formatFlagsLength(p []byte) int {
	n := 1
	for n < len(p) && strings.IndexByte("-+ #", p[n]) >= 0 {
		n++
	}
	for n < len(p) && p[n] >= '0' && p[n] <= '9' {
		n++
	}
	if n < len(p) && p[n] == '.' {
		for n++; n < len(p) && p[n] >= '0' && p[n] <= '9'; n++ {
		}
	}
	return n
}

// specifierLength is print.c get_format_specifer_length.
func specifierLength(c byte) int {
	switch {
	case c != 0 && strings.IndexByte("abcdDfFgGhHiklmMnpPsStuUyYZ%", c) >= 0:
		return 1
	case c != 0 && strings.IndexByte("ABCT", c) >= 0:
		return 2
	}
	return 0
}

// makeSegment is print.c make_segment: conversion letter and costs.
func makeSegment(s *segment, p *Predicate) {
	my := NeedsNothing
	s.conv = 's'
	switch s.fc {
	case '%':
		s.conv = '%'
	case 'l':
		p.needStat, my = true, NeedsLinkName
	case 'y':
		p.needType, my = true, NeedsType
	case 'i':
		p.needInum, my = true, NeedsInodeNumber
	case 'a', 'A', 'B', 'c', 'C', 'F', 'g', 'M', 's', 't', 'T', 'u', 'G', 'U', 'b', 'D', 'k', 'n':
		p.needStat, my = true, NeedsStatInfo
	case 'S':
		p.needStat, my, s.conv = true, NeedsStatInfo, 'g'
	case 'Y':
		p.needStat, my = true, NeedsType
	case 'Z':
		my = NeedsAccessInfo
	case 'd':
		s.conv = 'd'
	case 'm':
		s.conv, p.needStat, my = 'o', true, NeedsStatInfo
	}
	if my > p.cost {
		p.cost = my
	}
}
```
**Port check** against `print.c:298-428`:
- `%%` makes a format segment that writes `raw`: the prefix plus everything up to the second `%`, so `[%5%]` → `[%5`.
- An ABCT directive followed by NUL warns "should be followed by another character".
- The `%-5` bug at the end of a format: `fmt_editpos` lands on the NUL, `strchr("{[(", 0)` matches the terminator, and the reserved-directive message prints a raw NUL. The spec checks only the `find: error: ` prefix and the rejection, so `fatalf` with `dc == 0` reproduces it.

- [ ] **Step 5: `printf_render.go` — `pred_fprintf` and `do_fprintf`**

Each directive is `checked_fprintf(dest, segment->text, value)`: the segment's literal prefix, then the value formatted with the segment's C spec and conversion. The `print_quoted` directives (`%f %F %h %l %p %P`) first replace non-printable characters with `?` when the stream is a terminal. `%H` and `%P` use `Entry.startLen` (Task 4).

```go
package find

import (
	"math"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"qfind/internal/filemode"
	"qfind/internal/idcache"
	"qfind/internal/mounts"
)

// predFprintf is print.c pred_fprintf (-printf and -fprintf). Every
// segment is one checked stdio call (checked_fwrite, checked_fprintf,
// checked_fflush): a failed write is diagnosed under the stream's name.
func predFprintf(r *Run, e *Entry, p *Predicate) bool {
	fv := p.args.format
	for i := range fv.segments {
		seg := &fv.segments[i]
		switch seg.kind {
		case segPlain:
			r.emitChecked(e, fv.out, seg.text, failReported)
		case segStop: // \c: the last segment; flush the format's stream
			r.emitChecked(e, fv.out, seg.text, failReported)
			r.flushStream(e, fv.out, failReported)
		default:
			r.emitChecked(e, fv.out, r.renderDirective(e, fv.out, seg), failReported)
		}
	}
	return true
}

func tsOf(t unix.Timespec) Timespec { return Timespec{t.Sec, t.Nsec} }

// renderDirective is do_fprintf's KIND_FORMAT case, with pred_fprintf's
// date components; the result starts with the segment's literal prefix.
func (r *Run) renderDirective(e *Entry, out *Stream, seg *segment) string {
	st := &e.st
	str := func(v string) string { return seg.text + cfmtS(seg.spec, v) }
	quoted := func(v string) string { // checked_print_quoted
		if out.tty {
			v = r.loc.Qmark(v)
		}
		return str(v)
	}
	num := func(v uint64) string { return str(strconv.FormatUint(v, 10)) } // human_readable(v, …, 1, 1)
	if seg.fc2 != 0 {                                                      // %A? %B? %C? %T?
		var ts Timespec
		switch seg.fc {
		case 'A':
			ts = tsOf(st.Atim)
		case 'C':
			ts = tsOf(st.Ctim)
		case 'T':
			ts = tsOf(st.Mtim)
		case 'B': // no birth time in this build: {-1, -1}, valid for '@' only
			if seg.fc2 != '@' {
				return str("")
			}
			ts = Timespec{-1, -1}
		}
		return str(r.formatDate(ts, seg.fc2))
	}
	switch seg.fc {
	case 'a':
		return str(r.ctimeFormat(tsOf(st.Atim)))
	case 'c':
		return str(r.ctimeFormat(tsOf(st.Ctim)))
	case 't':
		return str(r.ctimeFormat(tsOf(st.Mtim)))
	case 'b': // human_readable(st_blocks, ceiling, 512, 512)
		return num(uint64(st.Blocks))
	case 'k': // human_readable(st_blocks, ceiling, 512, 1024)
		return num((uint64(st.Blocks) + 1) / 2)
	case 'd':
		return seg.text + cfmtD(seg.spec, int64(e.depth))
	case 'D':
		return num(st.Dev)
	case 'f':
		return quoted(baseName(e.path))
	case 'F':
		return quoted(r.fsType(e))
	case 'g':
		if name, ok := idcache.GroupName(st.Gid); ok {
			return str(name)
		}
		return num(uint64(st.Gid))
	case 'G':
		return num(uint64(st.Gid))
	case 'h':
		return quoted(dirnameH(e.path))
	case 'H':
		return str(e.path[:e.startLen])
	case 'i':
		return num(st.Ino)
	case 'l':
		target := ""
		if st.Mode&unix.S_IFMT == unix.S_IFLNK {
			t, err := readlink(e.sysPath)
			if err != nil {
				r.nonfatalTargetFileError(e, err)
				r.setFailure()
			}
			target = t
		}
		return quoted(target)
	case 'M':
		return str(filemode.String(st.Mode))
	case 'm':
		return seg.text + cfmtO(seg.spec, uint64(st.Mode&07777))
	case 'n':
		return num(uint64(st.Nlink))
	case 'p':
		return quoted(e.path)
	case 'P':
		rest := ""
		if e.depth > 0 {
			rest = e.path[e.startLen:]
			if rest != "" && rest[0] == '/' { // the separator fts added
				rest = rest[1:]
			}
		}
		return quoted(rest)
	case 's':
		return num(uint64(st.Size))
	case 'S':
		return seg.text + cfmtG(seg.spec, sparseness(st))
	case 'u':
		if name, ok := idcache.UserName(st.Uid); ok {
			return str(name)
		}
		return num(uint64(st.Uid))
	case 'U':
		return num(uint64(st.Uid))
	case 'Y':
		return str(r.linkTargetType(e))
	case 'y':
		return str(string(typeLetter(e.mode)))
	case 'Z':
		ctx, err := r.getfilecon(e)
		if err != nil {
			r.errorf(err, "getfilecon failed: %s", r.quote(e.path))
			r.setFailure()
			return str("")
		}
		return str(ctx)
	case '%':
		return seg.raw
	}
	return seg.text
}

// sparseness is print.c file_sparseness.
func sparseness(st *unix.Stat_t) float64 {
	if st.Size == 0 {
		switch {
		case st.Blocks == 0:
			return 1.0
		case st.Blocks < 0:
			return math.Inf(-1)
		}
		return math.Inf(1)
	}
	return 512 * float64(st.Blocks) / float64(st.Size)
}

// linkTargetType is %Y: the target's type letter for a symlink, N when it
// dangles (ENOENT/ENOTDIR), L for a loop; any other error prints '?' and a
// diagnostic without changing the exit status (print.c).
func (r *Run) linkTargetType(e *Entry) string {
	if e.st.Mode&unix.S_IFMT != unix.S_IFLNK {
		return string(typeLetter(e.st.Mode))
	}
	var st unix.Stat_t
	switch err := unix.Stat(e.sysPath, &st); err {
	case nil:
		return string(typeLetter(st.Mode))
	case unix.ENOENT, unix.ENOTDIR:
		return "N"
	case unix.ELOOP:
		return "L"
	default:
		r.errorf(err, "%s", r.quote(e.path))
		return "?"
	}
}

// fsType is fstype.c filesystem_type for %F and -fstype.
func (r *Run) fsType(e *Entry) string {
	t, ok := mounts.FSType(e.st.Dev)
	if !ok {
		r.fatalAt(e, nil, "Cannot read mounted file system list")
	}
	return t
}

// getfilecon is options.x_getfilecon (libselinux lgetfilecon/getfilecon
// per -P/-L/-H): the security.selinux attribute without its trailing NUL.
func (r *Run) getfilecon(e *Entry) (string, error) {
	get := unix.Lgetxattr
	if r.followingLinks(e) {
		get = unix.Getxattr
	}
	buf := make([]byte, 256)
	for {
		n, err := get(e.sysPath, "security.selinux", buf)
		if err == unix.ERANGE {
			buf = make([]byte, 2*len(buf))
			continue
		}
		if err != nil {
			return "", err
		}
		return strings.TrimSuffix(string(buf[:n]), "\x00"), nil
	}
}
```

```go
// dirnameH is print.c's %h (lines 957-985).
func dirnameH(path string) string {
	s := len(path) - 1
	for s >= 0 && path[s] == '/' {
		s--
	}
	p := path
	if s > 0 && s+1 < len(path) && path[s+1] == '/' {
		p = path[:s+1]
	}
	i := strings.LastIndexByte(p, '/')
	if i < 0 {
		return "."
	}
	return p[:i]
}
```

- [ ] **Step 6: `printf_time.go` — `ctime_format`, `format_date`, `do_time_format`**

```go
package find

import (
	"fmt"
	"strconv"
	"time"

	"qfind/internal/libc"
)

var weekdays = [...]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
var months = [...]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// localtime is localtime(3) in find's zone; false when the year does not
// fit a C int (glibc fails with EOVERFLOW).
func (r *Run) localtime(sec int64) (time.Time, bool) {
	t := time.Unix(sec, 0).In(r.tz)
	y := int64(t.Year()) - 1900
	return t, y == int64(int32(y))
}

// ctimeFormat is print.c ctime_format: "%3s %3s %2d %02d:%02d:%02d.%09ld0 %04d",
// or the %@ form when localtime fails.
func (r *Run) ctimeFormat(ts Timespec) string {
	t, ok := r.localtime(ts.Sec)
	if !ok {
		return r.formatDate(ts, '@')
	}
	return fmt.Sprintf("%3s %3s %2d %02d:%02d:%02d.%09d0 %04d", weekdays[t.Weekday()],
		months[t.Month()-1], t.Day(), t.Hour(), t.Minute(), t.Second(), ts.Nsec, t.Year())
}

// formatDate is print.c format_date. The nanosecond suffix is
// ".%09ld0"; for the absent birth time {-1,-1} that is ".-000000010", as
// in C (Go's %09d pads a negative number the same way).
func (r *Run) formatDate(ts Timespec, kind byte) string {
	format := "%" + string(kind)
	needNS := kind == 'S' || kind == 'T' || kind == 'X' || kind == '@'
	if kind == '+' {
		format, needNS = "%F+%T", true
	}
	ns := ""
	if needNS {
		ns = fmt.Sprintf(".%09d0", ts.Nsec)
	}
	if kind != '@' {
		if t, ok := r.localtime(ts.Sec); ok {
			return doTimeFormat(format, t, ns)
		}
	}
	// %@, or localtime failed: human_readable of the magnitude, signed.
	if ts.Sec < 0 {
		return "-" + strconv.FormatUint(uint64(-(ts.Sec+1))+1, 10) + ns
	}
	return strconv.FormatUint(uint64(ts.Sec), 10) + ns
}

// doTimeFormat is print.c do_time_format: strftime "_"+fmt, then again
// with tm_sec ± 11. If exactly one run of 2 differing digits followed by a
// non-digit is found, the nanoseconds are inserted after it.
func doTimeFormat(format string, t time.Time, ns string) string {
	buf := libc.Strftime("_"+format, t)
	alt := t.Add(-11 * time.Second)
	if t.Second() < 11 {
		alt = t.Add(11 * time.Second)
	}
	altBuf := libc.Strftime("_"+format, alt)
	if i, n, ok := digitDiff(buf, altBuf); ok && n == 2 && (i+n >= len(buf) || !isDigit(buf[i+n])) {
		buf = buf[:i+n] + ns + buf[i+n:]
	}
	return buf[1:]
}

// digitDiff is print.c scan_for_digit_differences.
func digitDiff(p, q string) (first, n int, ok bool) {
	if len(p) != len(q) {
		return 0, 0, false
	}
	seen := false
	for i := 0; i < len(p); i++ {
		if p[i] == q[i] {
			continue
		}
		if !isDigit(p[i]) || !isDigit(q[i]) {
			return 0, 0, false
		}
		switch {
		case !seen:
			first, n, seen = i, 1, true
		case i-first == n:
			n++
		default:
			return 0, 0, false
		}
	}
	return first, n, true
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
```
**Edge.** Changing `tm_sec` by ±11 in C keeps every other field. `time.Add` can roll the minute over, but only when the seconds would leave 0–59, and the ±11 rule prevents that. So `alt` differs only in the seconds digits, as in C.

Also note the B-time output `%B@` → `-1.-000000010`: `Sprintf(".%09d0", -1)` gives `.-000000010`, which matches C's `%09ld` of -1.

- [ ] **Step 7: `internal/libc/strftime.go` — glibc strftime in the C/POSIX LC_TIME**

find hands strftime one conversion at a time: `"%"+kind` from `-printf %Ak` (with do_time_format's `"_"` prefix), `"%F+%T"` and `-ls`'s two formats. So no flags or widths are needed. glibc's behaviour for these, checked against the C library through `python3 -c 'import time; time.strftime(...)'`:
- the year-like `%Y %C %G` and `%F`'s year are not padded (year 999 prints `999`, `999-01-01`, `9`);
- an unknown conversion, or one cut off by the end of the format, is copied literally (`%Q`, `%E`);
- one `E` or `O` modifier is accepted and ignored.

```go
package libc

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	wdayNames = [...]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	monNames  = [...]string{"January", "February", "March", "April", "May", "June", "July",
		"August", "September", "October", "November", "December"}
)

// Strftime is glibc strftime(3) under LC_TIME=C for the formats find uses.
func Strftime(format string, t time.Time) string {
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			b.WriteByte(format[i])
			continue
		}
		j := i + 1
		if j < len(format) && (format[j] == 'E' || format[j] == 'O') {
			j++
		}
		if j >= len(format) {
			b.WriteString(format[i:]) // bad_format at the end
			break
		}
		if s, ok := convert(format[j], t); ok {
			b.WriteString(s)
		} else {
			b.WriteString(format[i : j+1])
		}
		i = j
	}
	return b.String()
}

func pad2(v int) string { return fmt.Sprintf("%02d", v) }

// isoWeekDays is strftime_l.c iso_week_days (Monday weeks, Thursday rule).
func isoWeekDays(yday, wday int) int {
	const bigEnoughMultipleOf7 = (366/7 + 2) * 7
	return yday - (yday-wday+4+bigEnoughMultipleOf7)%7 + 4 - 1
}

func isLeap(y int) bool { return y%4 == 0 && (y%100 != 0 || y%400 == 0) }

func convert(c byte, t time.Time) (string, bool) {
	year, yday, wday := t.Year(), t.YearDay()-1, int(t.Weekday())
	tmYear := year - 1900
	hour12 := t.Hour() % 12
	if hour12 == 0 {
		hour12 = 12
	}
	switch c {
	case 'a':
		return wdayNames[wday][:3], true
	case 'A':
		return wdayNames[wday], true
	case 'b', 'h':
		return monNames[t.Month()-1][:3], true
	case 'B':
		return monNames[t.Month()-1], true
	case 'c':
		return Strftime("%a %b %e %H:%M:%S %Y", t), true
	case 'C':
		c := tmYear/100 + 19
		if tmYear%100 < 0 && year > 0 {
			c--
		}
		return strconv.Itoa(c), true
	case 'd':
		return pad2(t.Day()), true
	case 'D', 'x':
		return Strftime("%m/%d/%y", t), true
	case 'e':
		return fmt.Sprintf("%2d", t.Day()), true
	case 'F':
		return Strftime("%Y-%m-%d", t), true
	case 'g', 'G', 'V':
		adjust := 0
		days := isoWeekDays(yday, wday)
		if days < 0 {
			adjust = -1
			n := 365
			if isLeap(year - 1) {
				n = 366
			}
			days = isoWeekDays(yday+n, wday)
		} else {
			n := 365
			if isLeap(year) {
				n = 366
			}
			if d := isoWeekDays(yday-n, wday); d >= 0 {
				adjust, days = 1, d
			}
		}
		switch c {
		case 'g':
			yy := (tmYear%100 + adjust) % 100
			if yy < 0 {
				if tmYear < -1900-adjust {
					yy = -yy
				} else {
					yy += 100
				}
			}
			return pad2(yy), true
		case 'G':
			return strconv.Itoa(year + adjust), true
		}
		return pad2(days/7 + 1), true
	case 'H':
		return pad2(t.Hour()), true
	case 'I':
		return pad2(hour12), true
	case 'j':
		return fmt.Sprintf("%03d", yday+1), true
	case 'k':
		return fmt.Sprintf("%2d", t.Hour()), true
	case 'l':
		return fmt.Sprintf("%2d", hour12), true
	case 'm':
		return pad2(int(t.Month())), true
	case 'M':
		return pad2(t.Minute()), true
	case 'n':
		return "\n", true
	case 'p':
		if t.Hour() < 12 {
			return "AM", true
		}
		return "PM", true
	case 'P':
		if t.Hour() < 12 {
			return "am", true
		}
		return "pm", true
	case 'r':
		return Strftime("%I:%M:%S %p", t), true
	case 'R':
		return Strftime("%H:%M", t), true
	case 's':
		return strconv.FormatInt(t.Unix(), 10), true
	case 'S':
		return pad2(t.Second()), true
	case 't':
		return "\t", true
	case 'T', 'X':
		return Strftime("%H:%M:%S", t), true
	case 'u':
		if wday == 0 {
			return "7", true
		}
		return strconv.Itoa(wday), true
	case 'U':
		return pad2((yday - wday + 7) / 7), true
	case 'w':
		return strconv.Itoa(wday), true
	case 'W':
		return pad2((yday - (wday-1+7)%7 + 7) / 7), true
	case 'y':
		yy := tmYear % 100
		if yy < 0 {
			if tmYear < -1900 {
				yy = -yy
			} else {
				yy += 100
			}
		}
		return pad2(yy), true
	case 'Y':
		return strconv.Itoa(year), true
	case 'z':
		_, off := t.Zone()
		sign := '+'
		if off < 0 {
			sign, off = '-', -off
		}
		return fmt.Sprintf("%c%02d%02d", sign, off/3600, off/60%60), true
	case 'Z':
		name, _ := t.Zone()
		return name, true
	case '%':
		return "%", true
	}
	return "", false
}
```
Run `printf_times_spec.rb` after Step 9. It checks 32 conversions against Ruby's `strftime` with exact expected strings. For years before 1000 and the ISO-week edges, compare with `python3 -c 'import time; print(time.strftime("%G-%V %g %U %W %C %y", (2021, 1, 1, 0, 0, 0, 4, 1, 0)))'` for a handful of boundary dates (Dec 29 – Jan 4 of 2020/2021/2027).

- [ ] **Step 8: `internal/filemode`, `internal/idcache`, `internal/mounts`**

`filemode.go`:
```go
// Package filemode is gnulib filemode.c strmode (without the trailing space).
package filemode

import "golang.org/x/sys/unix"

func ftypelet(m uint32) byte {
	switch m & unix.S_IFMT {
	case unix.S_IFREG:
		return '-'
	case unix.S_IFDIR:
		return 'd'
	case unix.S_IFBLK:
		return 'b'
	case unix.S_IFCHR:
		return 'c'
	case unix.S_IFLNK:
		return 'l'
	case unix.S_IFIFO:
		return 'p'
	case unix.S_IFSOCK:
		return 's'
	}
	return '?'
}

// String is the 10-character mode string, e.g. "-rwsr-xr-t".
func String(m uint32) string {
	b := []byte{ftypelet(m), '-', '-', '-', '-', '-', '-', '-', '-', '-'}
	set := func(i int, bit uint32, c byte) {
		if m&bit != 0 {
			b[i] = c
		}
	}
	set(1, 0o400, 'r')
	set(2, 0o200, 'w')
	set(4, 0o040, 'r')
	set(5, 0o020, 'w')
	set(7, 0o004, 'r')
	set(8, 0o002, 'w')
	b[3] = special(m&0o100 != 0, m&unix.S_ISUID != 0, 'x', 's', 'S')
	b[6] = special(m&0o010 != 0, m&unix.S_ISGID != 0, 'x', 's', 'S')
	b[9] = special(m&0o001 != 0, m&unix.S_ISVTX != 0, 'x', 't', 'T')
	return string(b)
}

func special(x, s bool, plain, both, only byte) byte {
	switch {
	case s && x:
		return both
	case s:
		return only
	case x:
		return plain
	}
	return '-'
}
```
`idcache.go`:
```go
// Package idcache maps user and group IDs to names and back through the C
// library's NSS: with cgo, os/user calls getpwuid_r, getgrgid_r,
// getpwnam_r and getgrnam_r. Hits and misses are cached for the run, as
// gnulib's idcache does, and are safe for concurrent use.
package idcache

import (
	"os/user"
	"strconv"
	"sync"
)

type name struct {
	s  string
	ok bool
}

type id struct {
	n  uint32
	ok bool
}

var users, groups, userIDs, groupIDs sync.Map

func cached[K comparable, V any](m *sync.Map, k K, load func() V) V {
	if v, ok := m.Load(k); ok {
		return v.(V)
	}
	v := load()
	m.Store(k, v)
	return v
}

// UserName is getpwuid(uid)->pw_name.
func UserName(uid uint32) (string, bool) {
	n := cached(&users, uid, func() name {
		if u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10)); err == nil {
			return name{u.Username, true}
		}
		return name{}
	})
	return n.s, n.ok
}

// GroupName is getgrgid(gid)->gr_name.
func GroupName(gid uint32) (string, bool) {
	n := cached(&groups, gid, func() name {
		if g, err := user.LookupGroupId(strconv.FormatUint(uint64(gid), 10)); err == nil {
			return name{g.Name, true}
		}
		return name{}
	})
	return n.s, n.ok
}

// UserExists is getpwuid(uid) != NULL (-nouser).
func UserExists(uid uint32) bool { _, ok := UserName(uid); return ok }

// GroupExists is getgrgid(gid) != NULL (-nogroup).
func GroupExists(gid uint32) bool { _, ok := GroupName(gid); return ok }

// LookupUser is getpwnam(s)->pw_uid; getpwnam("") is NULL.
func LookupUser(s string) (uint32, bool) {
	v := cached(&userIDs, s, func() id {
		if u, err := user.Lookup(s); s != "" && err == nil {
			if n, err := strconv.ParseUint(u.Uid, 10, 32); err == nil {
				return id{uint32(n), true}
			}
		}
		return id{}
	})
	return v.n, v.ok
}

// LookupGroup is getgrnam(s)->gr_gid.
func LookupGroup(s string) (uint32, bool) {
	v := cached(&groupIDs, s, func() id {
		if g, err := user.LookupGroup(s); s != "" && err == nil {
			if n, err := strconv.ParseUint(g.Gid, 10, 32); err == nil {
				return id{uint32(n), true}
			}
		}
		return id{}
	})
	return v.n, v.ok
}
```
`cached`'s type arguments are inferred at each call. The go1.27 compiler crash noted in the toolchain memory needs an uninstantiated generic function used as a value, which this code never does.

`mounts.go`:
```go
// Package mounts maps devices to file system types from
// /proc/self/mountinfo (gnulib mountlist.c read_file_system_list).
package mounts

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sys/unix"
)

var (
	once     sync.Once
	readable bool              // read_file_system_list succeeded
	types    map[uint64]string // dev → type; later lines win (fstype.c keeps the last match)
	names    map[string]bool
)

func load() {
	types, names = map[uint64]string{}, map[string]bool{}
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return
	}
	defer f.Close()
	readable = true
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		sep := -1
		for i, fld := range fields {
			if fld == "-" {
				sep = i
				break
			}
		}
		if len(fields) < 5 || sep < 0 || sep+1 >= len(fields) {
			continue
		}
		majmin := strings.SplitN(fields[2], ":", 2)
		if len(majmin) != 2 {
			continue
		}
		maj, _ := strconv.ParseUint(majmin[0], 10, 32)
		min, _ := strconv.ParseUint(majmin[1], 10, 32)
		t := unescape(fields[sep+1])
		if t == "ignore" { // MNTTYPE_IGNORE entries are skipped by fstype.c
			continue
		}
		types[unix.Mkdev(uint32(maj), uint32(min))] = t
		names[t] = true
	}
}

// FSType is fstype.c filesystem_type: the type of the last mount entry on
// dev, or "unknown". ok is false when the mount list cannot be read, which
// GNU treats as fatal ("Cannot read mounted file system list").
func FSType(dev uint64) (fstype string, ok bool) {
	once.Do(load)
	if !readable {
		return "", false
	}
	if t, found := types[dev]; found {
		return t, true
	}
	return "unknown", true
}

// HasType is parser.c is_used_fs_type (for -fstype at -O2 and above):
// "afs" and an unreadable mount list both count as in use.
func HasType(name string) bool {
	once.Do(load)
	return name == "afs" || !readable || names[name]
}

// SELinuxEnabled is libselinux is_selinux_enabled() > 0 as find sees it:
// selinuxfs is mounted.
func SELinuxEnabled() bool {
	once.Do(load)
	return names["selinuxfs"]
}

// unescape undoes mountinfo's \ooo escapes.
func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			if n, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
```

- [ ] **Step 9: `parse_print.go` — the parsers**

```go
package find

// parsePrintf is parser.c parse_printf.
func parsePrintf(ps *parser, e *parserEntry) bool {
	format, ok := ps.collectArg()
	if !ok {
		return false
	}
	return ps.insertFprintf(ps.r.out.stdoutStream, e, format)
}

// parseFprintf is parser.c parse_fprintf: the file opens only after the
// format was collected; without one, the filename is the bad argument.
func parseFprintf(ps *parser, e *parserEntry) bool {
	name, ok := ps.collectArg()
	if !ok {
		return false
	}
	if ps.i >= len(ps.argv) {
		ps.i-- // don't consume the filename: "invalid argument `out' to `-fprintf'"
		return false
	}
	format, _ := ps.collectArg()
	return ps.insertFprintf(ps.openOutputFile(name), e, format)
}

// parseFprint is parser.c parse_fprint.
func parseFprint(ps *parser, e *parserEntry) bool {
	name, ok := ps.collectArg()
	if !ok {
		return false
	}
	p := ps.insertPrimary(e, name, true)
	p.args.out = ps.openOutputFile(name)
	p.sideEffects, p.noDefaultPrint = true, true
	p.needStat, p.needType = false, false
	p.estSuccessRate = 1.0
	return true
}

// parseFprint0 is parser.c parse_fprint0 → insert_fprint(entry, filename).
func parseFprint0(ps *parser, e *parserEntry) bool {
	name, ok := ps.collectArg()
	if !ok {
		return false
	}
	p := ps.insertPrimary(e, name, true)
	p.args.out = ps.openOutputFile(name)
	p.sideEffects, p.noDefaultPrint = true, true
	p.needStat, p.needType = false, false
	p.estSuccessRate = 1.0
	return true
}
```
Table entries:
```go
		{argAction, "fprint", parseFprint, predPrint, fnFprint},
		{argAction, "fprint0", parseFprint0, predPrint0, fnFprint0},
		{argAction, "fprintf", parseFprintf, predFprintf, fnFprintf},
		{argAction, "printf", parsePrintf, predFprintf, fnFprintf},
```
`predPrint` and `predPrint0` already write to `p.args.out`, so `-fprint` and `-fprint0` reuse them.

- [ ] **Step 10: Gates**

```bash
bundle exec rake qfind:build
bundle exec rspec spec/find/actions/{printf,printf_escapes,printf_names,printf_attributes,printf_times,fprint,fprint0,fprintf,print,print0}_spec.rb spec/find/unusual_filenames_spec.rb spec/find/tests/size_spec.rb spec/find/tests/xtype_spec.rb spec/find/options/symbolic_links_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
```
Expected: 0 failures except examples that use `-ls`/`-fls` (Task 9), `-exec` (Task 8), `-size` (Task 11) or `-perm` (Task 13). The `%Z` example passes when SELinux is off (it is skipped otherwise).

- [ ] **Step 11: Commit**

```bash
gofmt -l internal; go vet ./...
git add internal && git commit -m "qfind: -printf family, shared output files, filemode, idcache, mountinfo"
```

---

## Task 8: Command execution — `-exec`, `-execdir`, `-ok`, `-okdir` (`;` and `+`)

This task ports `parser.c` `insert_exec_ok` and `check_path_safety`, `exec.c`, `lib/buildcmd.c` (find's subset), `util.c`'s `complete_pending_execs`/`execdirs`, and gnulib `yesno`.

Children run **one at a time** under a global lock, just as GNU's `launch()` waits for each child. Before every fork, stdout is committed and flushed. The `-ok`/`-okdir` children reproduce the 4.11.0 bug: fd 0 is closed and `/dev/null` sits on fd 3. Under `GNU_FINDUTILS_FD_LEAK_CHECK`, the child's abort is emulated.

**Files:**
- Create: `internal/find/exec.go`, `internal/find/exec_parse.go`, `internal/find/buildcmd.go`, `internal/find/launch.go`, `internal/find/prompt.go`
- Modify: `internal/find/parsetable.go` (4 entries), `internal/find/predicate.go` (`predArgs.exec *execVal`), `internal/find/traverse.go` (`afterRoot`), `internal/find/pred_actions.go` (`cleanup`)

**Interfaces:**
- Consumes: `r.flushStdout(e)`, `r.commit(e)`, `r.fatalAt`, `r.writeStderr`, `e.dirPath()`, `baseName`, `libc.ArgMax()`, `r.loc.IsYes`, and `r.serial`, which Task 4's `needsSerial` sets for every exec-family action.
- Produces:
  - `type execVal struct{ multiple, local, closeStdin bool; replaceVec []string; ctl bcControl; mu sync.Mutex; batch bcState; dirBatches map[batchKey]*bcState; id FuncID }` with `type batchKey struct{ dir string; depth int }`
  - `(*Run).completePendingExecs(p *Predicate)`, `(*Run).completePendingExecdirs(p *Predicate)`
  - `r.execMu sync.Mutex` (one child at a time), `r.stdin *bufio.Reader` (the `-ok` answers), `r.inheritedFDs map[int]bool`

- [ ] **Step 1: Failing gates**

```bash
bundle exec rspec spec/find/actions/{exec,execdir,ok,okdir}_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
```

- [ ] **Step 2: `buildcmd.go` — arg-size accounting (lib/buildcmd.c)**

```go
package find

// bcControl is struct buildcmd_control for find (no xargs options).
type bcControl struct {
	argMax      int64 // bc_use_sensible_arg_max: min(128 KiB, posix max), at least 4096
	maxArgCount int   // posix_arg_size_max / sizeof(char*) - 2
	initialArgc int
}

// bcState is struct buildcmd_state: one pending command line.
type bcState struct {
	argv         []string
	chars        int64 // cmd_argv_chars (each argument counts strlen+1)
	initialChars int64
	todo         bool
	dir          string // -execdir: directory of the batch
}

// bcInit is bc_init_controlinfo(ctl, 2048) + bc_use_sensible_arg_max.
// It reports false when the environment leaves no room (the caller prints
// "The environment is too large for exec().").
func bcInit(env []string) (bcControl, bool) {
	var envSize int64
	for _, e := range env {
		envSize += int64(len(e)) + 1
	}
	posixMax := libcArgMax()
	if envSize > posixMax || 2048+envSize >= posixMax {
		return bcControl{}, false
	}
	posixMax -= envSize + 2048
	c := bcControl{maxArgCount: int(posixMax/8) - 2, argMax: 128 * 1024}
	if c.argMax > posixMax {
		c.argMax = posixMax
	}
	if c.argMax < 4096 {
		c.argMax = 4096
	}
	return c, true
}

func (s *bcState) reset(c *bcControl) {
	s.argv = s.argv[:c.initialArgc]
	s.chars, s.todo = s.initialChars, false
}

// push is bc_push_arg for the entry e (nil for the initial arguments,
// pushed at parse time). exec runs and clears the pending command.
func (c *bcControl) push(r *Run, e *Entry, s *bcState, arg, prefix string, initial bool, exec func()) {
	cost := int64(len(arg)+1) + int64(len(prefix))
	if s.chars+cost > c.argMax {
		if initial || len(s.argv) == c.initialArgc {
			r.fatalAt(e, nil, "cannot fit single argument within argument list size limit")
		}
		exec()
	}
	if len(s.argv) == c.maxArgCount {
		exec()
	}
	if !initial {
		s.todo = true
	}
	s.argv = append(s.argv, prefix+arg)
	s.chars += cost
	if !initial && len(s.argv) == c.maxArgCount {
		exec()
	}
	if initial {
		s.initialChars = s.chars
	}
}

// insert is bc_do_insert for one ';' template argument: every "{}" is
// replaced by prefix+target, within arg_max-1 bytes.
func (c *bcControl) insert(r *Run, e *Entry, tmpl, prefix, target string) string {
	left := c.argMax - 1
	var b []byte
	for {
		i := indexBraces(tmpl)
		n := len(tmpl)
		if i >= 0 {
			n = i
		}
		if left <= int64(n) {
			break
		}
		left -= int64(n)
		b = append(b, tmpl[:n]...)
		tmpl = tmpl[n:]
		if i >= 0 {
			if left <= int64(len(target)+len(prefix)) {
				break
			}
			left -= int64(len(target) + len(prefix))
			b = append(b, prefix...)
			b = append(b, target...)
			tmpl = tmpl[2:]
		}
		if tmpl == "" {
			return string(b)
		}
	}
	r.fatalAt(e, nil, "command too long")
	return ""
}

func indexBraces(s string) int { return strings.Index(s, "{}") }
```
(`libcArgMax` is `libc.ArgMax`. Add the `strings` import.)

Check: RashuBug totals 7 + 901·36 + 2818·35 = 131073 > 131072, so the 3719th push executes the first 3718 files.

- [ ] **Step 3: `exec_parse.go` — `insert_exec_ok` and `check_path_safety` (parser.c 2729/2779)**

```go
package find

import "strings"

// execVal is struct exec_val.
type execVal struct {
	multiple, local, closeStdin bool
	replaceVec                  []string // ';' form: command template
	ctl                         bcControl
	mu                          sync.Mutex
	batch                       bcState               // -exec +; also holds the initial arguments
	dirBatches                  map[batchKey]*bcState // -execdir +
	id                          FuncID
}

// batchKey groups -execdir … + arguments. GNU runs the pending command
// whenever fts changes level, so a batch never mixes directories or
// levels; "." and "/" share a working directory with their children but
// not a level.
type batchKey struct {
	dir   string
	depth int
}

// checkPathSafety is parser.c check_path_safety.
func (r *Run) checkPathSafety(action string) {
	path, ok := r.lookupEnv("PATH")
	if !ok {
		return
	}
	for _, f := range strings.Split(path, ":") {
		switch {
		case f == "" || f == ".":
			r.fatalf(nil, "The current directory is included in the PATH environment variable, which is insecure in combination with the %s action of find.  Please remove the current directory from your $PATH (that is, remove \".\", doubled colons, or leading or trailing colons)", action)
		case f[0] != '/':
			r.fatalf(nil, "The relative path %s is included in the PATH environment variable, which is insecure in combination with the %s action of find.  Please remove that entry from $PATH", r.quote(f), action)
		}
	}
}

// insertExecOk is parser.c insert_exec_ok.
func (ps *parser) insertExecOk(action string, e *parserEntry) bool {
	r := ps.r
	if ps.i >= len(ps.argv) {
		return false
	}
	p := ps.insertPrimaryWithPred(e, e.fn, e.id, "(some -exec* arguments)", true)
	p.sideEffects, p.noDefaultPrint = true, true
	p.needType, p.needStat = false, false
	ev := &execVal{id: e.id, local: e.id == fnExecdir || e.id == fnOkdir}
	p.args.exec = ev
	allowPlus := e.id == fnExec || e.id == fnExecdir
	if !allowPlus {
		r.opt.OkPromptStdin = true
		ev.closeStdin = true
	}
	if ev.local {
		r.opt.IgnoreReaddirRace = false
		r.checkPathSafety(action)
	}
	start, end := ps.i, ps.i
	prevBraces, braceCount, braceArg := false, 0, ""
	for ; end < len(ps.argv) && ps.argv[end] != ";"; end++ {
		a := ps.argv[end]
		if allowPlus && prevBraces && a == "+" {
			ev.multiple = true
			break
		}
		prevBraces = false
		if strings.Contains(a, "{}") {
			prevBraces = a == "{}"
			braceArg = a
			braceCount++
			if start == end && ev.local {
				r.fatalf(nil, "You may not use {} within the utility name for -execdir and -okdir, because this is a potential security problem.")
			}
		}
	}
	if end == start || end == len(ps.argv) {
		ps.i = end
		return false
	}
	if ev.multiple {
		suffix := ""
		if e.id == fnExecdir {
			suffix = "dir"
		}
		if braceCount > 1 {
			r.fatalf(nil, "Only one instance of {} is supported with -exec%s ... +", suffix)
		}
		_ = braceArg // the "must appear by itself" branch is unreachable (spec/README.md)
	}
	ctl, ok := bcInit(r.env)
	if !ok {
		r.fatalf(nil, "The environment is too large for exec().")
	}
	ev.ctl = ctl
	if ev.multiple {
		ev.ctl.initialArgc = end - start - 1
		for _, a := range ps.argv[start : end-1] {
			ev.ctl.push(r, nil, &ev.batch, a, "", true, nil)
		}
		ev.dirBatches = map[batchKey]*bcState{}
	} else {
		ev.replaceVec = append([]string(nil), ps.argv[start:end]...)
	}
	ps.i = end + 1
	return true
}

func parseExec(ps *parser, e *parserEntry) bool    { return ps.insertExecOk("-exec", e) }
func parseExecdir(ps *parser, e *parserEntry) bool { return ps.insertExecOk("-execdir", e) }
func parseOk(ps *parser, e *parserEntry) bool      { return ps.insertExecOk("-ok", e) }
func parseOkdir(ps *parser, e *parserEntry) bool   { return ps.insertExecOk("-okdir", e) }
```
The `+` form's initial pushes grow `argv` from 0, so `initialArgc` must equal the number of words pushed.

In `optimise.go` (Task 3), finish `getPredCost`'s exec branch as tree.c does:
```go
	case p.is(fnExec) || p.is(fnExecdir) || p.is(fnOk) || p.is(fnOkdir): // predicate_uses_exec
		inherent = NeedsImmediateExec
		if p.args.exec.multiple {
			inherent = NeedsEventualExec
		}
```

- [ ] **Step 4: `exec.go` — `impl_pred_exec`, pending batches, cleanup order**

```go
package find

// predExec is exec.c impl_pred_exec for all four actions (pred_ok and
// pred_okdir prompt first). Evaluation is serialised whenever an
// exec-family action is present (Task 4), so at most one predExec runs.
func predExec(r *Run, e *Entry, p *Predicate) bool {
	ev := p.args.exec
	if ev.id == fnOk || ev.id == fnOkdir {
		if !r.askOk(e, ev) {
			return false
		}
	}
	target, prefix, dir := e.path, "", ""
	if ev.local {
		target = baseName(e.accPath) // base_name(state.rel_pathname)
		if target == "" || target[0] != '/' {
			prefix = "./"
		}
	}
	if ev.multiple {
		ev.mu.Lock()
		defer ev.mu.Unlock()
		s := &ev.batch
		if ev.local {
			s = ev.dirBatch(r, e)
		}
		ev.ctl.push(r, e, s, target, prefix, false, func() { r.runBatch(e, ev, s) })
		if ev.local && len(ev.dirBatches) > maxPendingDirBatches {
			r.completePendingDirBatches(ev) // bound memory; grouping is incidental
		}
		return true // POSIX: "+" is always true
	}
	if ev.local {
		dir = r.recordExecDir(e)
	}
	argv := make([]string, len(ev.replaceVec))
	for i, t := range ev.replaceVec {
		argv[i] = ev.ctl.insert(r, e, t, prefix, target)
	}
	return r.launch(e, ev, argv, dir) == 0
}

// maxPendingDirBatches bounds the -execdir … + command lines kept open at
// once. GNU has one at a time because it flushes at every level change;
// keeping more lets one directory's names share a command line.
const maxPendingDirBatches = 1024

// dirBatch returns the -execdir + batch for e's directory and level. A new
// batch records its working directory (record_exec_dir runs only while
// state.todo is false) and starts from the initial arguments.
func (ev *execVal) dirBatch(r *Run, e *Entry) *bcState {
	key := batchKey{e.dirPath(), e.depth}
	s := ev.dirBatches[key]
	if s == nil {
		s = &bcState{argv: append([]string(nil), ev.batch.argv[:ev.ctl.initialArgc]...),
			chars: ev.batch.initialChars, initialChars: ev.batch.initialChars}
		ev.dirBatches[key] = s
	}
	if !s.todo {
		s.dir = r.recordExecDir(e)
	}
	return s
}

// completePendingDirBatches runs and forgets every pending -execdir batch
// of ev (complete_pending_execdirs for one predicate).
func (r *Run) completePendingDirBatches(ev *execVal) {
	for key, s := range ev.dirBatches {
		if s.todo {
			r.runBatch(nil, ev, s)
		}
		delete(ev.dirBatches, key)
	}
}

// runBatch is bc_do_exec + bc_clear_args with launch() as the callback.
// A non-zero child status (or a signal) makes find's exit status 1.
func (r *Run) runBatch(e *Entry, ev *execVal, s *bcState) {
	if status := r.launch(e, ev, append([]string(nil), s.argv...), s.dir); status != 0 {
		r.setFailure()
	}
	s.reset(&ev.ctl)
}

// completePendingExecs is util.c complete_pending_execs over the tree,
// in left-to-right order; completePendingExecdirs then flushes -execdir.
func (r *Run) completePendingExecs(p *Predicate) {
	if p == nil {
		return
	}
	r.completePendingExecs(p.left)
	if ev := p.args.exec; ev != nil && ev.multiple && !ev.local && ev.batch.todo {
		r.runBatch(nil, ev, &ev.batch)
	}
	r.completePendingExecs(p.right)
}

func (r *Run) completePendingExecdirs(p *Predicate) {
	if p == nil {
		return
	}
	r.completePendingExecdirs(p.left)
	if ev := p.args.exec; ev != nil && ev.multiple && ev.local {
		r.completePendingDirBatches(ev)
	}
	r.completePendingExecdirs(p.right)
}

// recordExecDir is exec.c record_exec_dir + initialize_wd_for_exec. GNU
// opens the working directory O_RDONLY relative to fts's cwd: "." inside
// the parent for an entry below a start point when fts changed directory
// (needs read and search permission on the parent), mdir_name(rel_pathname)
// otherwise (the start point, or every entry under -L's FTS_NOCHDIR).
// Failure is fatal; GNU's message quotes rel_pathname. The descriptor is
// not kept: the child changes directory by name (launch).
func (r *Run) recordExecDir(e *Entry) string {
	dir := e.dirPath()
	probe := dir
	if e.depth > 0 && r.opt.Symlinks != SymlinkAlwaysDeref {
		probe = dir + "/."
	}
	fd, err := unix.Open(probe, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		r.fatalAt(e, err, "Failed to save working directory in order to run a command on %s", r.quote(e.accPath))
	}
	unix.Close(fd)
	return dir
}
```
Wire-up:
- `afterRoot()` (Task 4) becomes `r.completePendingExecdirs(r.tree)`, so `-execdir +` never spans starting points.
- `cleanup()` becomes:
  1. `r.completePendingExecs(r.tree)`;
  2. `r.completePendingExecdirs(r.tree)`;
  3. `r.closeSharedFiles()`;
  4. `r.cleanupFlush()`.
- Add `exec *execVal` to `predArgs`.
- Table entries:
```go
		{argAction, "exec", parseExec, predExec, fnExec},
		{argAction, "execdir", parseExecdir, predExec, fnExecdir},
		{argAction, "ok", parseOk, predExec, fnOk},
		{argAction, "okdir", parseOkdir, predExec, fnOkdir},
```

- [ ] **Step 5: `launch.go` — fork/exec with execvp semantics, one child at a time**

```go
package find

import (
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// launch is exec.c launch: flush, spawn, wait. It returns the child's exit
// status (1 when the child could not run the command, 128+sig for a
// signal). Callers hold evalMu (serial mode), so one child runs at a time;
// execMu makes that independent of the evaluation mode.
func (r *Run) launch(e *Entry, ev *execVal, argv []string, dir string) int {
	r.execMu.Lock()
	defer r.execMu.Unlock()
	if r.opt.Debug&DebugExec != 0 {
		var b strings.Builder
		for _, a := range argv {
			b.WriteString(" " + r.quote(a))
		}
		r.debugf("DebugExec: launching process (argc=%d):%s\n", len(argv), b.String())
	}
	r.flushStdout(e) // fflush(stdout) before fork; stderr is unbuffered
	if ev.closeStdin && r.leakCheck() {
		return r.emulateLeakAbort(argv)
	}
	// prep_child_for_exec: the child fchdir()s to the recorded directory
	// and exits 1 if it cannot (search permission).
	if dir != "" {
		if err := unix.Access(dir, unix.X_OK); err != nil {
			r.errorf(err, "Failed to change directory")
			return 1
		}
	}
	files := []*os.File{os.Stdin, os.Stdout, os.Stderr}
	if ev.closeStdin { // 4.11.0 bug: fd 0 closed, /dev/null leaked on fd 3
		null, _ := os.Open(os.DevNull)
		defer null.Close()
		files = []*os.File{nil, os.Stdout, os.Stderr, null}
	}
	r.markFDsCloexec()
	attr := &os.ProcAttr{Dir: dir, Env: r.env, Files: files}
	path, err := r.execvpPath(argv[0], dir)
	var proc *os.Process
	if err == nil {
		proc, err = os.StartProcess(path, argv, attr)
		if errno, _ := libcErrno(err); errno == unix.ENOEXEC { // execvp's script fallback
			proc, err = os.StartProcess("/bin/sh", append([]string{"/bin/sh", path}, argv[1:]...), attr)
		}
	}
	if err != nil {
		errno, _ := libcErrno(err)
		if errno == unix.EAGAIN || errno == unix.ENOMEM {
			r.fatalAt(e, errno, "cannot fork")
		}
		r.errorf(errno, "%s", r.quote(argv[0])) // the child's message in GNU
		return 1
	}
	st, err := proc.Wait()
	if err != nil {
		r.errorf(err, "error waiting for the command we executed for %s", r.quote(argv[0]))
		r.setFailure()
		return 1
	}
	ws := st.Sys().(syscall.WaitStatus)
	if ws.Signaled() {
		r.errorf(nil, "%s terminated by signal %d", r.quote(argv[0]), int(ws.Signal()))
		return 128 + int(ws.Signal())
	}
	if r.opt.Debug&DebugExec != 0 {
		r.debugf("DebugExec: process (PID=%d) terminated with exit status: %d\n", proc.Pid, ws.ExitStatus())
	}
	return ws.ExitStatus()
}

// execvpPath resolves argv[0] like glibc execvp: names with '/' are used
// as given (relative to dir); otherwise PATH (default "/bin:/usr/bin" when
// unset; "" means ".") is searched, skipping EACCES candidates.
func (r *Run) execvpPath(name, dir string) (string, error) {
	if strings.Contains(name, "/") {
		return name, nil
	}
	path, ok := r.lookupEnv("PATH")
	if !ok {
		path = "/bin:/usr/bin"
	}
	sawEACCES := false
	for _, d := range strings.Split(path, ":") {
		if d == "" {
			d = "."
		}
		cand := d + "/" + name
		probe := cand
		if dir != "" && !strings.HasPrefix(cand, "/") {
			probe = dir + "/" + cand
		}
		var st unix.Stat_t
		if err := unix.Stat(probe, &st); err != nil {
			if err == unix.EACCES {
				sawEACCES = true
			}
			continue
		}
		if st.Mode&unix.S_IFMT != unix.S_IFREG || unix.Access(probe, unix.X_OK) != nil {
			sawEACCES = true
			continue
		}
		return cand, nil
	}
	if sawEACCES {
		return "", unix.EACCES
	}
	return "", unix.ENOENT
}

func (r *Run) leakCheck() bool { _, ok := r.lookupEnv("GNU_FINDUTILS_FD_LEAK_CHECK"); return ok }

// emulateLeakAbort reproduces the -ok/-okdir child under GNU's descriptor
// leak check: lib/fdleak.c complains about /dev/null on fd 3, the assertion
// fails, and the child dies of SIGABRT. Required to keep the documented-bug
// examples pending. glibc's assert message names __progname (the base name).
func (r *Run) emulateLeakAbort(argv []string) int {
	r.errorf(nil, "File descriptor 3 will leak; please report this as a bug, remembering to include a detailed description of the simplest way to reproduce this problem.")
	r.debugf("%s: fdleak.c:395: complain_about_leaky_fds: Assertion `no_leaks' failed.\n", filepath.Base(r.prog))
	r.errorf(nil, "%s terminated by signal %d", r.quote(argv[0]), 6)
	return 128 + 6
}

// recordInheritedFDs remembers the descriptors find started with that a
// child would inherit (fdleak.c remember_non_cloexec_fds). Main calls it
// first. Close-on-exec and already-closed descriptors are left out, such as
// the one openFDs read /proc/self/fd through, so a directory that fastwalk
// later opens under that number is still swept.
func (r *Run) recordInheritedFDs() {
	r.inheritedFDs = map[int]bool{}
	for _, fd := range openFDs() {
		if flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); err == nil && flags&unix.FD_CLOEXEC == 0 {
			r.inheritedFDs[fd] = true
		}
	}
}

// markFDsCloexec sets close-on-exec on every descriptor opened since
// startup: fastwalk opens directories with plain open(2), and GNU's fts
// never lets a child inherit one. Inherited descriptors stay inheritable,
// as in GNU. A directory a worker opens between this sweep and the fork
// can still leak into one child (an accepted divergence).
func (r *Run) markFDsCloexec() {
	for _, fd := range openFDs() {
		if fd < 3 || r.inheritedFDs[fd] {
			continue
		}
		if flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); err == nil && flags&unix.FD_CLOEXEC == 0 {
			unix.FcntlInt(uintptr(fd), unix.F_SETFD, flags|unix.FD_CLOEXEC)
		}
	}
}

func openFDs() []int {
	ents, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return nil
	}
	fds := make([]int, 0, len(ents))
	for _, d := range ents {
		if fd, err := strconv.Atoi(d.Name()); err == nil {
			fds = append(fds, fd)
		}
	}
	return fds
}
```
(`libcErrno` is `libc.Errno`. Imports: `os`, `path/filepath`, `strconv`, `strings`, `syscall`, `x/sys/unix`, `qfind/internal/libc`.) In `main.go`, call `r.recordInheritedFDs()` right after `newRun`. Add `execMu sync.Mutex`, `stdin *bufio.Reader` and `inheritedFDs map[int]bool` to `Run`.

**Port checks:**
- Exit status: GNU leaves it unchanged for `;` failures (the predicate is just false) and sets it for `+` failures (`runBatch`).
- `launch` never calls `setFailure` for a non-zero exit status.
- A signalled `;` child prints the message and evaluates to false.
- A fork failure is fatal, which drops pending batches, because `fatalf` does not run `cleanup`.

- [ ] **Step 6: `prompt.go` — the `-ok` question (pred.c `is_ok`, gnulib `yesno`)**

```go
package find

import (
	"bufio"
	"os"
	"strings"
)

// askOk is pred.c is_ok + gnulib yesno: flush stdout, write
// "< cmd ... path > ? " raw (unquoted, the template's first word as typed)
// to stderr in one write, then read one line from find's buffered stdin;
// EOF or an empty read means no. Both pred_ok and pred_okdir pass the full
// path (okdir_spec: "< echo ... tmp/fred > ? ").
func (r *Run) askOk(e *Entry, ev *execVal) bool {
	r.flushStdout(e)
	if err := r.writeStderr("< " + ev.replaceVec[0] + " ... " + e.path + " > ? "); err != nil {
		r.fatalAt(e, err, "Failed to write prompt for -ok") // no cleanup: pending batches dropped
	}
	if r.stdin == nil {
		r.stdin = bufio.NewReader(os.Stdin)
	}
	line, err := r.stdin.ReadString('\n')
	if line == "" && err != nil {
		return false
	}
	return r.loc.IsYes(strings.TrimSuffix(line, "\n"))
}
```
Evaluation is already serialised when `-ok`/`-okdir` is present (Task 4), so prompts and answers pair up one file at a time.

- [ ] **Step 7: Gates**

```bash
bundle exec rake qfind:build
bundle exec rspec spec/find/actions/{exec,execdir,ok,okdir,quit}_spec.rb spec/find/options/debug_spec.rb spec/find/global_options/ignore_readdir_race_spec.rb spec/find/actions/printf_escapes_spec.rb spec/find/tests/links_spec.rb spec/find/tests/perm_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
```
Expected:
- 0 failures, except the 15 `rlimit_nproc: 0` examples (infeasible; verify each failing description contains "no more processes").
- 2 pending in `ok_spec.rb` and 2 in `okdir_spec.rb`; they must still be *pending*, not failing.
- Any `-perm`/`-links` examples wait for Tasks 11 and 13.

- [ ] **Step 8: Commit**

```bash
gofmt -l internal; go vet ./...
git add internal && git commit -m "qfind: -exec/-execdir/-ok/-okdir with buildcmd accounting"
```

---

## Task 9: `-ls`/`-fls` (lib/listfile.c) and `-delete`

**Files:**
- Create: `internal/find/ls.go`, `internal/find/delete.go`
- Modify: `internal/find/parsetable.go` (3 entries)

**Interfaces:**
- Consumes: `filemode.String`, `idcache.UserName`/`GroupName`, `libc.Strftime`, `r.emit`, `readlink`, `openOutputFile`.
- Produces: `(*Run).listFile(e *Entry, out *Stream)`, `predLs`, `predDelete`.

- [ ] **Step 1: Failing gates**

```bash
bundle exec rspec spec/find/actions/{ls,fls,delete}_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
```

- [ ] **Step 2: `ls.go` — `list_file` with static, growing field widths**

```go
package find

import (
	"fmt"
	"math"
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
	st := e.st // copied: the closure runs at commit time
	blocks := (uint64(st.Blocks)*512 + uint64(r.opt.OutputBlockSize) - 1) / uint64(r.opt.OutputBlockSize)
	user, haveUser := idcache.UserName(st.Uid)
	group, haveGroup := idcache.GroupName(st.Gid)
	date, dateStage := r.lsDate(st.Mtim.Sec)
	name := lsQuote(e.path)
	link, haveLink := "", false
	if st.Mode&unix.S_IFMT == unix.S_IFLNK {
		if target, err := readlink(e.sysPath); err == nil {
			link, haveLink = lsQuote(target), true
		} else {
			r.errorf(err, "%s", e.path) // unquoted; the exit status is unchanged
		}
	}
	w := r.lsw
	r.emitRendered(e, out, func() []piece {
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
		if haveLink {
			add(" -> ", 1300)
			add(link, 1350)
		}
		add("\n", 1400)
		return ps
	})
}

// right is fprintf("%*s", *width, s), widening the column when s is wider.
func (w *lsWidths) right(width *int, s string) string {
	t := fmt.Sprintf("%*s", *width, s)
	grow(width, len(t))
	return t
}

// lsDate is list_file's timestamp column and its failed_at stage:
// strftime of the local time, "%b %e %H:%M" within six months (30-day
// months) before and one hour after find's start, "%b %e  %Y" otherwise;
// when localtime would fail (the year does not fit an int), the seconds.
func (r *Run) lsDate(mt int64) (string, int) {
	t := time.Unix(mt, 0).In(r.tz)
	if y := int64(t.Year()) - 1900; y > math.MaxInt32 || y < math.MinInt32 {
		if mt < 0 {
			num := strconv.FormatUint(uint64(-(mt+1))+1, 10)
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

func predLs(r *Run, e *Entry, p *Predicate) bool {
	r.listFile(e, p.args.out)
	return true
}

// insertFls is parser.c insert_fls.
func (ps *parser) insertFls(e *parserEntry, name string, hasName bool) bool {
	p := ps.insertPrimary(e, name, hasName)
	if hasName {
		p.args.out = ps.openOutputFile(name)
	} else {
		p.args.out = ps.r.out.stdoutStream
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
```
Notes:
- Add `lsw *lsWidths` to `Run` (`newLsWidths()` in `newRun`).
- The human block count is `human_readable(st_blocks, ceiling, 512, output_block_size)`.
- The time format uses `LC_TIME`'s month names. Only the C names are needed, since `ls_spec.rb` runs under `LC_ALL=C`.
- Under `-L`, `e.st` is the target's stat, so no ` -> ` is printed (spec/README.md).

- [ ] **Step 3: `delete.go` — `parse_delete` and `pred_delete`**

```go
package find

import "golang.org/x/sys/unix"

// parseDelete is parser.c parse_delete: -delete implies -depth.
func parseDelete(ps *parser, e *parserEntry) bool {
	p := ps.insertPrimaryNoArg(e)
	p.sideEffects, p.noDefaultPrint = true, true
	ps.r.opt.DoDirFirst = false
	p.needStat, p.needType = false, false
	p.estSuccessRate = 1.0
	return true
}

// predDelete is pred.c pred_delete.
func predDelete(r *Run, e *Entry, p *Predicate) bool {
	if e.accPath == "." {
		return true
	}
	flags := 0
	if e.haveStat && e.st.Mode&unix.S_IFMT == unix.S_IFDIR {
		flags = unix.AT_REMOVEDIR
	}
	err := unix.Unlinkat(unix.AT_FDCWD, e.sysPath, flags)
	if err == nil {
		return true
	}
	if err == unix.ENOENT && r.opt.IgnoreReaddirRace {
		return true
	}
	if err == unix.EISDIR && flags == 0 {
		if err = unix.Unlinkat(unix.AT_FDCWD, e.sysPath, unix.AT_REMOVEDIR); err == nil {
			return true
		}
	}
	r.errorf(err, "cannot delete %s", r.quote(e.path))
	r.setFailure()
	return false
}
```
Table:
```go
		{argAction, "delete", parseDelete, predDelete, fnDelete},
		{argAction, "fls", parseFls, predLs, fnFls},
		{argAction, "ls", parseLs, predLs, fnLs},
```

- [ ] **Step 4: Gates**

```bash
bundle exec rake qfind:build
bundle exec rspec spec/find/actions/{ls,fls,delete}_spec.rb spec/find/global_options/{depth,ignore_readdir_race}_spec.rb spec/find/actions/print_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
```
Expected: 0 failures. In particular:
- the post-order `-delete` chain;
- the `.` start point is never deleted;
- `cannot delete 'dir/sub': Directory not empty`;
- the 1000-run `-ls` readdir race.

- [ ] **Step 5: Commit**

```bash
git add internal && git commit -m "qfind: -ls/-fls and -delete"
```

---

## Task 10: GNU regex in Go — `-regex`, `-iregex`, `-regextype`

GNU find compiles with `re_compile_pattern` under `re_set_syntax(options.regex_options)` and matches with `re_match(...) == len`, i.e. the whole path must match. The engine is **glibc's** (`GL_COND_OBJ_REGEX_TRUE="#"`: gnulib's copy is not built), so `gl/lib/regcomp.c` and `regexec.c` (identical code) are the reference.

The plan has three parts:
- an exact port of the *parser* (`peek_token`, `parse_reg_exp`, `parse_branch`, `parse_expression`, `parse_sub_exp`, `parse_dup_op`, `fetch_number`, `parse_bracket_exp`), driven by the RE_* syntax bits;
- the glibc error strings;
- an NFA compiled from the same tree glibc builds, executed by a Pike VM (linear) when the pattern has no back-references and by a backtracker when it does.

`re_compile_pattern` sets `newline_anchor = 1`.

**Files:**
- Create: `internal/regex/syntax.go`, `internal/regex/errors.go`, `internal/regex/token.go`, `internal/regex/parse.go`, `internal/regex/bracket.go`, `internal/regex/nfa.go`, `internal/regex/exec.go`
- Create: `internal/find/pred_regex.go`
- Modify: `internal/find/parsetable.go` (`regex`, `iregex`, `regextype`), `internal/find/predicate.go` (`predArgs.regex`)

**Interfaces:**
- Produces:
  - `regex.Syntax` with constants `BackslashEscapeInLists … NoSub` (`regex.h` bits 0–25); `regex.SyntaxEmacs`, `SyntaxAwk`, `SyntaxGNUAwk`, `SyntaxPosixAwk`, `SyntaxGrep`, `SyntaxEgrep`, `SyntaxPosixBasic`, `SyntaxPosixMinimalBasic`, `SyntaxPosixExtended`
  - `regex.Compile(pattern string, syn regex.Syntax, utf8 bool) (*regex.Regexp, error)`, where the error's `Error()` is the glibc message
  - `(*regex.Regexp).FullMatch(s string) bool`

- [ ] **Step 1: Failing gates**

```bash
bundle exec rspec spec/find/tests/regex_spec.rb spec/find/tests/iregex_spec.rb spec/find/positional_options/regextype_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
```

- [ ] **Step 2: `syntax.go` and `errors.go`**

```go
// Package regex implements glibc's re_compile_pattern/re_match for the
// GNU syntaxes selectable with find -regextype.
package regex

// Syntax is reg_syntax_t: the RE_* bits of gl/lib/regex.h.
type Syntax uint32

const (
	BackslashEscapeInLists Syntax = 1 << iota
	BkPlusQM
	CharClasses
	ContextIndepAnchors
	ContextIndepOps
	ContextInvalidOps
	DotNewline
	DotNotNull
	HatListsNotNewline
	Intervals
	LimitedOps
	NewlineAlt
	NoBkBraces
	NoBkParens
	NoBkRefs
	NoBkVbar
	NoEmptyRanges
	UnmatchedRightParenOrd
	NoPosixBacktracking
	NoGNUOps
	Debug
	InvalidIntervalOrd
	ICase
	CaretAnchorsHere
	ContextInvalidDup
	NoSub
)

const (
	SyntaxEmacs           = CharClasses | Intervals
	SyntaxAwk             = BackslashEscapeInLists | DotNotNull | NoBkParens | NoBkRefs | NoBkVbar | NoEmptyRanges | DotNewline | ContextIndepAnchors | CharClasses | UnmatchedRightParenOrd | NoGNUOps
	posixCommon           = CharClasses | DotNewline | DotNotNull | Intervals | NoEmptyRanges
	SyntaxPosixBasic      = posixCommon | BkPlusQM | ContextInvalidDup
	SyntaxPosixMinimalBasic = posixCommon | LimitedOps
	SyntaxPosixExtended   = posixCommon | ContextIndepAnchors | ContextIndepOps | NoBkBraces | NoBkParens | NoBkVbar | ContextInvalidOps | UnmatchedRightParenOrd
	SyntaxGNUAwk          = (SyntaxPosixExtended | BackslashEscapeInLists | InvalidIntervalOrd) &^ (DotNotNull | ContextIndepOps | ContextInvalidOps)
	SyntaxPosixAwk        = SyntaxPosixExtended | BackslashEscapeInLists | Intervals | NoGNUOps | InvalidIntervalOrd
	SyntaxGrep            = (SyntaxPosixBasic | NewlineAlt) &^ (ContextInvalidDup | DotNotNull)
	SyntaxEgrep           = (SyntaxPosixExtended | InvalidIntervalOrd | NewlineAlt) &^ (ContextInvalidOps | DotNotNull)
)

// RE_DUP_MAX is the largest interval count.
const dupMax = 0x7fff
```
`errors.go`:
```go
package regex

type errcode int

const (
	regNoError errcode = iota
	regNoMatch
	regBadPat
	regECollate
	regECtype
	regEEscape
	regESubreg
	regEBrack
	regEParen
	regEBrace
	regBadBR
	regERange
	regESpace
	regBadRpt
	regEEnd
	regESize
	regERParen
)

var messages = [...]string{"Success", "No match", "Invalid regular expression",
	"Invalid collation character", "Invalid character class name", "Trailing backslash",
	"Invalid back reference", "Unmatched [, [^, [:, [., or [=", "Unmatched ( or \\(",
	"Unmatched \\{", "Invalid content of \\{\\}", "Invalid range end", "Memory exhausted",
	"Invalid preceding regular expression", "Premature end of regular expression",
	"Regular expression too big", "Unmatched ) or \\)"}

// Error is a compile error whose text is glibc's re_compile_pattern message.
type Error struct{ code errcode }

func (e *Error) Error() string { return messages[e.code] }
```

- [ ] **Step 3: `token.go` — `peek_token` (regcomp.c), verbatim**

```go
package regex

type tokType uint8

const (
	tCharacter tokType = iota
	tEndOfRE
	tBackSlash
	tAlt
	tBackRef
	tAnchor
	tWord
	tNotWord
	tSpace
	tNotSpace
	tOpenSubexp
	tCloseSubexp
	tDupPlus
	tDupQuestion
	tDupAsterisk
	tOpenDupNum
	tCloseDupNum
	tOpenBracket
	tPeriod
)

type anchorKind uint8

const (
	aLineFirst anchorKind = iota
	aLineLast
	aBufFirst
	aBufLast
	aWordFirst
	aWordLast
	aWordDelim
	aNotWordDelim
	aInsideWord
	aInsideNotWord
)

type token struct {
	typ    tokType
	c      rune // opr.c
	idx    int  // back-reference index (0-based)
	anchor anchorKind
}

// input is re_string_t reduced to what the parser needs: the pattern as
// characters ("units", see decode in exec.go), case-folded in s under
// ICase, unfolded in orig. glibc reads the folded buffer except after a
// backslash and inside [:class:] (re_string_peek_byte_case), so under
// -iregex an escaped "\e" stays lower case and never matches.
type input struct {
	s, orig []rune
	pos     int
}

func (in *input) eoi() bool { return in.pos >= len(in.s) }

// peekToken is regcomp.c peek_token; it returns the token and its length.
func peekToken(in *input, syn Syntax) (token, int) {
	if in.eoi() {
		return token{typ: tEndOfRE}, 0
	}
	c := in.s[in.pos]
	t := token{typ: tCharacter, c: c}
	if c == '\\' {
		if in.pos+1 >= len(in.s) {
			return token{typ: tBackSlash, c: c}, 1
		}
		c2 := in.orig[in.pos+1]
		t.c = c2
		switch {
		case c2 == '|' && syn&LimitedOps == 0 && syn&NoBkVbar == 0:
			t.typ = tAlt
		case c2 >= '1' && c2 <= '9' && syn&NoBkRefs == 0:
			t.typ, t.idx = tBackRef, int(c2-'1')
		case syn&NoGNUOps == 0 && (c2 == '<' || c2 == '>' || c2 == 'b' || c2 == 'B' || c2 == '`' || c2 == '\''):
			t.typ = tAnchor
			t.anchor = map[rune]anchorKind{'<': aWordFirst, '>': aWordLast, 'b': aWordDelim,
				'B': aNotWordDelim, '`': aBufFirst, '\'': aBufLast}[c2]
		case syn&NoGNUOps == 0 && c2 == 'w':
			t.typ = tWord
		case syn&NoGNUOps == 0 && c2 == 'W':
			t.typ = tNotWord
		case syn&NoGNUOps == 0 && c2 == 's':
			t.typ = tSpace
		case syn&NoGNUOps == 0 && c2 == 'S':
			t.typ = tNotSpace
		case c2 == '(' && syn&NoBkParens == 0:
			t.typ = tOpenSubexp
		case c2 == ')' && syn&NoBkParens == 0:
			t.typ = tCloseSubexp
		case c2 == '+' && syn&LimitedOps == 0 && syn&BkPlusQM != 0:
			t.typ = tDupPlus
		case c2 == '?' && syn&LimitedOps == 0 && syn&BkPlusQM != 0:
			t.typ = tDupQuestion
		case c2 == '{' && syn&Intervals != 0 && syn&NoBkBraces == 0:
			t.typ = tOpenDupNum
		case c2 == '}' && syn&Intervals != 0 && syn&NoBkBraces == 0:
			t.typ = tCloseDupNum
		}
		return t, 2
	}
	switch c {
	case '\n':
		if syn&NewlineAlt != 0 {
			t.typ = tAlt
		}
	case '|':
		if syn&LimitedOps == 0 && syn&NoBkVbar != 0 {
			t.typ = tAlt
		}
	case '*':
		t.typ = tDupAsterisk
	case '+':
		if syn&LimitedOps == 0 && syn&BkPlusQM == 0 {
			t.typ = tDupPlus
		}
	case '?':
		if syn&LimitedOps == 0 && syn&BkPlusQM == 0 {
			t.typ = tDupQuestion
		}
	case '{':
		if syn&Intervals != 0 && syn&NoBkBraces != 0 {
			t.typ = tOpenDupNum
		}
	case '}':
		if syn&Intervals != 0 && syn&NoBkBraces != 0 {
			t.typ = tCloseDupNum
		}
	case '(':
		if syn&NoBkParens != 0 {
			t.typ = tOpenSubexp
		}
	case ')':
		if syn&NoBkParens != 0 {
			t.typ = tCloseSubexp
		}
	case '[':
		t.typ = tOpenBracket
	case '.':
		t.typ = tPeriod
	case '^':
		if syn&(ContextIndepAnchors|CaretAnchorsHere) == 0 && in.pos != 0 {
			if syn&NewlineAlt == 0 || in.s[in.pos-1] != '\n' {
				break
			}
		}
		t.typ, t.anchor = tAnchor, aLineFirst
	case '$':
		if syn&ContextIndepAnchors == 0 && in.pos+1 != len(in.s) {
			in.pos++
			next, _ := peekToken(in, syn)
			in.pos--
			if next.typ != tAlt && next.typ != tCloseSubexp {
				break
			}
		}
		t.typ, t.anchor = tAnchor, aLineLast
	}
	return t, 1
}

func fetchToken(in *input, syn Syntax) token {
	t, n := peekToken(in, syn)
	in.pos += n
	return t
}
```

- [ ] **Step 4: `parse.go` — regcomp.c's parser, function for function**

The tree mirrors glibc's `bin_tree_t`. The end-of-pattern node is implicit: the compiled program ends in `opMatch`.

```go
package regex

type nodeKind uint8

const (
	nChar    nodeKind = iota // CHARACTER: one unit
	nAny                     // OP_PERIOD
	nSet                     // SIMPLE_BRACKET (+ COMPLEX_BRACKET in UTF-8)
	nConcat                  // CONCAT(left, right)
	nAlt                     // OP_ALT(left, right); a nil side is the empty string
	nStar                    // OP_DUP_ASTERISK(left)
	nSubexp                  // SUBEXP(left), group idx; a nil left is an empty group
	nBackref                 // OP_BACK_REF, group idx
	nAnchor                  // ANCHOR
)

type node struct {
	kind        nodeKind
	c           rune
	set         *charSet
	left, right *node
	idx         int
	anchor      anchorKind
}

// clone is duplicate_tree.
func (n *node) clone() *node {
	if n == nil {
		return nil
	}
	c := *n
	c.left, c.right = n.left.clone(), n.right.clone()
	return &c
}

type parser struct {
	in        *input
	syn       Syntax
	utf8      bool
	tok       token
	nsub      int
	completed uint32 // completed_bkref_map: groups 1-9 closed so far
	backrefs  bool
	err       errcode
}

func (p *parser) fail(code errcode) *node {
	if p.err == regNoError {
		p.err = code
	}
	return nil
}

func (p *parser) fetch(syn Syntax) { p.tok = fetchToken(p.in, syn) }

// parse is regcomp.c parse.
func (p *parser) parse() *node {
	p.fetch(p.syn | CaretAnchorsHere)
	return p.parseRegExp(0)
}

// parseRegExp is parse_reg_exp: branch ('|' branch)*. A back-reference in
// one alternative may not refer to a group closed in another.
func (p *parser) parseRegExp(nest int) *node {
	initial := p.completed
	tree := p.parseBranch(nest)
	if p.err != regNoError {
		return nil
	}
	for p.tok.typ == tAlt {
		p.fetch(p.syn | CaretAnchorsHere)
		var branch *node
		if p.tok.typ != tAlt && p.tok.typ != tEndOfRE && (nest == 0 || p.tok.typ != tCloseSubexp) {
			accumulated := p.completed
			p.completed = initial
			if branch = p.parseBranch(nest); p.err != regNoError {
				return nil
			}
			p.completed |= accumulated
		}
		tree = &node{kind: nAlt, left: tree, right: branch}
	}
	return tree
}

// parseBranch is parse_branch: a concatenation of expressions.
func (p *parser) parseBranch(nest int) *node {
	tree := p.parseExpression(nest)
	if p.err != regNoError {
		return nil
	}
	for p.tok.typ != tAlt && p.tok.typ != tEndOfRE && (nest == 0 || p.tok.typ != tCloseSubexp) {
		expr := p.parseExpression(nest)
		if p.err != regNoError {
			return nil
		}
		if tree == nil {
			tree = expr
		} else if expr != nil {
			tree = &node{kind: nConcat, left: tree, right: expr}
		}
	}
	return tree
}

// parseExpression is parse_expression.
func (p *parser) parseExpression(nest int) *node {
	var tree *node
	switch p.tok.typ {
	case tCharacter:
		tree = &node{kind: nChar, c: p.tok.c}
	case tOpenSubexp:
		if tree = p.parseSubExp(nest + 1); p.err != regNoError {
			return nil
		}
	case tOpenBracket:
		if tree = p.parseBracketExp(); p.err != regNoError {
			return nil
		}
	case tBackRef:
		if p.completed&(1<<p.tok.idx) == 0 {
			return p.fail(regESubreg)
		}
		p.backrefs = true
		tree = &node{kind: nBackref, idx: p.tok.idx}
	case tOpenDupNum, tDupAsterisk, tDupPlus, tDupQuestion:
		if p.tok.typ == tOpenDupNum && p.syn&ContextInvalidDup != 0 {
			return p.fail(regBadRpt)
		}
		if p.syn&ContextInvalidOps != 0 {
			return p.fail(regBadRpt)
		}
		if p.syn&ContextIndepOps != 0 { // a leading operator is ignored
			p.fetch(p.syn)
			return p.parseExpression(nest)
		}
		tree = &node{kind: nChar, c: p.tok.c} // otherwise an ordinary character
	case tCloseSubexp:
		if p.syn&UnmatchedRightParenOrd == 0 {
			return p.fail(regERParen)
		}
		tree = &node{kind: nChar, c: p.tok.c}
	case tCloseDupNum:
		tree = &node{kind: nChar, c: p.tok.c}
	case tAnchor:
		switch p.tok.anchor {
		case aWordDelim: // \b is \< or \>
			tree = &node{kind: nAlt, left: &node{kind: nAnchor, anchor: aWordFirst},
				right: &node{kind: nAnchor, anchor: aWordLast}}
		case aNotWordDelim: // \B is inside a word or inside a non-word
			tree = &node{kind: nAlt, left: &node{kind: nAnchor, anchor: aInsideWord},
				right: &node{kind: nAnchor, anchor: aInsideNotWord}}
		default:
			tree = &node{kind: nAnchor, anchor: p.tok.anchor}
		}
		p.fetch(p.syn)
		return tree // an anchor is never repeated: "^*" is ^ and a literal *
	case tPeriod:
		tree = &node{kind: nAny}
	case tWord, tNotWord:
		tree = &node{kind: nSet, set: p.charclassOp("alnum", "_", p.tok.typ == tNotWord)}
	case tSpace, tNotSpace:
		tree = &node{kind: nSet, set: p.charclassOp("space", "", p.tok.typ == tNotSpace)}
	case tAlt, tEndOfRE:
		return nil
	case tBackSlash:
		return p.fail(regEEscape)
	}
	p.fetch(p.syn)
	for p.tok.typ == tDupAsterisk || p.tok.typ == tDupPlus || p.tok.typ == tDupQuestion || p.tok.typ == tOpenDupNum {
		dup := p.parseDupOp(tree)
		if p.err != regNoError {
			return nil
		}
		tree = dup
		if p.syn&ContextInvalidDup != 0 && (p.tok.typ == tDupAsterisk || p.tok.typ == tOpenDupNum) {
			return p.fail(regBadRpt) // BRE: a** is an error
		}
	}
	return tree
}

// parseSubExp is parse_sub_exp.
func (p *parser) parseSubExp(nest int) *node {
	cur := p.nsub
	p.nsub++
	p.fetch(p.syn | CaretAnchorsHere)
	var tree *node
	if p.tok.typ != tCloseSubexp {
		tree = p.parseRegExp(nest)
		if p.err == regNoError && p.tok.typ != tCloseSubexp {
			p.err = regEParen
		}
		if p.err != regNoError {
			return nil
		}
	}
	if cur <= 8 {
		p.completed |= 1 << cur
	}
	return &node{kind: nSubexp, left: tree, idx: cur}
}

// parseDupOp is parse_dup_op: *, +, ? and {n,m}, expanded as glibc does
// (<re>{n,m} is n copies, then (<re>(<re>…)?)?, or <re>* when unbounded).
func (p *parser) parseDupOp(elem *node) *node {
	startPos, startTok := p.in.pos, p.tok
	var start, end int
	if p.tok.typ == tOpenDupNum {
		if start = p.fetchNumber(); start == -1 {
			if p.tok.typ != tCharacter || p.tok.c != ',' {
				return p.fail(regBadBR) // <re>{} is invalid
			}
			start = 0 // {,m} is {0,m}
		}
		if start != -2 {
			switch {
			case p.tok.typ == tCloseDupNum:
				end = start
			case p.tok.typ == tCharacter && p.tok.c == ',':
				end = p.fetchNumber()
			default:
				end = -2
			}
		}
		if start == -2 || end == -2 {
			if p.syn&InvalidIntervalOrd == 0 {
				if p.tok.typ == tEndOfRE {
					return p.fail(regEBrace)
				}
				return p.fail(regBadBR)
			}
			p.in.pos, p.tok = startPos, startTok // roll back: '{' is a literal
			p.tok.typ = tCharacter
			return elem
		}
		if (end != -1 && start > end) || p.tok.typ != tCloseDupNum {
			return p.fail(regBadBR)
		}
		lim := end
		if lim == -1 {
			lim = start
		}
		if lim > dupMax {
			return p.fail(regESize)
		}
	} else {
		start, end = 0, -1
		if p.tok.typ == tDupPlus {
			start = 1
		}
		if p.tok.typ == tDupQuestion {
			end = 1
		}
	}
	p.fetch(p.syn)
	if elem == nil || (start == 0 && end == 0) {
		return nil
	}
	var old *node
	if start > 0 {
		tree := elem
		for i := 2; i <= start; i++ {
			tree = &node{kind: nConcat, left: tree, right: elem.clone()}
		}
		if start == end {
			return tree
		}
		elem, old = elem.clone(), tree
	}
	var tree *node
	if end == -1 {
		tree = &node{kind: nStar, left: elem}
	} else {
		tree = &node{kind: nAlt, left: elem}
		for i := start + 2; i <= end; i++ {
			tree = &node{kind: nAlt, left: &node{kind: nConcat, left: tree, right: elem.clone()}}
		}
	}
	if old != nil {
		tree = &node{kind: nConcat, left: old, right: tree}
	}
	return tree
}

// fetchNumber is fetch_number: -1 for an empty field, -2 for an error,
// RE_DUP_MAX+1 when too large.
func (p *parser) fetchNumber() int {
	num := -1
	for {
		p.fetch(p.syn)
		c := p.tok.c
		if p.tok.typ == tEndOfRE {
			return -2
		}
		if p.tok.typ == tCloseDupNum || c == ',' {
			return num
		}
		switch {
		case p.tok.typ != tCharacter || c < '0' || c > '9' || num == -2:
			num = -2
		case num == -1:
			num = int(c - '0')
		default:
			num = min(dupMax+1, num*10+int(c-'0'))
		}
	}
}
```
- [ ] **Step 5: `bracket.go` — `parse_bracket_exp` and the character sets**

Two rules govern bracket matching:
- **C locale.** A set is 256 bits over bytes.
- **UTF-8 locale.** glibc masks the bit set to single-byte characters (ASCII) and matches valid multibyte characters through a `COMPLEX_BRACKET` part: listed characters, code-point ranges (C.UTF-8 collates by code point), character classes through `iswctype`, and negation. An invalid byte is a single-byte character outside ASCII, so no bracket expression matches it, negated or not.

```go
package regex

import (
	"slices"
	"unicode"
	"unicode/utf8"
)

// charSet is a compiled bracket expression.
type charSet struct {
	bits     [256]bool // single-byte part, negation already applied
	utf8     bool
	runes    []rune      // COMPLEX_BRACKET: mbchars
	ranges   [][2]rune   // range_starts/range_ends
	classes  []string    // char_classes
	nonMatch bool        // negates the multibyte part
}

// has reports whether the unit u matches.
func (cs *charSet) has(u rune) bool {
	switch {
	case !cs.utf8:
		return u >= 0 && u < 256 && cs.bits[u]
	case u < 0x80:
		return cs.bits[u]
	case u >= invalidBase: // an invalid byte: neither part accepts it
		return false
	}
	m := slices.Contains(cs.runes, u)
	for _, r := range cs.ranges {
		m = m || (r[0] <= u && u <= r[1])
	}
	for _, c := range cs.classes {
		m = m || isClass(c, u, true)
	}
	return m != cs.nonMatch
}

// isClass is isctype in the C locale (ASCII only) and iswctype in UTF-8.
func isClass(name string, u rune, wide bool) bool {
	if !wide && u >= 0x80 {
		return false
	}
	switch name {
	case "alnum":
		return unicode.IsLetter(u) || unicode.IsDigit(u)
	case "alpha":
		return unicode.IsLetter(u)
	case "blank":
		return u == ' ' || u == '\t' || (wide && u >= 0x80 && unicode.Is(unicode.Zs, u) && u != 0xa0 && u != 0x2007 && u != 0x202f)
	case "cntrl":
		return unicode.IsControl(u)
	case "digit":
		return u >= '0' && u <= '9'
	case "graph":
		return unicode.IsGraphic(u) && !unicode.IsSpace(u) && u != ' '
	case "lower":
		return unicode.IsLower(u)
	case "print":
		return unicode.IsPrint(u) || u == ' '
	case "punct":
		return u < 0x80 && (unicode.IsPunct(u) || unicode.IsSymbol(u)) || u >= 0x80 && wide && (unicode.IsPunct(u) || unicode.IsSymbol(u))
	case "space":
		return unicode.IsSpace(u)
	case "upper":
		return unicode.IsUpper(u)
	case "xdigit":
		return u < 0x80 && (u >= '0' && u <= '9' || u|0x20 >= 'a' && u|0x20 <= 'f')
	}
	return false
}

var classNames = []string{"alnum", "alpha", "blank", "cntrl", "digit", "graph", "lower", "print", "punct", "space", "upper", "xdigit"}

// buildCharclass is build_charclass: under ICase, upper and lower are alpha.
func (p *parser) buildCharclass(cs *charSet, name string) bool {
	if p.syn&ICase != 0 && (name == "upper" || name == "lower") {
		name = "alpha"
	}
	if !slices.Contains(classNames, name) {
		return false
	}
	for b := 0; b < 256; b++ { // trans is NULL for find: no translation here
		if isClass(name, rune(b), false) {
			cs.bits[b] = true
		}
	}
	if p.utf8 {
		cs.classes = append(cs.classes, name)
	}
	return true
}

// charclassOp is build_charclass_op (\w \W \s \S).
func (p *parser) charclassOp(name, extra string, nonMatch bool) *charSet {
	cs := &charSet{utf8: p.utf8, nonMatch: nonMatch}
	saved := p.syn
	p.syn = 0 // "we don't care the syntax in this case"
	p.buildCharclass(cs, name)
	p.syn = saved
	for _, c := range extra {
		cs.bits[c] = true
	}
	if nonMatch {
		for i := range cs.bits {
			cs.bits[i] = !cs.bits[i]
		}
	}
	return cs
}

// bracket token kinds (peek_token_bracket).
type btok uint8

const (
	bChar btok = iota
	bClose
	bNonMatch
	bRange
	bOpenColl
	bOpenEquiv
	bOpenClass
	bEnd
)

// peekBracket is peek_token_bracket; it returns the kind, the unit and the
// token length in units.
func (p *parser) peekBracket() (btok, rune, int) {
	in := p.in
	if in.eoi() {
		return bEnd, 0, 0
	}
	c := in.s[in.pos]
	if c == '\\' && p.syn&BackslashEscapeInLists != 0 && in.pos+1 < len(in.s) {
		in.pos++ // the escaped character itself is the token
		return bChar, in.s[in.pos], 1
	}
	if c == '[' {
		var c2 rune
		if in.pos+1 < len(in.s) {
			c2 = in.s[in.pos+1]
		}
		switch {
		case c2 == '.':
			return bOpenColl, c2, 2
		case c2 == '=':
			return bOpenEquiv, c2, 2
		case c2 == ':' && p.syn&CharClasses != 0:
			return bOpenClass, c2, 2
		}
		return bChar, c, 1
	}
	switch c {
	case ']':
		return bClose, c, 1
	case '^':
		return bNonMatch, c, 1
	case '-':
		if !(in.pos+2 < len(in.s) && in.s[in.pos+1] == '-' && in.s[in.pos+2] == '-') {
			return bRange, c, 1
		}
		in.pos += 2 // "---" is one '-'
	}
	return bChar, c, 1
}

type elemKind uint8

const (
	eChar elemKind = iota // SB_CHAR or MB_CHAR
	eColl
	eEquiv
	eClass
)

type bracketElem struct {
	kind elemKind
	c    rune
	name []rune
}

// parseBracketExp is parse_bracket_exp.
func (p *parser) parseBracketExp() *node {
	cs := &charSet{utf8: p.utf8}
	kind, c, n := p.peekBracket()
	if kind == bEnd {
		return p.fail(regBadPat)
	}
	nonMatch := false
	if kind == bNonMatch {
		nonMatch, cs.nonMatch = true, true
		if p.syn&HatListsNotNewline != 0 {
			cs.bits['\n'] = true
		}
		p.in.pos += n
		if kind, c, n = p.peekBracket(); kind == bEnd {
			return p.fail(regBadPat)
		}
	}
	if kind == bClose {
		kind = bChar // a first ']' is ordinary
	}
	first := true
	for {
		start, code := p.parseBracketElement(kind, c, n, first)
		if code != regNoError {
			return p.fail(code)
		}
		first = false
		kind, c, n = p.peekBracket()
		isRange := false
		var kind2 btok
		var c2 rune
		var n2 int
		if start.kind != eClass && start.kind != eEquiv {
			if kind == bEnd {
				return p.fail(regEBrack)
			}
			if kind == bRange {
				p.in.pos += n // skip '-'
				if kind2, c2, n2 = p.peekBracket(); kind2 == bEnd {
					return p.fail(regEBrack)
				}
				if kind2 == bClose { // a last '-' is ordinary
					p.in.pos -= n
					kind = bChar
				} else {
					isRange = true
				}
			}
		}
		if isRange {
			end, code := p.parseBracketElement(kind2, c2, n2, true)
			if code != regNoError {
				return p.fail(code)
			}
			kind, c, n = p.peekBracket()
			if code := p.buildRange(cs, start, end); code != regNoError {
				return p.fail(code)
			}
		} else if code := p.addElem(cs, start); code != regNoError {
			return p.fail(code)
		}
		if kind == bEnd {
			return p.fail(regEBrack)
		}
		if kind == bClose {
			break
		}
	}
	p.in.pos += n // skip ']'
	if nonMatch {
		for i := range cs.bits {
			cs.bits[i] = !cs.bits[i]
		}
	}
	return &node{kind: nSet, set: cs}
}

// parseBracketElement is parse_bracket_element (+ parse_bracket_symbol).
func (p *parser) parseBracketElement(kind btok, c rune, n int, acceptHyphen bool) (bracketElem, errcode) {
	in := p.in
	if p.utf8 && c >= 0x80 && c < invalidBase && kind == bChar { // MB_CHAR
		in.pos += n
		return bracketElem{kind: eChar, c: c}, regNoError
	}
	in.pos += n
	switch kind {
	case bOpenColl, bOpenEquiv, bOpenClass:
		return p.parseBracketSymbol(kind)
	case bRange:
		if !acceptHyphen {
			if k, _, _ := p.peekBracket(); k != bClose {
				return bracketElem{}, regERange
			}
		}
	}
	return bracketElem{kind: eChar, c: c}, regNoError
}

// parseBracketSymbol reads NAME of [.NAME.], [=NAME=] or [:NAME:] (class
// names from the unfolded pattern); names of 32 bytes or more are EBRACK.
func (p *parser) parseBracketSymbol(kind btok) (bracketElem, errcode) {
	in := p.in
	delim := map[btok]rune{bOpenColl: '.', bOpenEquiv: '=', bOpenClass: ':'}[kind]
	if in.eoi() {
		return bracketElem{}, regEBrack
	}
	var name []rune
	nbytes := 0
	for {
		if nbytes >= 32 {
			return bracketElem{}, regEBrack
		}
		ch := in.s[in.pos]
		if kind == bOpenClass {
			ch = in.orig[in.pos]
		}
		in.pos++
		if in.eoi() {
			return bracketElem{}, regEBrack
		}
		if ch == delim && in.s[in.pos] == ']' {
			break
		}
		name = append(name, ch)
		nbytes += unitLen(ch)
	}
	in.pos++
	k := map[btok]elemKind{bOpenColl: eColl, bOpenEquiv: eEquiv, bOpenClass: eClass}[kind]
	return bracketElem{kind: k, name: name}, regNoError
}

// addElem adds a single element (parse_bracket_exp's switch).
func (p *parser) addElem(cs *charSet, e bracketElem) errcode {
	switch e.kind {
	case eChar:
		p.addChar(cs, e.c)
	case eEquiv, eColl: // nrules == 0: a single byte only
		if len(e.name) != 1 || unitLen(e.name[0]) != 1 {
			return regECollate
		}
		p.addChar(cs, e.name[0])
	case eClass:
		if !p.buildCharclass(cs, string(e.name)) {
			return regECtype
		}
	}
	return regNoError
}

func (p *parser) addChar(cs *charSet, c rune) {
	if !p.utf8 || c < 0x80 {
		if c >= 0 && c < 256 {
			cs.bits[c] = true
		}
		return
	}
	if c < invalidBase {
		cs.runes = append(cs.runes, c)
	}
}

// buildRange is build_range_exp with nrules == 0 (code-point order).
func (p *parser) buildRange(cs *charSet, start, end bracketElem) errcode {
	if start.kind == eClass || start.kind == eEquiv || end.kind == eClass || end.kind == eEquiv {
		return regERange
	}
	lo, ok1 := collValue(start)
	hi, ok2 := collValue(end)
	if !ok1 || !ok2 {
		return regECollate
	}
	if p.syn&NoEmptyRanges != 0 && lo > hi {
		return regERange
	}
	if p.utf8 {
		cs.ranges = append(cs.ranges, [2]rune{lo, hi})
	}
	for b := rune(0); b < 256; b++ { // the single-byte table
		if lo <= b && b <= hi && (!p.utf8 || b < 0x80) {
			cs.bits[b] = true
		}
	}
	return regNoError
}

// collValue is lookup_collation_sequence_value for nrules == 0.
func collValue(e bracketElem) (rune, bool) {
	switch e.kind {
	case eChar:
		return e.c, true
	case eColl:
		if len(e.name) == 1 && unitLen(e.name[0]) == 1 {
			return e.name[0], true
		}
	}
	return 0, false
}

// unitLen is a unit's length in bytes in the pattern.
func unitLen(u rune) int {
	if u < 0x80 || u >= invalidBase {
		return 1
	}
	return utf8.RuneLen(u)
}
```
In the C locale `p.utf8` is false and every unit is a byte value 0–255. `addChar`, `buildRange` and `has` then reduce to the 256-bit set, exactly glibc's `SIMPLE_BRACKET`.

The `blank` and `punct` lines above approximate glibc's `iswctype` tables for non-ASCII characters. ASCII is exact. The suite runs brackets only on ASCII and in C.UTF-8.

- [ ] **Step 6: `exec.go` — units, compilation and whole-string matching**

```go
package regex

import (
	"unicode"
	"unicode/utf8"
)

// invalidBase marks a byte that is not part of a valid UTF-8 sequence:
// unit invalidBase+b. Literals match it only by the same byte; '.' matches
// it; brackets never do (see bracket.go).
const invalidBase = 0x110000

// decode turns a pattern or subject into units: bytes in a single-byte
// locale; runes in UTF-8, with an invalid byte b as invalidBase+b.
func decode(s string, wide bool) []rune {
	us := make([]rune, 0, len(s))
	for i := 0; i < len(s); {
		if !wide {
			us = append(us, rune(s[i]))
			i++
			continue
		}
		r, n := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && n <= 1 {
			us = append(us, invalidBase+rune(s[i]))
			i++
			continue
		}
		us = append(us, r)
		i += n
	}
	return us
}

// fold is RE_ICASE's translation: toupper in the C locale (ASCII only),
// towupper for valid characters in UTF-8; invalid bytes are unchanged.
func foldUnit(u rune, wide bool) rune {
	switch {
	case u < 0x80:
		if u >= 'a' && u <= 'z' {
			return u - 'a' + 'A'
		}
		return u
	case !wide || u >= invalidBase:
		return u
	}
	return unicode.ToUpper(u)
}

// isWord is IS_WORD_CHAR / IS_WIDE_WORD_CHAR. An invalid byte b counts as
// the wide character b, as glibc's build_wcs_buffer makes it.
func isWord(u rune, wide bool) bool {
	if u >= invalidBase {
		u -= invalidBase
	}
	if u == '_' {
		return true
	}
	if !wide && u >= 0x80 {
		return false
	}
	return unicode.IsLetter(u) || unicode.IsDigit(u)
}

type opcode uint8

const (
	opChar opcode = iota
	opAny
	opSet
	opSplit // try x, then y
	opJmp
	opSave
	opBackref
	opAssert
	opMatch
)

type inst struct {
	op     opcode
	c      rune
	set    *charSet
	anchor anchorKind
	x, y   int
	n      int
}

// Regexp is a compiled pattern.
type Regexp struct {
	prog     []inst
	backrefs bool
	nsub     int
	syn      Syntax
	utf8     bool
}

// Compile is re_compile_pattern (newline_anchor = 1) for a pattern string.
func Compile(pattern string, syn Syntax, wide bool) (*Regexp, error) {
	orig := decode(pattern, wide)
	s := orig
	if syn&ICase != 0 {
		s = make([]rune, len(orig))
		for i, u := range orig {
			s[i] = foldUnit(u, wide)
		}
	}
	p := &parser{in: &input{s: s, orig: orig}, syn: syn, utf8: wide}
	tree := p.parse()
	if p.err != regNoError {
		return nil, &Error{p.err}
	}
	re := &Regexp{backrefs: p.backrefs, nsub: p.nsub, syn: syn, utf8: wide}
	re.emit(tree)
	re.prog = append(re.prog, inst{op: opMatch})
	return re, nil
}

// emit is a Thompson construction of the tree.
func (re *Regexp) emit(n *node) {
	if n == nil {
		return
	}
	add := func(in inst) int { re.prog = append(re.prog, in); return len(re.prog) - 1 }
	switch n.kind {
	case nChar:
		add(inst{op: opChar, c: n.c})
	case nAny:
		add(inst{op: opAny})
	case nSet:
		add(inst{op: opSet, set: n.set})
	case nConcat:
		re.emit(n.left)
		re.emit(n.right)
	case nAlt:
		split := add(inst{op: opSplit})
		re.prog[split].x = len(re.prog)
		re.emit(n.left)
		jmp := add(inst{op: opJmp})
		re.prog[split].y = len(re.prog)
		re.emit(n.right)
		re.prog[jmp].x = len(re.prog)
	case nStar:
		split := add(inst{op: opSplit})
		re.prog[split].x = len(re.prog)
		re.emit(n.left)
		add(inst{op: opJmp, x: split})
		re.prog[split].y = len(re.prog)
	case nSubexp:
		add(inst{op: opSave, n: 2 * n.idx})
		re.emit(n.left)
		add(inst{op: opSave, n: 2*n.idx + 1})
	case nBackref:
		add(inst{op: opBackref, n: n.idx})
	case nAnchor:
		add(inst{op: opAssert, anchor: n.anchor})
	}
}

// FullMatch is re_match(re, s, len, 0, NULL) == len: some match starting
// at 0 spans all of s.
func (re *Regexp) FullMatch(s string) bool {
	in := decode(s, re.utf8)
	if re.syn&ICase != 0 {
		for i, u := range in {
			in[i] = foldUnit(u, re.utf8)
		}
	}
	if re.backrefs {
		caps := make([]int, 2*max(re.nsub, 9))
		for i := range caps {
			caps[i] = -1
		}
		return re.backtrack(in, 0, 0, caps, map[[2]int]bool{})
	}
	return re.pike(in)
}

// assert evaluates an anchor between in[i-1] and in[i]. newline_anchor is
// set by re_compile_pattern, so ^ and $ also match at embedded newlines.
func (re *Regexp) assert(k anchorKind, in []rune, i int) bool {
	prevWord := i > 0 && isWord(in[i-1], re.utf8)
	nextWord := i < len(in) && isWord(in[i], re.utf8)
	switch k {
	case aLineFirst:
		return i == 0 || in[i-1] == '\n'
	case aLineLast:
		return i == len(in) || in[i] == '\n'
	case aBufFirst:
		return i == 0
	case aBufLast:
		return i == len(in)
	case aWordFirst:
		return !prevWord && nextWord
	case aWordLast:
		return prevWord && !nextWord
	case aInsideWord:
		return prevWord && nextWord
	case aInsideNotWord:
		return !prevWord && !nextWord
	}
	return false
}

// step reports whether instruction pc consumes unit u.
func (re *Regexp) step(pc int, u rune) bool {
	switch in := &re.prog[pc]; in.op {
	case opChar:
		return u == in.c
	case opAny:
		return !(u == '\n' && re.syn&DotNewline == 0) && !(u == 0 && re.syn&DotNotNull != 0)
	case opSet:
		return in.set.has(u)
	}
	return false
}

// pike decides a full match without back-references in linear time: the
// set of live instructions advances one unit at a time.
func (re *Regexp) pike(in []rune) bool {
	n := len(re.prog)
	cur, next := make([]int, 0, n), make([]int, 0, n)
	onList := make([]int, n) // generation stamps
	gen := 1
	var add func(list []int, pc, i int) []int
	add = func(list []int, pc, i int) []int {
		if onList[pc] == gen {
			return list
		}
		onList[pc] = gen
		switch inst := &re.prog[pc]; inst.op {
		case opJmp:
			return add(list, inst.x, i)
		case opSplit:
			return add(add(list, inst.x, i), inst.y, i)
		case opSave:
			return add(list, pc+1, i)
		case opAssert:
			if re.assert(inst.anchor, in, i) {
				return add(list, pc+1, i)
			}
			return list
		}
		return append(list, pc)
	}
	cur = add(cur, 0, 0)
	for i := 0; ; i++ {
		if len(cur) == 0 {
			return false
		}
		if i == len(in) {
			for _, pc := range cur {
				if re.prog[pc].op == opMatch {
					return true
				}
			}
			return false
		}
		gen++
		next = next[:0]
		for _, pc := range cur {
			if re.step(pc, in[i]) {
				next = add(next, pc+1, i+1)
			}
		}
		cur, next = next, cur
	}
}

// backtrack decides a full match with back-references by depth-first
// search over the program; caps holds group boundaries. seen cuts empty
// loops: a (pc, i) pair revisited without consuming input is a dead end.
func (re *Regexp) backtrack(in []rune, pc, i int, caps []int, seen map[[2]int]bool) bool {
	for {
		inst := &re.prog[pc]
		switch inst.op {
		case opMatch:
			return i == len(in)
		case opChar, opAny, opSet:
			if i >= len(in) || !re.step(pc, in[i]) {
				return false
			}
			pc, i = pc+1, i+1
			clear(seen)
		case opJmp:
			pc = inst.x
		case opSplit:
			key := [2]int{pc, i}
			if seen[key] {
				return false
			}
			seen[key] = true
			saved := append([]int(nil), caps...)
			if re.backtrack(in, inst.x, i, caps, cloneSeen(seen)) {
				return true
			}
			copy(caps, saved)
			pc = inst.y
		case opSave:
			old := caps[inst.n]
			caps[inst.n] = i
			if re.backtrack(in, pc+1, i, caps, seen) {
				return true
			}
			caps[inst.n] = old
			return false
		case opBackref:
			s, e := caps[2*inst.n], caps[2*inst.n+1]
			if s < 0 || e < 0 || i+(e-s) > len(in) {
				return false
			}
			for k := 0; k < e-s; k++ {
				if in[s+k] != in[i+k] {
					return false
				}
			}
			if e > s {
				clear(seen)
			}
			pc, i = pc+1, i+(e-s)
		case opAssert:
			if !re.assert(inst.anchor, in, i) {
				return false
			}
			pc++
		}
	}
}

func cloneSeen(m map[[2]int]bool) map[[2]int]bool {
	c := make(map[[2]int]bool, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}
```
**Port checks against the oracle.** Before wiring the predicates, verify with `.scratch/gnufind` that these give identical results:
- the patterns of `patterns.rb` (`a whole-path regular expression test`) and the `regextype_spec.rb` matrix;
- `-iregex '/\etc/hosts'` against `/etc/hosts` (no match);
- `-regex '\(a*\)*\1'` (empty iterations).

`Regexp` holds no mutable state: `FullMatch` allocates per call, so one compiled pattern serves every goroutine.

- [ ] **Step 7: find side — `pred_regex.go` (parser.c insert_regex, parse_regextype; pred.c pred_regex; lib/regextype.c)**

```go
package find

import (
	"strings"

	"qfind/internal/regex"
)

var regexTypes = []struct {
	name string
	syn  regex.Syntax
}{
	{"findutils-default", regex.SyntaxEmacs | regex.DotNewline},
	{"awk", regex.SyntaxAwk},
	{"ed", regex.SyntaxPosixBasic},
	{"egrep", regex.SyntaxEgrep},
	{"emacs", regex.SyntaxEmacs},
	{"gnu-awk", regex.SyntaxGNUAwk},
	{"grep", regex.SyntaxGrep},
	{"posix-awk", regex.SyntaxPosixAwk},
	{"posix-basic", regex.SyntaxPosixBasic},
	{"posix-egrep", regex.SyntaxEgrep},
	{"posix-extended", regex.SyntaxPosixExtended},
	{"posix-minimal-basic", regex.SyntaxPosixMinimalBasic},
	{"sed", regex.SyntaxPosixBasic},
}

// parseRegextype is parser.c parse_regextype + lib/regextype.c get_regex_type.
func parseRegextype(ps *parser, e *parserEntry) bool {
	name, ok := ps.collectArg()
	if !ok {
		return false
	}
	for _, t := range regexTypes {
		if t.name == name {
			ps.r.opt.RegexSyntax = t.syn
			return parseNoop(ps, e)
		}
	}
	quoted := make([]string, len(regexTypes))
	for i, t := range regexTypes {
		quoted[i] = ps.r.quote(t.name)
	}
	ps.r.fatalf(nil, "Unknown regular expression type %s; valid types are %s.", ps.r.quote(name), strings.Join(quoted, ", "))
	return false
}

// insertRegex is parser.c insert_regex.
func (ps *parser) insertRegex(e *parserEntry, syn regex.Syntax) bool {
	rx, ok := ps.collectArg()
	if !ok {
		return false
	}
	p := ps.insertPrimaryWithPred(e, predRegex, fnRegex, rx, true)
	p.needStat, p.needType = false, false
	re, err := regex.Compile(rx, syn, ps.r.loc.UTF8)
	if err != nil {
		ps.r.fatalf(nil, "failed to compile regular expression '%s': %s", rx, err.Error())
	}
	p.args.regex = re
	p.estSuccessRate = estimatePatternMatchRate(rx, true)
	return true
}

func parseRegex(ps *parser, e *parserEntry) bool  { return ps.insertRegex(e, ps.r.opt.RegexSyntax) }
func parseIregex(ps *parser, e *parserEntry) bool { return ps.insertRegex(e, ps.r.opt.RegexSyntax|regex.ICase) }

// predRegex is pred.c pred_regex.
func predRegex(r *Run, e *Entry, p *Predicate) bool { return p.args.regex.FullMatch(e.path) }
```
Table:
```go
		{argPositionalOption, "regextype", parseRegextype, nil, fnNone},
		{argTest, "iregex", parseIregex, predRegex, fnRegex},
		{argTest, "regex", parseRegex, predRegex, fnRegex},
```
Replace Task 2's placeholder `internal/regex/syntax.go` with this file. `Options.RegexSyntax` defaults to `regex.SyntaxEmacs`, **not** findutils-default.

- [ ] **Step 8: Gates and differential check (Review Focus 5)**

```bash
bundle exec rake qfind:build
bundle exec rspec spec/find/tests/regex_spec.rb spec/find/tests/iregex_spec.rb spec/find/positional_options/regextype_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
```
Expected: 0 failures across all 1,692 examples.

Then `.scratch/diff/bytes.sh` (Review Focus 5). It must print only `same` lines:
```bash
#!/bin/bash
set -uf # -f: the patterns below are words for find, not globs
D=/workspace/qfind.master/.scratch/diff/bytetree
G=/workspace/qfind.master/findutils-4.11.0/find/find Q=/workspace/qfind.master/dist/find
if [ ! -d "$D" ]; then
  mkdir -p "$D"
  for b in $(seq 1 255); do
    [ "$b" -eq 47 ] && continue                              # '/'
    printf -v name 'n\\x%02x' "$b"; printf -v name '%b' "$name"   # one raw byte
    touch "$D/$name" "$D/x$name" 2>/dev/null
  done
  touch "$D/caf$(printf '\xc3\xa9')" "$D/bad$(printf '\xe9')" "$D/$(printf 'a\nb')"
fi
for loc in C C.UTF-8; do
  for args in "-name *" "-regex .*" "-iregex .*[A-Z].*" "-regex .*[^a].*" "-printf %p|%f|%h\n" "-ls" "-print0"; do
    z=; [ "$args" = -print0 ] && z=-z   # NUL-separated records sort with -z
    if cmp -s <(LC_ALL=$loc exec -a find $G $D $args 2>&1 | sort $z) <(LC_ALL=$loc exec -a find $Q $D $args 2>&1 | sort $z)
    then echo "same: $loc $args"; else echo "DIFF: $loc $args"; fi
  done
done
```

- [ ] **Step 9: Commit**

```bash
gofmt -l internal; go vet ./...
git add internal && git commit -m "qfind: GNU regex syntaxes, -regex/-iregex/-regextype"
```

---

## Task 11: Numbers, sizes, links, inodes, owners, `-samefile` and `-empty`

This task ports these numeric-argument tests from `parser.c` and `pred.c`: `get_comp_type`, `get_num`, `insert_num`, `parse_size`, `parse_inum`, `parse_links`, `parse_uid`, `parse_gid`, `parse_user`, `parse_group`, `parse_nouser`, `parse_nogroup`, `parse_samefile`, `parse_empty`, `collect_arg_stat_info`, `compare_num` and their `pred_*`. Numbers go through gnulib `xstrtoumax(s, NULL, 10, &n, "")` semantics:
- leading white space and `+` are allowed;
- `-` is never allowed;
- nothing may follow the digits;
- overflow is an error.

**Files:**
- Create: `internal/libc/strtol.go`, `internal/find/parse_numeric.go`, `internal/find/pred_numeric.go`, `internal/find/pred_owner.go`
- Modify: `internal/find/parsetable.go` (11 entries), `internal/find/predicate.go` (`predArgs.num`, `.size`, `.uid`, `.gid`, `.samefile`)

**Interfaces:**
- Consumes: `r.xstatName` (Task 3), `idcache.*` (Task 7), `devIno` (Task 7), `quote`, `fatalf`, `debugf`.
- Produces:
  - `libc.Xstrtoumax(s string) (uint64, bool)`
  - `type compKind uint8` (`compGT`, `compLT`, `compEQ`), `getCompType(s string) (compKind, string)`, `compareNum(kind compKind, l, v uint64) bool`
  - `(*parser).collectArgStatInfo() (string, unix.Stat_t, bool)`, `(*Run).fatalTargetFileError(err error, name string)`
  - `predArgs.num numInfo`, `predArgs.size sizeInfo`, `predArgs.uid uint32`, `predArgs.gid uint32`, `predArgs.samefile devIno`

- [ ] **Step 1: Failing gates**

```bash
bundle exec rspec spec/find/tests/{size,links,inum,uid,gid,user,group,nouser,nogroup,samefile,empty,numeric_arguments}_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
```

- [ ] **Step 2: `internal/libc/strtol.go`**

```go
package libc

import "math/bits"

// Xstrtoumax is gnulib xstrtoumax(s, NULL, 10, &v, "") == LONGINT_OK over
// glibc strtoumax: optional white space and '+', then decimal digits to
// the end of s. A '-' (even after white space), no digits, a trailing
// character (valid_suffixes is "") and overflow all fail.
func Xstrtoumax(s string) (uint64, bool) {
	i := 0
	for i < len(s) && IsSpace(s[i]) {
		i++
	}
	if i < len(s) && s[i] == '-' {
		return 0, false
	}
	if i < len(s) && s[i] == '+' {
		i++
	}
	start := i
	var v uint64
	overflow := false
	for ; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		hi, lo := bits.Mul64(v, 10)
		sum, carry := bits.Add64(lo, uint64(s[i]-'0'), 0)
		overflow = overflow || hi != 0 || carry != 0
		v = sum
	}
	if i == start || i != len(s) || overflow {
		return 0, false
	}
	return v, true
}

// IsSpace is isspace(3) in the C locale.
func IsSpace(c byte) bool { return c == ' ' || (c >= '\t' && c <= '\r') }
```

- [ ] **Step 3: `parse_numeric.go` — comparison prefixes, `insert_num`, reference files**

```go
package find

import (
	"golang.org/x/sys/unix"

	"qfind/internal/libc"
)

// compKind is enum comparison_type (defs.h).
type compKind uint8

const (
	compGT compKind = iota
	compLT
	compEQ
)

// getCompType is parser.c get_comp_type: '+' greater, '-' less, else equal.
func getCompType(s string) (compKind, string) {
	if s != "" {
		switch s[0] {
		case '+':
			return compGT, s[1:]
		case '-':
			return compLT, s[1:]
		}
	}
	return compEQ, s
}

// compareNum is pred.c compare_num.
func compareNum(kind compKind, l, v uint64) bool {
	switch kind {
	case compGT:
		return v > l
	case compLT:
		return v < l
	}
	return v == l
}

type numInfo struct {
	kind compKind
	val  uint64
}

// insertNum is parser.c insert_num (-inum, -links, -uid, -gid).
func (ps *parser) insertNum(e *parserEntry) *Predicate {
	numstr, ok := ps.collectArg()
	if !ok {
		return nil
	}
	kind, rest := getCompType(numstr)
	v, ok := libc.Xstrtoumax(rest)
	if !ok {
		ps.r.fatalf(nil, "non-numeric argument to %s: %s", ps.argv[ps.i-2], ps.r.quote(numstr))
	}
	p := ps.insertPrimary(e, numstr, true)
	p.args.num = numInfo{kind, v}
	if ps.r.opt.Debug&DebugExpressionTree != 0 {
		names := [...][2]string{compGT: {"gt", " >"}, compLT: {"lt", " <"}, compEQ: {"eq", " ="}}
		ps.r.debugf("inserting %s\n    type: %s    %s  %d\n", p.name, names[kind][0], names[kind][1], v)
	}
	return p
}

// collectArgStatInfo is parser.c collect_arg_stat_info: options.xstat of a
// reference file named on the command line (state.curdepth is 0, so -H
// follows it), fatal on failure.
func (ps *parser) collectArgStatInfo() (string, unix.Stat_t, bool) {
	name, ok := ps.collectArg()
	if !ok {
		return "", unix.Stat_t{}, false
	}
	st, err := ps.r.xstatName(name, name, 0)
	if err != nil {
		ps.r.fatalTargetFileError(err, name)
	}
	return name, st, true
}

// fatalTargetFileError is util.c fatal_target_file_error.
func (r *Run) fatalTargetFileError(err error, name string) {
	r.fatalf(err, "%s", r.quote(name))
}

func parseInum(ps *parser, e *parserEntry) bool {
	p := ps.insertNum(e)
	if p == nil {
		return false
	}
	p.estSuccessRate = 1e-6
	p.needInum, p.needStat, p.needType = true, false, false
	return true
}

func parseLinks(ps *parser, e *parserEntry) bool {
	p := ps.insertNum(e)
	if p == nil {
		return false
	}
	switch p.args.num.val {
	case 1:
		p.estSuccessRate = 0.99
	case 2:
		p.estSuccessRate = 0.01
	default:
		p.estSuccessRate = 1e-3
	}
	return true
}

// parseUid and parseGid are parser.c parse_uid / parse_gid.
func parseUid(ps *parser, e *parserEntry) bool { return ps.insertIDNum(e) }
func parseGid(ps *parser, e *parserEntry) bool { return ps.insertIDNum(e) }

func (ps *parser) insertIDNum(e *parserEntry) bool {
	p := ps.insertNum(e)
	if p == nil {
		return false
	}
	p.estSuccessRate = 0.2
	if p.args.num.val < 100 {
		p.estSuccessRate = 0.99
	}
	return true
}

type sizeInfo struct {
	kind      compKind
	blockSize int64
	size      uint64
}

// parseSize is parser.c parse_size. GNU strips the unit suffix from argv in
// place, so -D tree shows "-size 1" for "-size 1k".
func parseSize(ps *parser, e *parserEntry) bool {
	r := ps.r
	if ps.i >= len(ps.argv) {
		return false
	}
	arg := ps.argv[ps.i]
	if arg == "" {
		r.fatalf(nil, "invalid null argument to -size")
	}
	num, suffix := arg[:len(arg)-1], arg[len(arg)-1]
	var bs int64
	switch suffix {
	case 'b':
		bs = 512
	case 'c':
		bs = 1
	case 'k':
		bs = 1024
	case 'M':
		bs = 1 << 20
	case 'G':
		bs = 1 << 30
	case 'w':
		bs = 2
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		num, suffix, bs = arg, 0, 512
	default:
		r.fatalf(nil, "invalid -size type `%s'", cbyte(suffix))
	}
	kind, rest := getCompType(num)
	v, ok := libc.Xstrtoumax(rest)
	if !ok {
		tail := ""
		if suffix != 0 {
			tail = cbyte(suffix)
		}
		r.fatalf(nil, "Invalid argument `%s%s' to -size", num, tail)
	}
	ps.argv[ps.i] = num // the in-place truncation, seen by build_expression_tree
	p := ps.insertPrimary(e, num, true)
	p.args.size = sizeInfo{kind, bs, v}
	p.needStat, p.needType = true, false
	big := v*uint64(bs) > 20480 // uintmax_t arithmetic: wraps like C
	switch {
	case kind == compGT && big, kind == compLT && !big:
		p.estSuccessRate = 0.1
	case kind == compGT, kind == compLT:
		p.estSuccessRate = 0.9
	default:
		p.estSuccessRate = 0.01
	}
	ps.i++
	return true
}

// parseSamefile is parser.c parse_samefile. O_NOFOLLOW is available, so
// the opened file's fstat wins whenever the open succeeds, and the
// descriptor stays open for the whole run (GNU's guard against inode reuse).
func parseSamefile(ps *parser, e *parserEntry) bool {
	r := ps.r
	name, st, ok := ps.collectArgStatInfo()
	if !ok {
		return false
	}
	flags := unix.O_RDONLY | unix.O_CLOEXEC
	if r.opt.Symlinks == SymlinkNeverDeref {
		flags |= unix.O_NOFOLLOW
	}
	if fd, err := unix.Open(name, flags, 0); err == nil {
		var fst unix.Stat_t
		if err := unix.Fstat(fd, &fst); err != nil {
			r.fatalTargetFileError(err, name)
		}
		if _, err := r.xstatName(name, name, 0); err != nil {
			r.fatalTargetFileError(err, name)
		}
		st = fst
		r.heldFDs = append(r.heldFDs, fd)
	}
	p := ps.insertPrimary(e, name, true)
	p.args.samefile = devIno{st.Dev, st.Ino}
	p.needType, p.needStat = false, true
	p.estSuccessRate = 0.01
	return true
}

func parseEmpty(ps *parser, e *parserEntry) bool {
	p := ps.insertPrimaryNoArg(e)
	p.estSuccessRate = 0.01
	return true
}
```
Add `heldFDs []int` to `Run`. The held descriptor is opened with `O_CLOEXEC`, so no child inherits it, and `markFDsCloexec` has nothing to do for it.

- [ ] **Step 4: `pred_numeric.go`**

```go
package find

import "golang.org/x/sys/unix"

func predInum(r *Run, e *Entry, p *Predicate) bool {
	return compareNum(p.args.num.kind, p.args.num.val, e.st.Ino)
}

func predLinks(r *Run, e *Entry, p *Predicate) bool {
	return compareNum(p.args.num.kind, p.args.num.val, uint64(e.st.Nlink))
}

func predUid(r *Run, e *Entry, p *Predicate) bool {
	return compareNum(p.args.num.kind, p.args.num.val, uint64(e.st.Uid))
}

func predGid(r *Run, e *Entry, p *Predicate) bool {
	return compareNum(p.args.num.kind, p.args.num.val, uint64(e.st.Gid))
}

// predSize is pred.c pred_size: the size in units, rounded up.
func predSize(r *Run, e *Entry, p *Predicate) bool {
	bs := p.args.size.blockSize
	f := e.st.Size / bs
	if e.st.Size%bs != 0 {
		f++
	}
	return compareNum(p.args.size.kind, p.args.size.size, uint64(f))
}

// predSamefile is pred.c pred_samefile (need_stat: the stat is current).
func predSamefile(r *Run, e *Entry, p *Predicate) bool {
	return e.st.Ino == p.args.samefile.ino && e.st.Dev == p.args.samefile.dev
}

// predEmpty is pred.c pred_empty: a directory with no entries but . and ..
// (read now, so under -depth it sees the state after its contents were
// processed), or a regular file of size 0.
func predEmpty(r *Run, e *Entry, p *Predicate) bool {
	switch e.st.Mode & unix.S_IFMT {
	case unix.S_IFDIR:
		empty, err := dirIsEmpty(e.sysPath)
		if err != nil {
			r.errorf(err, "%s", r.quote(e.path))
			r.setFailure()
			return false
		}
		return empty
	case unix.S_IFREG:
		return e.st.Size == 0
	}
	return false
}

// dirIsEmpty is pred_empty's openat/fdopendir/readdir/closedir sequence.
func dirIsEmpty(path string) (bool, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOCTTY|unix.O_NONBLOCK, 0)
	if err != nil {
		return false, err
	}
	buf := make([]byte, 8192)
	empty := true
	for empty {
		n, err := unix.Getdents(fd, buf)
		if err != nil {
			unix.Close(fd)
			return false, err
		}
		if n == 0 {
			break
		}
		var names []string
		_, _, names = unix.ParseDirent(buf[:n], -1, names) // skips "." and ".."
		empty = len(names) == 0
	}
	if err := unix.Close(fd); err != nil {
		return false, err
	}
	return empty, nil
}
```

- [ ] **Step 5: `pred_owner.go` — `-user`, `-group`, `-nouser`, `-nogroup`**

```go
package find

import (
	"math"

	"qfind/internal/idcache"
	"qfind/internal/libc"
)

// parseUser is parser.c parse_user: a name known to getpwnam, else a
// decimal UID no larger than UID_T_MAX.
func parseUser(ps *parser, e *parserEntry) bool {
	name, ok := ps.collectArg()
	if !ok {
		return false
	}
	uid, found := idcache.LookupUser(name)
	if !found {
		v, ok := libc.Xstrtoumax(name)
		if !ok || v > math.MaxUint32 {
			ps.r.fatalf(nil, "invalid user name or UID argument to -user: %s", ps.r.quote(name))
		}
		uid = uint32(v)
	}
	p := ps.insertPrimary(e, name, true)
	p.args.uid = uid
	p.estSuccessRate = 0.2
	if uid < 100 {
		p.estSuccessRate = 0.99
	}
	return true
}

// parseGroup is parser.c parse_group.
func parseGroup(ps *parser, e *parserEntry) bool {
	name, ok := ps.collectArg()
	if !ok {
		return false
	}
	gid, found := idcache.LookupGroup(name)
	if !found {
		v, ok := libc.Xstrtoumax(name)
		if !ok || v > math.MaxUint32 {
			ps.r.fatalf(nil, "invalid group name or GID argument to -group: %s", ps.r.quote(name))
		}
		gid = uint32(v)
	}
	p := ps.insertPrimary(e, name, true)
	p.args.gid = gid
	p.estSuccessRate = 0.2
	if gid < 100 {
		p.estSuccessRate = 0.99
	}
	return true
}

func parseNouser(ps *parser, e *parserEntry) bool {
	p := ps.insertPrimaryNoArg(e)
	p.estSuccessRate = 1e-3
	return true
}

func parseNogroup(ps *parser, e *parserEntry) bool {
	p := ps.insertPrimaryNoArg(e)
	p.estSuccessRate = 1e-4
	return true
}

func predUser(r *Run, e *Entry, p *Predicate) bool  { return e.st.Uid == p.args.uid }
func predGroup(r *Run, e *Entry, p *Predicate) bool { return e.st.Gid == p.args.gid }

// predNouser and predNogroup are getpwuid/getgrgid == NULL, cached.
func predNouser(r *Run, e *Entry, p *Predicate) bool  { return !idcache.UserExists(e.st.Uid) }
func predNogroup(r *Run, e *Entry, p *Predicate) bool { return !idcache.GroupExists(e.st.Gid) }
```
`idcache.LookupUser("")` must report not found, as `getpwnam("")` does. Through cgo, `os/user.Lookup` uses `getpwnam_r` and the same NSS sources as glibc, so names from LDAP and sssd resolve as they do in GNU find.

- [ ] **Step 6: Wire the table and build**

```go
		{argTest, "empty", parseEmpty, predEmpty, fnEmpty},
		{argTest, "gid", parseGid, predGid, fnGid},
		{argTest, "group", parseGroup, predGroup, fnGroup},
		{argTest, "inum", parseInum, predInum, fnInum},
		{argTest, "links", parseLinks, predLinks, fnLinks},
		{argTest, "nogroup", parseNogroup, predNogroup, fnNogroup},
		{argTest, "nouser", parseNouser, predNouser, fnNouser},
		{argTest, "samefile", parseSamefile, predSamefile, fnSamefile},
		{argTest, "size", parseSize, predSize, fnSize},
		{argTest, "uid", parseUid, predUid, fnUid},
		{argTest, "user", parseUser, predUser, fnUser},
```
```bash
bundle exec rake qfind:build
bundle exec rspec spec/find/tests/{size,links,inum,uid,gid,user,group,nouser,nogroup,samefile,empty,numeric_arguments}_spec.rb spec/find/options/debug_spec.rb spec/find/options/optimisation_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
```
Expected:
- 0 failures, except the `-mmin`/`-amin` contexts of `numeric_arguments_spec.rb`, which need Task 12.
- The inode-zero examples stay pending (skipped).
- `debug_spec.rb` is fully green; its expression uses `-empty`.

- [ ] **Step 7: Commit**

```bash
gofmt -l internal; go vet ./...
git add internal && git commit -m "qfind: numeric tests, ownership tests, -samefile and -empty"
```

---

## Task 12: Time tests and gnulib `parse_datetime`

This task ports:
- **Parsers** (`parser.c`): `get_relative_timestamp`, `parse_time` (`-atime -ctime -mtime`), `do_parse_xmin` (`-amin -cmin -mmin`), `parse_used`, `parse_newer`, `parse_anewer`, `parse_cnewer`, `parse_newerXY`, and the success-rate estimators.
- **Predicates** (`pred.c`): `pred_timewindow` and the eleven time predicates.
- **Support:** gnulib `timespec_sub`, glibc `strtod` + gnulib `xstrtod`, and gnulib `parse-datetime.y`.

`parse-datetime.y` is a bison LALR grammar with 31 shift/reduce conflicts. It is ported as a **goyacc** grammar, so the accepted language and the conflict resolution (shift) are identical. goyacc is a build-time tool dependency (`go get -tool`). It is not linked into `dist/find`, and its generated `parse.go` is committed.

**Files:**
- Create: `internal/libc/strtod.go`
- Modify: `internal/libc/tz.go` (`TZFromString`)
- Create: `internal/datetime/parse.y`, `internal/datetime/parse.go` (generated), `internal/datetime/lex.go`, `internal/datetime/tables.go`, `internal/datetime/datetime.go`, `internal/datetime/mktime.go`
- Create: `internal/find/parse_time.go`, `internal/find/pred_time.go`
- Modify: `go.mod` (tool directive), `internal/find/parsetable.go` (12 entries incl. `parseEntryNewerXY`), `internal/find/predicate.go` (`predArgs.reftime timeVal`)

**Interfaces:**
- Consumes: Task 11's `getCompType`, `compKind`, `collectArgStatInfo`, `fatalTargetFileError`; Task 2's `libc.Ctime`, `r.tz`, `Options.CurDayStart`, `Options.StartTime`.
- Produces:
  - `libc.Strtod(s string) (v float64, n int, erange bool)`, `libc.Xstrtod(s string) (float64, bool)`, `libc.TZFromString(tz string) *time.Location`
  - `datetime.Parse(s string, now datetime.Timespec, loc *time.Location) (datetime.Timespec, bool)`
  - `type timeVal struct{ xval xval; ts Timespec; kind compKind }`, `(Timespec).cmp(Timespec) int`, `timespecSub(a, b Timespec) Timespec`

- [ ] **Step 1: Failing gates**

```bash
bundle exec rspec spec/find/tests/{amin,atime,cmin,ctime,mmin,mtime,used,newer,anewer,cnewer,newerxy,numeric_arguments}_spec.rb spec/find/positional_options/daystart_spec.rb spec/find/positional_options/follow_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
```

- [ ] **Step 2: `internal/libc/strtod.go` — glibc strtod in the C locale, gnulib xstrtod**

```go
package libc

import (
	"math"
	"strconv"
	"strings"
)

// Strtod is glibc strtod(3) in the C locale. It returns the value of the
// longest prefix of s (after white space) that is a number, the length of
// that prefix (0 when there is none), and whether errno would be ERANGE:
// overflow to ±HUGE_VAL, or underflow to zero or a subnormal value.
//
// Accepted: [+-] then "inf", "infinity" or "nan[(chars)]" in any case; a
// hexadecimal "0x" mantissa with optional "p[+-]digits" (Go's ParseFloat
// requires the exponent, so it is supplied); or decimal digits with an
// optional '.' and an "e[+-]digits" exponent consumed only when digits
// follow it.
func Strtod(s string) (v float64, n int, erange bool) {
	i := 0
	for i < len(s) && IsSpace(s[i]) {
		i++
	}
	start := i
	neg := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	sign := func(f float64) float64 {
		if neg {
			return -f
		}
		return f
	}
	lower := strings.ToLower(s[i:min(len(s), i+8)])
	switch {
	case strings.HasPrefix(lower, "infinity"):
		return sign(math.Inf(1)), i + 8, false
	case strings.HasPrefix(lower, "inf"):
		return sign(math.Inf(1)), i + 3, false
	case strings.HasPrefix(lower, "nan"):
		j := i + 3
		if j < len(s) && s[j] == '(' {
			k := j + 1
			for k < len(s) && (isAlnum(s[k]) || s[k] == '_') {
				k++
			}
			if k < len(s) && s[k] == ')' {
				j = k + 1
			}
		}
		return math.NaN(), j, false
	}
	if i+1 < len(s) && s[i] == '0' && (s[i+1] == 'x' || s[i+1] == 'X') {
		j := i + 2
		mant, digits := "", 0
		for j < len(s) && isHex(s[j]) {
			j++
			digits++
		}
		if j < len(s) && s[j] == '.' {
			j++
			for j < len(s) && isHex(s[j]) {
				j++
				digits++
			}
		}
		if digits > 0 {
			mant = s[i+2 : j]
			exp := "p0"
			if j < len(s) && (s[j] == 'p' || s[j] == 'P') {
				k := j + 1
				if k < len(s) && (s[k] == '+' || s[k] == '-') {
					k++
				}
				if k < len(s) && isDigit(s[k]) {
					for k < len(s) && isDigit(s[k]) {
						k++
					}
					exp, j = "p"+s[j+1:k], k
				}
			}
			f, err := strconv.ParseFloat("0x"+mant+exp, 64)
			return sign(f), j, rangeError(f, err, strings.Trim(mant, "0.") != "")
		}
		// "0x" without hex digits: the number is the "0".
	}
	j, digits := i, 0
	for j < len(s) && isDigit(s[j]) {
		j++
		digits++
	}
	if j < len(s) && s[j] == '.' {
		j++
		for j < len(s) && isDigit(s[j]) {
			j++
			digits++
		}
	}
	if digits == 0 {
		return 0, 0, false
	}
	nonzero := strings.ContainsAny(s[i:j], "123456789")
	if j < len(s) && (s[j] == 'e' || s[j] == 'E') {
		k := j + 1
		if k < len(s) && (s[k] == '+' || s[k] == '-') {
			k++
		}
		if k < len(s) && isDigit(s[k]) {
			for k < len(s) && isDigit(s[k]) {
				k++
			}
			j = k
		}
	}
	f, err := strconv.ParseFloat(s[start:j], 64)
	return f, j, rangeError(f, err, nonzero)
}

// rangeError reports glibc's ERANGE: overflow, or an inexact underflow
// (approximated as any subnormal result, or zero from non-zero digits).
func rangeError(f float64, err error, nonzero bool) bool {
	if err != nil {
		return true // strconv.ErrRange: ±Inf
	}
	a := math.Abs(f)
	return (a != 0 && a < 0x1p-1022) || (a == 0 && nonzero)
}

// Xstrtod is gnulib xstrtod(s, NULL, &v, strtod): the whole string must be
// one number, and ERANGE with a non-zero result (overflow, or a subnormal)
// fails while underflow to zero does not.
func Xstrtod(s string) (float64, bool) {
	v, n, erange := Strtod(s)
	if n == 0 || n != len(s) {
		return v, false
	}
	if v != 0 && erange {
		return v, false
	}
	return v, true
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func isHex(c byte) bool   { return isDigit(c) || (c|0x20 >= 'a' && c|0x20 <= 'f') }
func isAlnum(c byte) bool { return isDigit(c) || (c|0x20 >= 'a' && c|0x20 <= 'z') }
```
In `tz.go`, split `LoadTZ` so that `TZFromString(tz string) *time.Location` does everything after the environment lookup. `LoadTZ` becomes `TZFromString(tz)` for a set `TZ` and `time.Local` for an unset one.

- [ ] **Step 3: goyacc as a tool dependency**

```bash
go get -tool golang.org/x/tools/cmd/goyacc@v0.50.0
go tool goyacc -h 2>&1 | head -3
```
`go.mod` gains a `tool golang.org/x/tools/cmd/goyacc` line. `datetime.go` carries `//go:generate go tool goyacc -l -o parse.go parse.y`. Commit the generated `parse.go`, and delete goyacc's `y.output` (do not commit it).

- [ ] **Step 4: `internal/datetime/parse.y` — the grammar of parse-datetime.y**

This is the grammar of `gl/lib/parse-datetime.y` lines 589–1028, rule for rule. The changes from the C version:
- Actions are Go, reached through `ctl(yylex)`.
- `YYABORT` becomes `pc.abort()`, which records failure; `Parse` then rejects the input. Parsing may continue, but it can only fail later, so the result is the same.
- The `debug_print_*` calls are dropped, because find calls `parse_datetime`, never `parse_datetime2`.

```yacc
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
```
`dayUnit(n, unit)` is the `ckd_mul (&$$.day, n, unit)` actions: it calls `pc.abort()` on overflow.

- [ ] **Step 5: `internal/datetime/tables.go` and `lex.go` — the lexical tables, `lookup_word`, `yylex`**

`tables.go` holds `meridian_table`, `dst_table`, `month_and_day_table`, `time_units_table`, `relative_time_table`, `universal_time_zone_table`, `time_zone_table` and `military_table`. Copy them from `parse-datetime.y` lines 1032–1222 into this form:
```go
package datetime

type table struct {
	name  string
	typ   int // token: tMONTH, tDAY, tZONE, … or a character such as 'J'
	value int64
}

const (
	merAM = iota
	merPM
	mer24
)

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
```
`lex.go`:
```go
package datetime

import "strings"

// lexer is yylex over the parser state.
type lexer struct {
	pc  *parserControl
	in  string
	pos int
}

func ctl(l yyLexer) *parserControl { return l.(*lexer).pc }

func (l *lexer) Error(string) {} // yyerror does nothing

func (l *lexer) at(i int) byte {
	if i < len(l.in) {
		return l.in[i]
	}
	return 0
}

func cIsSpace(c byte) bool { return c == ' ' || (c >= '\t' && c <= '\r') }
func cIsDigit(c byte) bool { return c >= '0' && c <= '9' }
func cIsAlpha(c byte) bool { return c|0x20 >= 'a' && c|0x20 <= 'z' }

// Lex is parse-datetime.y yylex.
func (l *lexer) Lex(lval *yySymType) int {
	pc := l.pc
	for {
		for cIsSpace(l.at(l.pos)) {
			l.pos++
		}
		c := l.at(l.pos)
		if cIsDigit(c) || c == '-' || c == '+' {
			p, sign := l.pos, 0
			if c == '-' || c == '+' {
				sign = 1
				if c == '-' {
					sign = -1
				}
				for {
					p++
					l.pos = p
					if c = l.at(p); !cIsSpace(c) {
						break
					}
				}
				if !cIsDigit(c) {
					continue // a lone sign is skipped
				}
			}
			var value int64
			for {
				v, ovf1 := ckdMul(value, 10)
				d := int64(c - '0')
				if sign < 0 {
					d = -d
				}
				v, ovf2 := ckdAdd(v, d)
				if ovf1 || ovf2 {
					return '?'
				}
				value = v
				p++
				if c = l.at(p); !cIsDigit(c) {
					break
				}
			}
			if (c == '.' || c == ',') && cIsDigit(l.at(p+1)) {
				if pc.notDecimal { // the second period of DD.MM.YYYY
					pc.notDecimal = false
					lval.textint = textint{negative: sign < 0, value: value, digits: int64(p - l.pos)}
					l.pos = p
					return tUNUMBER_DOTTED
				}
				if ts, end, ok := l.fraction(value, sign, p); ok {
					lval.ts = ts
					l.pos = end
					if sign != 0 {
						return tSDECIMAL_NUMBER
					}
					return tUDECIMAL_NUMBER
				} else if end < 0 {
					return '?'
				}
				pc.notDecimal = true // a DD.MM.YYYY date, not a decimal: normal_value
			} else {
				pc.notDecimal = false
			}
			lval.textint = textint{negative: sign < 0, value: value, digits: int64(p - l.pos)}
			l.pos = p
			if sign != 0 {
				return tSNUMBER
			}
			return tUNUMBER
		}
		if cIsAlpha(c) {
			var word []byte
			for {
				if len(word) < 19 { // char buff[20]
					word = append(word, c)
				}
				l.pos++
				if c = l.at(l.pos); !(cIsAlpha(c) || c == '.') {
					break
				}
			}
			tp := pc.lookupWord(string(word))
			if tp == nil {
				return '?'
			}
			lval.intval = tp.value
			return tp.typ
		}
		if c != '(' {
			l.pos++
			return int(c) // 0 at the end of the input: EOF
		}
		for count := 0; ; { // skip a parenthesised comment
			c = l.at(l.pos)
			l.pos++
			if c == 0 {
				return 0
			}
			if c == '(' {
				count++
			} else if c == ')' {
				count--
			}
			if count == 0 {
				break
			}
		}
	}
}

// fraction accumulates the digits after '.' or ',' at p to nanosecond
// precision (yylex's decimal branch). ok is false with end >= 0 when a
// second '.' shows a DD.MM.YYYY date, and with end < 0 on overflow.
func (l *lexer) fraction(s int64, sign, p int) (ts timespec, end int, ok bool) {
	p++
	ns := int64(l.at(p) - '0')
	p++
	for digits := 2; digits <= 9; digits++ {
		ns *= 10
		if l.at(p) == '.' {
			return timespec{}, 0, false
		}
		if cIsDigit(l.at(p)) {
			ns += int64(l.at(p) - '0')
			p++
		}
	}
	if sign < 0 { // truncate toward -Infinity
		for ; cIsDigit(l.at(p)); p++ {
			if l.at(p) != '0' {
				ns++
				break
			}
		}
	}
	for cIsDigit(l.at(p)) {
		p++
	}
	if sign < 0 && ns != 0 {
		var ovf bool
		if s, ovf = ckdSub(s, 1); ovf {
			return timespec{}, -1, false
		}
		ns = 1e9 - ns
	}
	return timespec{sec: s, nsec: ns}, p, true
}

// lookupWord is parse-datetime.y lookup_word.
func (pc *parserControl) lookupWord(word string) *table {
	word = strings.ToUpper(word) // c_toupper: ASCII letters and '.' only reach here
	for i := range meridianTable {
		if word == meridianTable[i].name {
			return &meridianTable[i]
		}
	}
	abbrev := len(word) == 3 || (len(word) == 4 && word[3] == '.')
	for i := range monthAndDayTable {
		name := monthAndDayTable[i].name
		if (abbrev && strings.HasPrefix(name, word[:3])) || (!abbrev && word == name) {
			return &monthAndDayTable[i]
		}
	}
	if tp := pc.lookupZone(word); tp != nil {
		return tp
	}
	if word == dstTable.name {
		return &dstTable
	}
	for i := range timeUnitsTable {
		if word == timeUnitsTable[i].name {
			return &timeUnitsTable[i]
		}
	}
	if strings.HasSuffix(word, "S") { // strip a plural
		singular := word[:len(word)-1]
		for i := range timeUnitsTable {
			if singular == timeUnitsTable[i].name {
				return &timeUnitsTable[i]
			}
		}
	}
	for i := range relativeTimeTable {
		if word == relativeTimeTable[i].name {
			return &relativeTimeTable[i]
		}
	}
	if len(word) == 1 {
		for i := range militaryTable {
			if word[0] == militaryTable[i].name[0] {
				return &militaryTable[i]
			}
		}
	}
	if strings.Contains(word, ".") { // drop periods and try the zones again
		return pc.lookupZone(strings.ReplaceAll(word, ".", ""))
	}
	return nil
}

// lookupZone is lookup_zone: universal zones, then the local zone's own
// abbreviations, then the fixed table.
func (pc *parserControl) lookupZone(name string) *table {
	for _, tbl := range [][]table{universalTimeZoneTable, pc.localTimeZoneTable, timeZoneTable} {
		for i := range tbl {
			if tbl[i].name == name {
				return &tbl[i]
			}
		}
	}
	return nil
}
```
**Port check (abbreviations):** for a 3-character word, `strncmp(word, name, 3)` compares the first three characters of both strings. So `"MAR"` matches `"MARCH"`, and `"SEP"` matches `"SEPTEMBER"` before `"SEPT"`. `strings.HasPrefix(name, word[:3])` is the same test.

- [ ] **Step 6: `internal/datetime/datetime.go` and `mktime.go` — `parse_datetime_body`**

```go
package datetime

//go:generate go tool goyacc -l -o parse.go parse.y

import (
	"strings"
	"time"

	"qfind/internal/libc"
)

// Timespec is struct timespec.
type Timespec struct{ Sec, Nsec int64 }

type timespec struct{ sec, nsec int64 }

type textint struct {
	negative bool
	value    int64
	digits   int64
}

type relTime struct {
	year, month, day, hour, minutes, seconds int64
	ns                                       int64
}

// parserControl is parser_control without the debug fields.
type parserControl struct {
	dayOrdinal         int64
	dayNumber          int64
	localIsDST         int64
	timeZone           int64 // seconds east of UT (C int)
	meridian           int
	year               textint
	month, day         int64
	hour, minutes      int64
	seconds            timespec
	rel                relTime
	timespecSeen       bool
	relsSeen           bool
	datesSeen          int
	daysSeen           int
	jZonesSeen         int
	localZonesSeen     int
	dstsSeen           int
	timesSeen          int
	zonesSeen          int
	yearSeen           bool
	notDecimal         bool
	localTimeZoneTable []table // at most two entries
	aborted            bool
}

func (pc *parserControl) abort() { pc.aborted = true }

func (pc *parserControl) setHHMMSS(h, m, s, ns int64) {
	pc.hour, pc.minutes, pc.seconds = h, m, timespec{s, ns}
}

// dayUnit is the relunit actions' ckd_mul(&$$.day, n, unit).
func (pc *parserControl) dayUnit(n, unit int64) relTime {
	d, ovf := ckdMul(n, unit)
	if ovf {
		pc.abort()
	}
	return relTime{day: d}
}

// digitsToDateTime is digits_to_date_time: YYYYMMDD, HHMM, HH, or a year.
func (pc *parserControl) digitsToDateTime(t textint) {
	if pc.datesSeen > 0 && pc.year.digits == 0 && !pc.relsSeen && (pc.timesSeen > 0 || 2 < t.digits) {
		pc.yearSeen, pc.year = true, t
		return
	}
	if 4 < t.digits {
		pc.datesSeen++
		pc.day, pc.month = t.value%100, (t.value/100)%100
		pc.year.value, pc.year.digits = t.value/10000, t.digits-4
		return
	}
	pc.timesSeen++
	if t.digits <= 2 {
		pc.hour, pc.minutes = t.value, 0
	} else {
		pc.hour, pc.minutes = t.value/100, t.value%100
	}
	pc.seconds, pc.meridian = timespec{}, mer24
}

// applyRelativeTime is apply_relative_time (factor 1 or -1).
func (pc *parserControl) applyRelativeTime(rel relTime, factor int64) bool {
	op := ckdAdd
	if factor < 0 {
		op = ckdSub
	}
	var ovf, o bool
	if pc.rel.ns, o = op(pc.rel.ns, rel.ns); o || pc.rel.ns != int64(int32(pc.rel.ns)) { // ns is a C int
		ovf = true
	}
	for _, f := range []struct{ dst *int64; v int64 }{
		{&pc.rel.seconds, rel.seconds}, {&pc.rel.minutes, rel.minutes}, {&pc.rel.hour, rel.hour},
		{&pc.rel.day, rel.day}, {&pc.rel.month, rel.month}, {&pc.rel.year, rel.year},
	} {
		if *f.dst, o = op(*f.dst, f.v); o {
			ovf = true
		}
	}
	if ovf {
		return false
	}
	pc.relsSeen = true
	return true
}

// timeZoneHHMM is time_zone_hhmm: HH or HHMM, or HH and MM with mm >= 0,
// within ±24:00.
func (pc *parserControl) timeZoneHHMM(s textint, mm int64) bool {
	if s.digits <= 2 && mm < 0 {
		s.value *= 100
	}
	var n int64
	if mm < 0 {
		n = (s.value/100)*60 + s.value%100
	} else {
		var o1, o2 bool
		n, o1 = ckdMul(s.value, 60)
		if s.negative {
			n, o2 = ckdSub(n, mm)
		} else {
			n, o2 = ckdAdd(n, mm)
		}
		if o1 || o2 {
			return false
		}
	}
	if n < -24*60 || n > 24*60 {
		return false
	}
	pc.timeZone = n * 60
	return true
}

// toHour is to_hour.
func toHour(hours int64, meridian int) int64 {
	switch meridian {
	case merAM:
		switch {
		case 0 < hours && hours < 12:
			return hours
		case hours == 12:
			return 0
		}
		return -1
	case merPM:
		switch {
		case 0 < hours && hours < 12:
			return hours + 12
		case hours == 12:
			return 12
		}
		return -1
	}
	if 0 <= hours && hours < 24 {
		return hours
	}
	return -1
}

// toTmYear is to_tm_year: two-digit years 00-68 are 20xx, 69-99 19xx;
// false when year-1900 does not fit an int.
func toTmYear(t textint) (int64, bool) {
	year := t.value
	if 0 <= year && t.digits == 2 {
		if year < 69 {
			year += 2000
		} else {
			year += 1900
		}
	}
	y := year - 1900
	return y, y == int64(int32(y)) && year >= -(1<<62)
}

// Parse is gnulib parse_datetime(result, s, now) with the default zone
// tzalloc(getenv("TZ")), passed in as loc.
func Parse(s string, now Timespec, loc *time.Location) (Timespec, bool) {
	p := strings.TrimLeft(s, " \t\n\v\f\r")
	tz := loc
	if strings.HasPrefix(p, `TZ="`) { // TZ="zone" prefix, with \\ and \" escapes
		if name, rest, ok := quotedTZ(p[4:]); ok {
			tz = libc.TZFromString(name)
			p = strings.TrimLeft(rest, " \t\n\v\f\r")
		}
	}
	tmp := localtime(now.Sec, tz)
	if p == "" {
		p = "0" // the empty string means "0", not an error during a DST change
	}
	pc := &parserControl{
		year:    textint{value: tmp.year + 1900},
		month:   tmp.mon + 1,
		day:     tmp.mday,
		hour:    tmp.hour,
		minutes: tmp.min,
		seconds: timespec{tmp.sec, now.Nsec},
	}
	pc.meridian = mer24
	pc.localTimeZoneTable = []table{{tmp.zone, tLOCAL_ZONE, tmp.isdst}}
	for quarter := int64(1); quarter <= 3; quarter++ { // the next three quarters' abbreviations
		probe := localtime(now.Sec+quarter*90*24*60*60, tz)
		if probe.isdst != pc.localTimeZoneTable[0].value {
			if probe.zone == pc.localTimeZoneTable[0].name { // one abbreviation for both
				pc.localTimeZoneTable[0].value = -1
			} else {
				pc.localTimeZoneTable = append(pc.localTimeZoneTable, table{probe.zone, tLOCAL_ZONE, probe.isdst})
			}
			break
		}
	}
	if yyParse(&lexer{pc: pc, in: p}) != 0 || pc.aborted {
		return Timespec{}, false
	}
	if pc.timespecSeen {
		return Timespec{pc.seconds.sec, pc.seconds.nsec}, true
	}
	if 1 < max(pc.timesSeen, pc.datesSeen, pc.daysSeen, pc.dstsSeen, pc.jZonesSeen+pc.localZonesSeen+pc.zonesSeen) {
		return Timespec{}, false
	}
	var tm tmT
	var ok bool
	tm.isdst = tmp.isdst
	if tm.year, ok = toTmYear(pc.year); !ok {
		return Timespec{}, false
	}
	tm.mon, tm.mday = pc.month-1, pc.day
	if tm.mon != int64(int32(tm.mon)) || tm.mday != int64(int32(tm.mday)) {
		return Timespec{}, false
	}
	if pc.timesSeen > 0 || (pc.relsSeen && pc.datesSeen == 0 && pc.daysSeen == 0) {
		if tm.hour = toHour(pc.hour, pc.meridian); tm.hour < 0 {
			return Timespec{}, false
		}
		tm.min, tm.sec = pc.minutes, pc.seconds.sec
	} else {
		tm.hour, tm.min, tm.sec, pc.seconds.nsec = 0, 0, 0, 0
	}
	if pc.datesSeen > 0 || pc.daysSeen > 0 || pc.timesSeen > 0 {
		tm.isdst = -1 // let mktime decide for an absolute time
	}
	if pc.localZonesSeen > 0 {
		tm.isdst = pc.localIsDST
	}
	tm0 := tm
	start, okMk := mktime(&tm, tz)
	if !okMk || !tm0.sameFields(&tm) {
		repaired := false
		if pc.zonesSeen > 0 { // retry in a zone at the parsed offset ("XXX" + offset)
			tm = tm0
			if start, okMk = mktime(&tm, libc.TZFromString("XXX"+timeZoneStr(pc.timeZone))); okMk {
				repaired = tm0.sameFields(&tm)
			}
		}
		if !repaired {
			return Timespec{}, false
		}
	}
	if pc.daysSeen > 0 && pc.datesSeen == 0 { // "next Tuesday"
		ord := pc.dayOrdinal
		if 0 < ord && tm.wday != pc.dayNumber {
			ord--
		}
		dayincr, o1 := ckdMul(ord, 7)
		dayincr, o2 := ckdAdd((pc.dayNumber-tm.wday+7)%7, dayincr)
		mday, o3 := ckdAdd(dayincr, tm.mday)
		if o1 || o2 || o3 || mday != int64(int32(mday)) {
			return Timespec{}, false
		}
		tm.mday, tm.isdst = mday, -1
		if start, okMk = mktime(&tm, tz); !okMk {
			return Timespec{}, false
		}
	}
	if pc.rel.year != 0 || pc.rel.month != 0 || pc.rel.day != 0 { // relative date
		year, o1 := ckdAdd(tm.year, pc.rel.year)
		month, o2 := ckdAdd(tm.mon, pc.rel.month)
		day, o3 := ckdAdd(tm.mday, pc.rel.day)
		if o1 || o2 || o3 || !fitsInt(year, month, day) {
			return Timespec{}, false
		}
		tm.year, tm.mon, tm.mday = year, month, day
		tm.hour, tm.min, tm.sec, tm.isdst = tm0.hour, tm0.min, tm0.sec, tm0.isdst
		if start, okMk = mktime(&tm, tz); !okMk {
			return Timespec{}, false
		}
	}
	if pc.zonesSeen > 0 { // the parsed zone replaces the local offset
		delta, o1 := ckdSub(pc.timeZone, tm.gmtoff)
		t1, o2 := ckdSub(start, delta)
		if o1 || o2 {
			return Timespec{}, false
		}
		start = t1
	}
	// Relative hours, minutes and seconds, ignoring leap seconds.
	sumNs := pc.seconds.nsec + pc.rel.ns
	normNs := (sumNs%1e9 + 1e9) % 1e9
	d4 := (sumNs - normNs) / 1e9
	d1, o1 := ckdMul(pc.rel.hour, 60*60)
	t1, o2 := ckdAdd(start, d1)
	d2, o3 := ckdMul(pc.rel.minutes, 60)
	t2, o4 := ckdAdd(t1, d2)
	t3, o5 := ckdAdd(t2, pc.rel.seconds)
	t4, o6 := ckdAdd(t3, d4)
	if o1 || o2 || o3 || o4 || o5 || o6 {
		return Timespec{}, false
	}
	return Timespec{t4, normNs}, true
}

// quotedTZ reads `zone"` after TZ=", undoing \\ and \" escapes.
func quotedTZ(s string) (name, rest string, ok bool) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\':
			if i+1 >= len(s) || (s[i+1] != '\\' && s[i+1] != '"') {
				return "", "", false
			}
			i++
			b.WriteByte(s[i])
		case '"':
			return b.String(), s[i+1:], true
		default:
			b.WriteByte(c)
		}
	}
	return "", "", false
}

// timeZoneStr is time_zone_str: "+HH", "+HH:MM" or "+HH:MM:SS".
func timeZoneStr(tz int64) string {
	sign := byte('+')
	if tz < 0 {
		sign = '-'
	}
	h, rem := abs64(tz/3600), abs64(tz%3600)
	s := string(sign) + pad2(h)
	if rem != 0 {
		s += ":" + pad2(rem/60)
		if rem%60 != 0 {
			s += ":" + pad2(rem%60)
		}
	}
	return s
}
```
`mktime.go` holds the `struct tm` emulation and the checked arithmetic (`<stdckdint.h>`):
```go
package datetime

import (
	"math"
	"strconv"
	"time"
)

// tmT is struct tm (years since 1900, months 0-11), plus tm_gmtoff and
// tm_zone. Fields are int64 so that C int overflow can be checked.
type tmT struct {
	year, mon, mday, hour, min, sec int64
	wday, yday, isdst, gmtoff       int64
	zone                            string
}

// localtime is localtime_rz(loc, &t).
func localtime(t int64, loc *time.Location) tmT {
	lt := time.Unix(t, 0).In(loc)
	zone, off := lt.Zone()
	isdst := int64(0)
	if lt.IsDST() {
		isdst = 1
	}
	return tmT{year: int64(lt.Year()) - 1900, mon: int64(lt.Month()) - 1, mday: int64(lt.Day()),
		hour: int64(lt.Hour()), min: int64(lt.Minute()), sec: int64(lt.Second()),
		wday: int64(lt.Weekday()), yday: int64(lt.YearDay()) - 1, isdst: isdst,
		gmtoff: int64(off), zone: zone}
}

// sameFields is mktime_ok's comparison of the requested and the
// normalised fields (tm_isdst is not compared).
func (a *tmT) sameFields(b *tmT) bool {
	return a.sec == b.sec && a.min == b.min && a.hour == b.hour &&
		a.mday == b.mday && a.mon == b.mon && a.year == b.year
}

// fitsInt reports whether every value fits a C int.
func fitsInt(vs ...int64) bool {
	for _, v := range vs {
		if v != int64(int32(v)) {
			return false
		}
	}
	return true
}

func ckdAdd(a, b int64) (int64, bool) {
	s := a + b
	return s, (b > 0 && s < a) || (b < 0 && s > a)
}

func ckdSub(a, b int64) (int64, bool) {
	d := a - b
	return d, (b > 0 && d > a) || (b < 0 && d < a)
}

func ckdMul(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, false
	}
	p := a * b
	return p, p/b != a || (a == -1 && b == math.MinInt64) || (b == -1 && a == math.MinInt64)
}

// ckdAddInt is ckd_add on a C int (pc->time_zone).
func ckdAddInt(a, b int64) (int64, bool) {
	s, ovf := ckdAdd(a, b)
	return s, ovf || !fitsInt(s)
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func pad2(v int64) string {
	if v < 10 {
		return "0" + strconv.FormatInt(v, 10)
	}
	return strconv.FormatInt(v, 10)
}
```
The core of the file:
```go
// mktime is glibc mktime_z: normalise tm in loc (out-of-range fields roll
// over, as time.Date does) and fill in wday, yday, isdst, gmtoff and zone.
// A tm.isdst of 0 or 1 that the zone contradicts is honoured the way
// __mktime_internal honours it: find an instant within ±536454000 s
// (stride 601200 s) whose isdst matches and use that instant's UTC offset.
// ok is false when the result does not fit time_t (wday stays -1 in C).
func mktime(tm *tmT, loc *time.Location) (int64, bool) {
	if tm.year > 292277026596-1900 || tm.year < -292277022657-1900 {
		return 0, false
	}
	t := time.Date(int(tm.year)+1900, time.Month(tm.mon+1), int(tm.mday),
		int(tm.hour), int(tm.min), int(tm.sec), 0, loc)
	if tm.isdst >= 0 && t.IsDST() != (tm.isdst > 0) {
		_, off := t.Zone()
	probe:
		for delta := int64(601200); delta < 536454000; delta += 601200 {
			for _, dir := range []int64{-1, 1} {
				ot := time.Unix(t.Unix()+dir*delta, 0).In(loc)
				if ot.IsDST() == (tm.isdst > 0) {
					_, ooff := ot.Zone()
					t = t.Add(time.Duration(off-ooff) * time.Second)
					break probe
				}
			}
		}
	}
	*tm = localtime(t.Unix(), loc)
	return t.Unix(), true
}
```
With `TZ=UTC` and `EST5`, the zones the suite uses, there is no DST, so the probing never runs there. It exists so that `"1 day ago"` across a DST change behaves like glibc.

- [ ] **Step 7: Generate and check the conflict count**

```bash
cd internal/datetime && go generate && grep -c 'conflicts' y.output; grep 'conflicts' y.output | tail -1; rm y.output; cd ../..
go vet ./internal/datetime
```
Expected: goyacc reports `31 shift/reduce` and no reduce/reduce conflicts. That is the `%expect 31` of the bison source. Any other number means a rule was mistranslated: compare the rule list with `parse-datetime.y`.

- [ ] **Step 8: `parse_time.go` — relative timestamps and the time parsers**

```go
package find

import (
	"math"
	"strings"
	"time"

	"qfind/internal/datetime"
	"qfind/internal/libc"
)

const daySecs = 86400

// xval is enum xval: which timestamp -newerXY compares.
type xval uint8

const (
	xvalAtime xval = iota
	xvalBirthtime
	xvalCtime
	xvalMtime
)

// timeVal is struct time_val.
type timeVal struct {
	xval xval
	ts   Timespec
	kind compKind
}

// cmp is gnulib timespec_cmp: compare seconds, then nanoseconds.
func (t Timespec) cmp(u Timespec) int {
	switch {
	case t.Sec != u.Sec:
		if t.Sec < u.Sec {
			return -1
		}
		return 1
	case t.Nsec < u.Nsec:
		return -1
	case t.Nsec > u.Nsec:
		return 1
	}
	return 0
}

// estimateFileAgeSuccessRate is parser.c estimate_file_age_success_rate.
func estimateFileAgeSuccessRate(numDays float32) float32 {
	switch {
	case numDays < 0.1:
		return 0.01
	case numDays < 1.0:
		return 0.3
	case numDays > 100.0:
		return 0.3
	}
	return 0.39
}

// estimateTimestampSuccessRate is parser.c estimate_timestamp_success_rate
// (int num_days = (cur_day_start - when) / 86400).
func (r *Run) estimateTimestampSuccessRate(when int64) float32 {
	return estimateFileAgeSuccessRate(float32(int32((r.opt.CurDayStart.Sec - when) / daySecs)))
}

// relativeTimestamp is parser.c get_relative_timestamp: the comparison is
// inverted (older than N means a timestamp less than origin-N), the count
// is a C double, and the subtraction mirrors C's double-to-time_t
// conversion (the same instruction on each platform as in C).
func (r *Run) relativeTimestamp(str string, origin Timespec, secPerUnit float64, overflowMsg string) (timeVal, bool) {
	var tv timeVal
	kind, rest := getCompType(str)
	switch kind {
	case compLT:
		kind = compGT
	case compGT:
		kind = compLT
	}
	tv.kind = kind
	offset, ok := libc.Xstrtod(rest)
	if !ok {
		return tv, false
	}
	if math.IsNaN(offset) {
		r.fatalf(nil, "invalid not-a-number argument: `%s'", rest)
	}
	seconds, frac := math.Modf(offset * secPerUnit)
	nanosec := frac * 1e9
	sec := int64(float64(origin.Sec) - seconds)
	if (origin.Sec < sec) != (seconds < 0) {
		r.fatalf(nil, overflowMsg, rest)
	}
	nsec := int64(float64(origin.Nsec) - nanosec)
	if float64(origin.Nsec) < nanosec {
		nsec += 1e9
		sec--
	}
	tv.ts = Timespec{sec, nsec}
	return tv, true
}

// parseTime is parser.c parse_time (-atime, -ctime, -mtime).
func parseTime(ps *parser, e *parserEntry) bool {
	r := ps.r
	saved := ps.i
	timearg, ok := ps.collectArg()
	if !ok {
		return false
	}
	origin := r.opt.CurDayStart
	if kind, _ := getCompType(timearg); kind == compLT {
		if origin.Sec > math.MaxInt64-(daySecs-1) {
			r.fatalf(nil, "arithmetic overflow when trying to calculate the end of today")
		}
		origin.Sec += daySecs - 1
	}
	tv, ok := r.relativeTimestamp(timearg, origin, daySecs, "arithmetic overflow while converting %s days to a number of seconds")
	if !ok {
		ps.i = saved // don't consume the invalid argument
		return false
	}
	p := ps.insertPrimary(e, timearg, true)
	p.args.reftime = tv
	p.estSuccessRate = r.estimateTimestampSuccessRate(tv.ts.Sec)
	if r.opt.Debug&DebugExpressionTree != 0 {
		names := [...][2]string{compGT: {"gt", " >"}, compLT: {"lt", " <"}, compEQ: {"eq", ">="}}
		r.debugf("inserting %s\n    type: %s    %s  %d %s", p.name, names[tv.kind][0], names[tv.kind][1],
			uint64(tv.ts.Sec), libc.Ctime(time.Unix(tv.ts.Sec, 0).In(r.tz)))
		if tv.kind == compEQ {
			t := tv.ts.Sec + daySecs
			r.debugf("                 <  %d %s", uint64(t), libc.Ctime(time.Unix(t, 0).In(r.tz)))
		}
	}
	return true
}

// parseXmin is parser.c do_parse_xmin (-amin, -cmin, -mmin): minutes back
// from the end of the reference day (cur_day_start + 1 day).
func parseXmin(ps *parser, e *parserEntry) bool {
	r := ps.r
	saved := ps.i
	minutes, ok := ps.collectArg()
	if !ok {
		return false
	}
	origin := r.opt.CurDayStart
	origin.Sec += daySecs
	tv, ok := r.relativeTimestamp(minutes, origin, 60, "arithmetic overflow while converting %s minutes to a number of seconds")
	if !ok {
		ps.i = saved
		return false
	}
	p := ps.insertPrimary(e, minutes, true)
	p.args.reftime = tv
	p.estSuccessRate = r.estimateTimestampSuccessRate(tv.ts.Sec)
	return true
}

// parseUsed is parser.c parse_used: a delta from 0, in days.
func parseUsed(ps *parser, e *parserEntry) bool {
	r := ps.r
	offset, ok := ps.collectArg()
	if !ok {
		return false
	}
	tv, ok := r.relativeTimestamp(offset, Timespec{}, daySecs, "arithmetic overflow while converting %s days to a number of seconds")
	if !ok {
		r.fatalf(nil, "Invalid argument %s to -used", offset)
	}
	p := ps.insertPrimary(e, offset, true)
	p.args.reftime = tv
	p.estSuccessRate = estimateFileAgeSuccessRate(float32(tv.ts.Sec / daySecs))
	return true
}

// insertNewer is parse_newer/parse_anewer/parse_cnewer: the reference's
// mtime (for all three) against the file's x timestamp.
func (ps *parser) insertNewer(e *parserEntry, x xval) bool {
	name, st, ok := ps.collectArgStatInfo()
	if !ok {
		return false
	}
	p := ps.insertPrimary(e, name, true)
	p.args.reftime = timeVal{xval: x, ts: tsOf(st.Mtim), kind: compGT}
	p.estSuccessRate = ps.r.estimateTimestampSuccessRate(st.Mtim.Sec)
	return true
}

func parseNewer(ps *parser, e *parserEntry) bool  { return ps.insertNewer(e, xvalMtime) }
func parseAnewer(ps *parser, e *parserEntry) bool { return ps.insertNewer(e, xvalAtime) }
func parseCnewer(ps *parser, e *parserEntry) bool { return ps.insertNewer(e, xvalCtime) }

// parseNewerXY is parser.c parse_newerXY. It is ARG_SPECIAL_PARSE: ps.i is
// at the -newerXY word itself. This build has no birth times.
func parseNewerXY(ps *parser, e *parserEntry) bool {
	r := ps.r
	if ps.i >= len(ps.argv) || len(ps.argv[ps.i]) != 8 {
		return false
	}
	word := ps.argv[ps.i]
	x, y := word[6], word[7]
	if x == 'B' || y == 'B' {
		r.errorf(nil, "This system does not provide a way to find the birth time of a file.")
		return false
	}
	const valid = "aBcmt"
	if x == 't' || strings.IndexByte(valid, x) < 0 || strings.IndexByte(valid, y) < 0 {
		return false
	}
	if ps.i+1 >= len(ps.argv) {
		r.fatalf(nil, "The %s test needs to be followed by an argument", r.quote(word))
	}
	ps.i++
	arg := ps.argv[ps.i]
	p := ps.insertPrimary(e, arg, true)
	tv := timeVal{kind: compGT, xval: map[byte]xval{'a': xvalAtime, 'c': xvalCtime, 'm': xvalMtime}[x]}
	if y == 't' {
		ts, ok := datetime.Parse(arg, datetime.Timespec(r.opt.StartTime), r.tz)
		if !ok {
			r.fatalf(nil, "I cannot figure out how to interpret %s as a date or time", r.quote(arg))
		}
		tv.ts = Timespec(ts)
	} else {
		st, err := r.xstatName(arg, arg, 0)
		if err != nil {
			r.fatalTargetFileError(err, arg)
		}
		switch y {
		case 'a':
			tv.ts = tsOf(st.Atim)
		case 'c':
			tv.ts = tsOf(st.Ctim)
		case 'm':
			tv.ts = tsOf(st.Mtim)
		}
	}
	p.args.reftime = tv
	p.estSuccessRate = r.estimateTimestampSuccessRate(tv.ts.Sec)
	ps.i++
	return true
}
```
`Timespec(r.opt.StartTime)` converts between identically laid-out struct types (`Sec`, `Nsec int64`), so this is legal Go.

- [ ] **Step 9: `pred_time.go`**

```go
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

func predAmin(r *Run, e *Entry, p *Predicate) bool  { return timeWindow(tsOf(e.st.Atim), p.args.reftime, 60) }
func predAtime(r *Run, e *Entry, p *Predicate) bool { return timeWindow(tsOf(e.st.Atim), p.args.reftime, daySecs) }
func predCmin(r *Run, e *Entry, p *Predicate) bool  { return timeWindow(tsOf(e.st.Ctim), p.args.reftime, 60) }
func predCtime(r *Run, e *Entry, p *Predicate) bool { return timeWindow(tsOf(e.st.Ctim), p.args.reftime, daySecs) }
func predMmin(r *Run, e *Entry, p *Predicate) bool  { return timeWindow(tsOf(e.st.Mtim), p.args.reftime, 60) }
func predMtime(r *Run, e *Entry, p *Predicate) bool { return timeWindow(tsOf(e.st.Mtim), p.args.reftime, daySecs) }

func predNewer(r *Run, e *Entry, p *Predicate) bool  { return tsOf(e.st.Mtim).cmp(p.args.reftime.ts) > 0 }
func predAnewer(r *Run, e *Entry, p *Predicate) bool { return tsOf(e.st.Atim).cmp(p.args.reftime.ts) > 0 }
func predCnewer(r *Run, e *Entry, p *Predicate) bool { return tsOf(e.st.Ctim).cmp(p.args.reftime.ts) > 0 }

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
```

- [ ] **Step 10: Wire, build and gate**

```go
		{argTest, "amin", parseXmin, predAmin, fnAmin},
		{argTest, "anewer", parseAnewer, predAnewer, fnAnewer},
		{argTest, "atime", parseTime, predAtime, fnAtime},
		{argTest, "cmin", parseXmin, predCmin, fnCmin},
		{argTest, "cnewer", parseCnewer, predCnewer, fnCnewer},
		{argTest, "ctime", parseTime, predCtime, fnCtime},
		{argTest, "mmin", parseXmin, predMmin, fnMmin},
		{argTest, "mtime", parseTime, predMtime, fnMtime},
		{argTest, "newer", parseNewer, predNewer, fnNewer},
		{argTest, "used", parseUsed, predUsed, fnUsed},
```
`parseEntryNewerXY = parserEntry{argSpecialParse, "newerXY", parseNewerXY, predNewerXY, fnNewerXY}`.
```bash
bundle exec rake qfind:build
bundle exec rspec spec/find/tests/{amin,atime,cmin,ctime,mmin,mtime,used,newer,anewer,cnewer,newerxy,numeric_arguments}_spec.rb spec/find/positional_options/daystart_spec.rb spec/find/positional_options/follow_spec.rb spec/find/options/symbolic_links_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
```
Expected: 0 failures. This includes:
- `-newermt` with "3 days ago", `@1600000000.123456789`, `2020-01-01T00:00:00Z` and the `%Y-%m-%d %H:%M:%S.%N %z` form;
- `TZ=EST5`;
- every NaN, exponent and hexadecimal count form;
- the `-follow` before/after reference-file cases.

Then differential-check `parse_datetime` against the oracle. Its date strings are a long tail, so compare `.scratch/gnufind . -maxdepth 0 -newermt "$s" -printf x` with `.scratch/qf` for 60 strings, under both `TZ=UTC` and `TZ=Europe/Berlin` (prefix both wrappers with the zone, e.g. `TZ=Europe/Berlin .scratch/gnufind …`; they keep a caller's `TZ`):
- `"now"`, `"yesterday"`, `"tomorrow 10am"`, `"last friday"`, `"next month"`, `"2 weeks ago"`, `"1 fortnight"`;
- `"Jan 1"`, `"1 Jan 2000"`, `"Jan-1-2000"`, `"1/2/03"`, `"2003/1/2"`, `"17.6."`, `"17.6.1992"`;
- `"20200101"`, `"20200101 +3 days"`, `"10:30pm"`, `"10:30:15.5"`, `"12am"`, `"12pm"`;
- `"EST"`, `"2020-06-01 EDT"`, `"2020-06-01 CEST"`, `"2020-01-01 J"`, `"TZ=\"Asia/Tokyo\" 2020-01-01"`;
- `"@-1.5"`, `"(comment) 2020-01-01"`, `""`, `"0"`, `"13/45/2000"` (invalid), `"2020-02-30"` (invalid), `"25:00"` (invalid).

Use a timestamp file pair just before and after each instant: `touch -d` with the oracle's own parse via `date -d`, which is the same gnulib code. The outputs must agree on every string.

- [ ] **Step 11: Commit**

```bash
gofmt -l internal; go vet ./...
git add go.mod go.sum internal && git commit -m "qfind: time tests, -newerXY and a goyacc port of parse-datetime"
```

---

## Task 13: `-perm` (gnulib modechange), access tests, `-fstype`, `-context`

This task ports the following:
- `parser.c`: `parse_perm`, `parse_accesscheck`, `parse_fstype` (including the `-O2` substitution by `-false`), `estimate_fstype_success_rate` and `parse_context`.
- The matching `pred_*` functions in `pred.c`.
- `gl/lib/modechange.c` (`mode_compile`, `mode_adjust`), complete except for `mode_create_from_ref`.
- libselinux's `is_selinux_enabled`, used by `-context` and by `--version`'s `SELINUX` feature.

**Files:**
- Create: `internal/modechange/modechange.go`, `internal/find/pred_access.go`, `internal/find/pred_misc.go`
- Modify: `internal/find/parsetable.go` (6 entries), `internal/find/predicate.go` (`predArgs.perm`), `internal/mounts/mounts.go` (`SELinuxEnabled`), `internal/find/help.go` (`SELINUX ` feature)

**Interfaces:**
- Consumes: Task 7's `mounts.FSType`, `mounts.HasType` and `r.getfilecon`; Task 5's `fnmatch.Match`; Task 3's `insertFalse`.
- Produces: `modechange.Compile(s string) (modechange.Changes, bool)`, `(modechange.Changes).Adjust(oldmode uint32, dir bool) uint32`.

- [ ] **Step 1: Failing gates**

```bash
bundle exec rspec spec/find/tests/{perm,readable,writable,executable,fstype,context}_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
```

- [ ] **Step 2: `internal/modechange/modechange.go`**

```go
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
```

- [ ] **Step 3: `pred_access.go` — `-perm`, `-readable`, `-writable`, `-executable`**

```go
package find

import (
	"golang.org/x/sys/unix"

	"qfind/internal/modechange"
)

type permKind uint8

const (
	permAtLeast permKind = iota // -MODE
	permAny                     // /MODE
	permExact                   // MODE
)

type permVal struct {
	kind permKind
	val  [2]uint32 // for a non-directory, for a directory
}

// parsePerm is parser.c parse_perm.
func parsePerm(ps *parser, e *parserEntry) bool {
	r := ps.r
	expr, ok := ps.collectArg()
	if !ok {
		return false
	}
	kind, rate, start := permExact, float32(0.01), 0
	if expr != "" {
		switch expr[0] {
		case '-':
			kind, rate, start = permAtLeast, 0.2, 1
		case '/':
			kind, rate, start = permAny, 0.3, 1
		}
	}
	mc, ok := modechange.Compile(expr[start:])
	// +NUMERICMODE was a GNU extension, withdrawn to agree with chmod.
	if !ok || (len(expr) > 1 && expr[0] == '+' && expr[1] >= '0' && expr[1] < '8') {
		r.fatalf(nil, "invalid file mode %s", r.quote(expr))
	}
	val := [2]uint32{mc.Adjust(0, false), mc.Adjust(0, true)}
	if expr[0] == '/' && val[0] == 0 && val[1] == 0 {
		r.errorf(nil, "warning: you have specified a mode pattern %s (which is equivalent to /000). The meaning of -perm /000 was changed in 2007 to be consistent with -perm -000; that is, while it used to match no files, it now matches all files. This warning message will be removed in a future findutils release.", expr)
		kind, rate = permAtLeast, 0.9986
	}
	p := ps.insertPrimary(e, expr, true)
	p.estSuccessRate = rate
	p.args.perm = permVal{kind, val}
	return true
}

// predPerm is pred.c pred_perm.
func predPerm(r *Run, e *Entry, p *Predicate) bool {
	mode := e.st.Mode
	val := p.args.perm.val[0]
	if mode&unix.S_IFMT == unix.S_IFDIR {
		val = p.args.perm.val[1]
	}
	switch p.args.perm.kind {
	case permAtLeast:
		return mode&val == val
	case permAny:
		return val == 0 || mode&val != 0 // Savannah bug 14748: /000 matches all
	}
	return mode&07777 == val
}

// parseAccesscheck is parser.c parse_accesscheck.
func parseAccesscheck(ps *parser, e *parserEntry) bool {
	p := ps.insertPrimaryNoArg(e)
	p.needStat, p.needType = false, false
	p.sideEffects, p.noDefaultPrint = false, false
	p.estSuccessRate = 0.9
	if e.id == fnExecutable {
		p.estSuccessRate = 0.2
	}
	return true
}

// The access tests are faccessat(cwd_dir_fd, rel_pathname, mode, 0):
// real IDs, following symbolic links.
func predReadable(r *Run, e *Entry, p *Predicate) bool {
	return unix.Faccessat(unix.AT_FDCWD, e.sysPath, unix.R_OK, 0) == nil
}

func predWritable(r *Run, e *Entry, p *Predicate) bool {
	return unix.Faccessat(unix.AT_FDCWD, e.sysPath, unix.W_OK, 0) == nil
}

func predExecutable(r *Run, e *Entry, p *Predicate) bool {
	return unix.Faccessat(unix.AT_FDCWD, e.sysPath, unix.X_OK, 0) == nil
}
```
`parsePerm` indexes `expr[0]` after a successful compile. An empty `expr` never compiles, so this is safe. Add `perm permVal` to `predArgs`.

- [ ] **Step 4: `pred_misc.go` — `-fstype` and `-context`**

```go
package find

import (
	"golang.org/x/sys/unix"

	"qfind/internal/fnmatch"
	"qfind/internal/mounts"
)

// parseFstype is parser.c parse_fstype. From -O2, a type that no mounted
// file system has can never match, so the test becomes -false (renamed
// back to -fstype by the parse loop, as in GNU).
func parseFstype(ps *parser, e *parserEntry) bool {
	r := ps.r
	name, ok := ps.collectArg()
	if !ok {
		return false
	}
	if r.opt.OptimisationLevel < 2 || mounts.HasType(name) {
		p := ps.insertPrimary(e, name, true)
		p.args.str = name
		p.estSuccessRate = r.estimateFstypeSuccessRate(name)
		return true
	}
	if r.opt.Debug&DebugTreeOpt != 0 {
		r.debugf("-fstype %s can never succeed, substituting -false\n", name)
	}
	return ps.insertFalse()
}

// estimateFstypeSuccessRate is parser.c estimate_fstype_success_rate.
func (r *Run) estimateFstypeSuccessRate(name string) float32 {
	var st unix.Stat_t
	if unix.Stat("/", &st) != nil {
		return 1.0
	}
	t, ok := mounts.FSType(st.Dev)
	if !ok {
		r.fatalf(nil, "Cannot read mounted file system list")
	}
	if t == name {
		return 0.7
	}
	return 0.3
}

// predFstype is pred.c pred_fstype.
func predFstype(r *Run, e *Entry, p *Predicate) bool { return r.fsType(e) == p.args.str }

// parseContext is parser.c parse_context: insert_primary(entry, NULL)
// with the pattern taken from argv directly; need_type stays set.
func parseContext(ps *parser, e *parserEntry) bool {
	r := ps.r
	if ps.i >= len(ps.argv) {
		return false
	}
	if !mounts.SELinuxEnabled() {
		r.fatalf(nil, "invalid predicate -context: SELinux is not enabled.")
	}
	p := ps.insertPrimary(e, "", false)
	p.estSuccessRate = 0.01
	p.needStat = false
	p.args.str = ps.argv[ps.i]
	ps.i++
	return true
}

// predContext is pred.c pred_context: fnmatch(pattern, context, 0); a
// getfilecon failure is diagnosed without changing the exit status.
func predContext(r *Run, e *Entry, p *Predicate) bool {
	ctx, err := r.getfilecon(e)
	if err != nil {
		r.errorf(err, "getfilecon failed: %s", r.quote(e.path))
		return false
	}
	return fnmatch.Match(p.args.str, ctx, r.fnOpts(false))
}
```
In `internal/mounts/mounts.go`, make `SELinuxEnabled` match libselinux's `is_selinux_enabled() > 0`. It requires two things:
- selinuxfs is mounted;
- `/etc/selinux/config` exists, which is what libselinux's `has_selinux_config` checks.

```go
func SELinuxEnabled() bool {
	once.Do(load)
	_, err := os.Stat("/etc/selinux/config")
	return names["selinuxfs"] && err == nil
}
```
In `help.go`'s `version()`, insert `SELINUX ` before `FTS(FTS_CWDFD) ` when `mounts.SELinuxEnabled()`. To do that, build the features line in code instead of keeping it in the constant.

- [ ] **Step 5: Wire, build and gate**

```go
		{argTest, "context", parseContext, predContext, fnContext},
		{argTest, "executable", parseAccesscheck, predExecutable, fnExecutable},
		{argTest, "fstype", parseFstype, predFstype, fnFstype},
		{argTest, "perm", parsePerm, predPerm, fnPerm},
		{argTest, "readable", parseAccesscheck, predReadable, fnReadable},
		{argTest, "writable", parseAccesscheck, predWritable, fnWritable},
```
```bash
bundle exec rake qfind:build
bundle exec rspec spec/find/tests/{perm,readable,writable,executable,fstype,context}_spec.rb spec/find/options/optimisation_spec.rb spec/find/options/version_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
./dist/find -O2 -D opt / -maxdepth 0 -fstype nosuchfs 2>&1 | grep 'can never succeed'
```
Expected: 0 failures. The last command prints `-fstype nosuchfs can never succeed, substituting -false`.

- [ ] **Step 6: Commit**

```bash
gofmt -l internal; go vet ./...
git add internal && git commit -m "qfind: -perm via modechange, access tests, -fstype and -context"
```

---

## Task 14: Locale availability and whole-suite convergence

At this point every primary exists. This task makes the whole suite match the oracle. It has two parts:
- **Locale availability.** glibc's `setlocale(LC_ALL, "")` silently falls back to `C` for every category when any category names a locale that is not installed. That is common in containers with `LANG=en_US.UTF-8` and no locale data, and it changes quote characters, multibyte handling and yes/no answers.
- **Convergence.** Run the full suite and close every remaining difference against the oracle.

**Files:**
- Modify: `internal/libc/locale.go` (availability), `internal/find/*` (fixes found by the full run)
- Create (git-ignored): `.scratch/diff/locale.sh`

**Interfaces:**
- Produces: `libc.LocaleFromEnv` now also returns the all-`C` locale when `setlocale` would fail.

- [ ] **Step 1: Record the full-suite baseline**

```bash
bundle exec rake qfind:build
bundle exec rspec --format json --out .scratch/run.json --format progress 2>&1 | grep -E '^[0-9]+ examples?, '
ruby .scratch/failures.rb .scratch/run.json
```

- [ ] **Step 2: `setlocale(LC_ALL, "")` availability**

Add to `internal/libc/locale.go`:
```go
// categories are the LC_* categories setlocale(LC_ALL, "") loads; if any
// one is unavailable the call fails and every category stays "C".
var categories = []string{"LC_CTYPE", "LC_NUMERIC", "LC_TIME", "LC_COLLATE", "LC_MONETARY",
	"LC_MESSAGES", "LC_PAPER", "LC_NAME", "LC_ADDRESS", "LC_TELEPHONE", "LC_MEASUREMENT",
	"LC_IDENTIFICATION"}

// available reports whether glibc can load a locale name: the built-in C,
// POSIX and C.UTF-8, an entry of the locale archive, or a directory under
// $LOCPATH or /usr/lib/locale (with the codeset normalised as
// _nl_normalize_codeset does: "UTF-8" is "utf8").
func available(name string, lookup func(string) (string, bool)) bool {
	if isCLocale(name) && (name == "C" || name == "POSIX" || isUTF8(name)) {
		return true
	}
	if strings.Contains(name, "/") {
		return false
	}
	norm := normalizeLocale(name)
	if dir, ok := lookup("LOCPATH"); ok && dir != "" {
		return dirExists(dir+"/"+name) || dirExists(dir+"/"+norm)
	}
	return archiveNames()[norm] || archiveNames()[name] ||
		dirExists("/usr/lib/locale/"+name) || dirExists("/usr/lib/locale/"+norm)
}

// normalizeLocale lower-cases the codeset and drops its punctuation,
// prefixing "iso" when only digits remain ("en_US.UTF-8" → "en_US.utf8").
func normalizeLocale(name string) string {
	dot := strings.IndexByte(name, '.')
	if dot < 0 {
		return name
	}
	end := len(name)
	if at := strings.IndexByte(name[dot:], '@'); at >= 0 {
		end = dot + at
	}
	var cs strings.Builder
	digitsOnly := true
	for _, c := range []byte(name[dot+1 : end]) {
		switch {
		case c >= 'A' && c <= 'Z':
			cs.WriteByte(c + 'a' - 'A')
			digitsOnly = false
		case c >= 'a' && c <= 'z':
			cs.WriteByte(c)
			digitsOnly = false
		case c >= '0' && c <= '9':
			cs.WriteByte(c)
		}
	}
	codeset := cs.String()
	if digitsOnly {
		codeset = "iso" + codeset
	}
	return name[:dot+1] + codeset + name[end:]
}

// archiveNames reads the locale names of /usr/lib/locale/locale-archive:
// struct locarhead (magic 0xde020109, then namehash offset at byte 8 and
// size at byte 16), whose namehashent records {hashval, name_offset,
// locrec_offset} point at NUL-terminated names. Read with ReadAt; the
// archive can be hundreds of megabytes.
var archiveNames = sync.OnceValue(func() map[string]bool {
	names := map[string]bool{}
	f, err := os.Open("/usr/lib/locale/locale-archive")
	if err != nil {
		return names
	}
	defer f.Close()
	var head [56]byte
	if _, err := f.ReadAt(head[:], 0); err != nil || binary.NativeEndian.Uint32(head[0:]) != 0xde020109 {
		return names
	}
	off, size := binary.NativeEndian.Uint32(head[8:]), binary.NativeEndian.Uint32(head[16:])
	table := make([]byte, 12*int(size))
	if _, err := f.ReadAt(table, int64(off)); err != nil {
		return names
	}
	buf := make([]byte, 256)
	for i := 0; i < int(size); i++ {
		nameOff := binary.NativeEndian.Uint32(table[12*i+4:])
		if nameOff == 0 {
			continue
		}
		n, _ := f.ReadAt(buf, int64(nameOff))
		if j := bytes.IndexByte(buf[:n], 0); j > 0 {
			names[string(buf[:j])] = true
		}
	}
	return names
})

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
```
In `LocaleFromEnv`, compute every category's name with the same `LC_ALL` > `LC_<CAT>` > `LANG` > `"C"` precedence. If any name fails `available`, return `Locale{CType: "C", Messages: "C", Time: "C"}`. Otherwise keep the existing result. (`LocaleFromEnv` gains no new parameter: it already has `lookup`.)

`.scratch/diff/locale.sh` compares the oracle and qfind on `find /nonexistent; find . -name 'x*' -printf '%p\n'` with output to a tty (use `script -qc`). Run it under each of these, and expect identical quotes and exit codes:
- `LANG=en_US.UTF-8` installed;
- `LANG=xx_YY.UTF-8` not installed, which must fall back to C with `'…'` quotes;
- `LANG=C.UTF-8`;
- `LC_ALL=` empty with `LANG=C.UTF-8`;
- `LC_MESSAGES=xx_YY` alone, which makes the whole call fail.

- [ ] **Step 3: Close the remaining differences, one spec file at a time**

For each file in the failure summary, from most failures to fewest, run `bundle exec rspec <file> --format documentation`. Reproduce each failing example by hand with `.scratch/gnufind` and `.scratch/qf` in a scratch directory, and fix the port by re-reading the GNU function named in the code's comment. The following cross-cutting checks are already known to need a look at this stage:
- `unusual_filenames_spec.rb`:
  - terminal output (`find_with_terminal_output` runs under a pty), which goes through `Qmark` and the line-buffered `cfile`;
  - names with newlines, tabs, ESC and invalid UTF-8.
- `placement_spec.rb` and `help_spec.rb`: the global-option placement warning wording for `-help`, `-version` and `-files0-from`.
- `debug_spec.rb` with `all`. Every narration type appears, and the run is serial.
- `files0_from_spec.rb`: 100,000 starting points with no per-start-point goroutines (profile with `QFIND_WORKERS=1` versus the default if it is slow).

- [ ] **Step 4: Pending set equals the oracle's**

```bash
FIND=$PWD/findutils-4.11.0/find/find bundle exec rspec --format json --out .scratch/oracle.json >/dev/null 2>&1
bundle exec rspec --format json --out .scratch/run.json >/dev/null 2>&1
ruby -rjson -e 'p = ->(f) { JSON.parse(File.read(f))["examples"].select { it["status"] == "pending" }.map { it["id"] }.sort }
  a, b = p.(".scratch/oracle.json"), p.(".scratch/run.json"); puts (a - b), "--", (b - a); puts a == b'
ruby .scratch/failures.rb .scratch/run.json
```
Expected:
- the pending sets are equal (the script prints `true`);
- the failure summary lists only the 15 `rlimit_nproc: 0` examples in `exec_spec.rb`, `ok_spec.rb`, `execdir_spec.rb` and `okdir_spec.rb`. Check that every failing description contains "no more processes".

- [ ] **Step 5: Commit**

```bash
gofmt -l internal; go vet ./...
git add internal && git commit -m "qfind: glibc locale availability; suite converged with the oracle"
```

---

## Task 15: Paths beyond PATH_MAX, robustness checks, performance and final verification

fastwalk addresses every file by its full path. Below `PATH_MAX` (4096 bytes) its `open` and qfind's `lstat` fail with `ENAMETOOLONG`, but GNU's fts (`FTS_CWDFD`) walks any depth.

qfind keeps fastwalk as its only walker and keeps every system-call path short. When a directory's fastwalk path gets close to `PATH_MAX`, qfind opens it and walks it again with fastwalk, rooted at `/proc/self/fd/N`. The pieces this relies on are already in place:
- `Entry.sysPath` (Task 3) is the short system-call path.
- `Entry.path` keeps GNU's spelling for output.
- The walker's `rootPaths`, `baseDepth` and `root` (Task 4) map a re-rooted walk back to real paths and depths.

The task also runs the race-detector build over the suite, the five Review Focus differential checks and the performance comparison.

**Files:**
- Create: `internal/find/deep.go`, `.scratch/bench/bench.sh` (git-ignored)
- Modify: `internal/find/visit.go` (hand deep directories to `deep.go`), `internal/find/traverse.go` (run re-rooted walks; close their descriptors)

**Interfaces:**
- Consumes: Task 4's `walker`, `rootPaths`, `dirNode`, `considerDir`, `readError`, `runDeferred` and `afterRoot`.
- Produces: `(*walker).deepen(e *Entry, key string, node *dirNode) bool`, `(*walker).walkDeep()` and `(*walker).closeDeep()`.

- [ ] **Step 1: A failing deep-tree check**

`.scratch/diff/deep.sh`:
```bash
#!/bin/bash
set -eu
D=/workspace/qfind.master/.scratch/diff/deeptree
if [ ! -d "$D" ]; then
  mkdir -p "$D"; cd "$D"
  for i in $(seq 1 700); do mkdir d123456789; cd d123456789; done # about 7,700 bytes
  touch leaf; ln -s leaf link; cd "$D"
fi
cd "$D"
G=/workspace/qfind.master/findutils-4.11.0/find/find Q=/workspace/qfind.master/dist/find
for args in "." ". -name leaf" ". -type l -printf %d:%l\n" ". -depth -name leaf" ". -name leaf -execdir pwd ;"; do
  diff <(exec -a find $G $args 2>&1 | sort | md5sum) <(exec -a find $Q $args 2>&1 | sort | md5sum) >/dev/null && echo "same: $args" || echo "DIFF: $args"
done
```
Expected before this task: `DIFF` lines with `File name too long`.

- [ ] **Step 2: `deep.go` — re-rooting at `/proc/self/fd/N`**

```go
package find

import (
	"io/fs"
	"strconv"
	"sync"

	"golang.org/x/sys/unix"
)

// deepThreshold is the fastwalk path length above which a directory is
// re-rooted: its entries' paths, at most NAME_MAX+1 bytes longer, must
// stay below PATH_MAX for fastwalk's open(2) and qfind's lstat(2).
const deepThreshold = unix.PathMax - 255 - 2

// deepDir is a directory to walk from /proc/self/fd/<fd>.
type deepDir struct {
	e    *Entry
	node *dirNode
	fd   int
}

// deepQueue is shared by every walker of one start point.
type deepQueue struct {
	mu   sync.Mutex
	todo []deepDir
	fds  []int // kept open until the start point is finished
}

// deepen is called by considerDir instead of letting fastwalk descend
// into a directory whose path is too long. The directory has already
// been considered (visited, or deferred under -depth); only its walk is
// postponed. It reports false when the directory cannot be opened, which
// is fts's FTS_DNR.
func (w *walker) deepen(e *Entry, key string, node *dirNode) bool {
	fd, err := unix.Open(e.sysPath, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		w.readError(e, &fs.PathError{Op: "open", Path: key, Err: err})
		return false
	}
	w.deep.mu.Lock()
	w.deep.todo = append(w.deep.todo, deepDir{e, node, fd})
	w.deep.fds = append(w.deep.fds, fd)
	w.deep.mu.Unlock()
	return true
}

// walkDeep walks every postponed directory, and the ones those walks
// postpone in turn. Each walk shares the start point's dirTree (cycle
// detection) and deferred list (-depth).
func (w *walker) walkDeep() {
	for {
		w.deep.mu.Lock()
		if len(w.deep.todo) == 0 {
			w.deep.mu.Unlock()
			return
		}
		d := w.deep.todo[0]
		w.deep.todo = w.deep.todo[1:]
		w.deep.mu.Unlock()
		fw := "/proc/self/fd/" + strconv.Itoa(d.fd)
		w.dirs.add(fw, d.node)
		nw := *w
		nw.rp = rootPaths{arg: d.e.path, fw: fw, fwSep: fw + "/", prefix: d.e.path + "/"}
		nw.root, nw.baseDepth = d.e, d.e.depth
		nw.walk()
	}
}

// closeDeep closes the re-rooting descriptors once nothing can use a
// sysPath below them: after the deferred visits and pending -execdir runs.
func (w *walker) closeDeep() {
	for _, fd := range w.deep.fds {
		unix.Close(fd)
	}
}
```
Add `deep *deepQueue` to `walker`, created in `findRoot` next to `dirs` and `defers`. `nw := *w` copies the walker, which contains no locks itself, only pointers to shared state. In `considerDir`, replace the final `if viaLink … return nil` with:
```go
	if len(key) > deepThreshold {
		w.deepen(e, key, w.dirs.get(key))
		return skip(viaLink)
	}
	if viaLink {
		return fastwalk.ErrTraverseLink
	}
	return nil
```
`findRoot` becomes: consider the root; `walk`; `walkDeep`; `runDeferred`; `afterRoot`; `closeDeep`.

- [ ] **Step 3: Gate the deep tree and the suite**

```bash
bundle exec rake qfind:build && .scratch/diff/deep.sh
bundle exec rspec --format progress 2>&1 | grep -E '^[0-9]+ examples?, '
```
Expected: `same:` for all five commands, and the suite is unchanged: 15 failures, all `rlimit_nproc`, and 547 pending.

- [ ] **Step 4: Race-detector run of the whole suite**

```bash
go build -race -trimpath -o .scratch/qfind-race ./cmd/qfind
FIND=$PWD/.scratch/qfind-race bundle exec rspec --format json --out .scratch/race.json --format progress 2>&1 | grep -E '^[0-9]+ examples?, '
ruby -rjson -e 'puts JSON.parse(File.read(".scratch/race.json"))["examples"].count { it.dig("exception", "message").to_s.include?("DATA RACE") }'
```
Expected: `0`. A race-enabled binary prints `WARNING: DATA RACE` on stderr and exits 66, so a race fails the example it occurs in, and its message carries the report. Expect a few extra failures that are not races: examples near resource limits (`rlimit_nofile`, `rlimit_stack`), where the race runtime's own overhead matters. Read the message of each. Fix every race by putting the shared state behind the lock its comment names.

- [ ] **Step 5: Review Focus differential checks**

Run all five scripts against the final binary:
- `.scratch/diff/pipes.sh` (Task 3)
- `.scratch/diff/many.sh` (Task 4)
- `.scratch/diff/huge.sh` (Task 4)
- `.scratch/diff/bytes.sh` (Task 10)
- `.scratch/diff/deep.sh` (Task 15)

Each must report no difference from GNU find. `huge.sh` must stay under 64 MiB peak RSS.

- [ ] **Step 6: Performance comparison**

`.scratch/bench/bench.sh` builds two trees:
- a large source tree, `references/go` (about 15,000 files);
- a synthetic 1,000,000-file tree: 100 × 100 directories of 100 files.

For each tree, it times GNU find and qfind with three runs after a warm-up, on these expressions: `-print`, `-name '*.go'`, `-type f -size +4k`, `-printf '%p %s\n'`, `-ls`, `-type f -exec true {} +`, and `-depth -print`. Report the medians in the task's commit message.

Expected: qfind is faster on every expression except `-exec … ;`, where both are bound by `fork`. If an expression is not faster, profile it with a CPU profile of the real binary (there are no Go benchmarks; no Go tests exist):
```bash
go build -o .scratch/qfind-prof ./cmd/qfind
QFIND_CPUPROFILE=.scratch/cpu.out .scratch/qfind-prof /path/to/tree -name '*.go' >/dev/null
go tool pprof -top .scratch/qfind-prof .scratch/cpu.out | head -30
```
The `QFIND_CPUPROFILE` hook is a few lines in `cmd/qfind/main.go`. When the variable is set, it calls `pprof.StartCPUProfile` and stores `pprof.StopCPUProfile` in an exported `var BeforeExit func()` in `internal/find`. `Run.exitClaimed` calls that hook just before `os.Exit`. These are the usual suspects:
- lock contention on stdout: commit records in larger batches per entry;
- `os/user` lookups: these are cached;
- `Entry` allocations: pool them only if the profile shows it.

- [ ] **Step 7: Final verification and commit**

```bash
bundle exec rake 2>&1 | grep -E '^[0-9]+ examples?, |files? inspected'     # setup, qfind:build, rubocop, spec
git status --short
```
Expected:
- rubocop, which runs first, reports `no offenses detected`, since the specs are unchanged;
- the suite reports `19214 examples, 15 failures, 547 pending`, and the 15 failures are the `rlimit_nproc: 0` examples listed in Global Constraints. They make `rake` itself exit non-zero.

```bash
gofmt -l cmd internal; go vet ./...
git add internal cmd && git commit -m "qfind: deep trees via /proc/self/fd, race-free, benchmarked against GNU find"
```
