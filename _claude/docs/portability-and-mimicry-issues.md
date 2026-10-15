# Portability and GNU-mimicry issues (recorded 2026-10-15)

Trigger: `rake` on macOS failed to compile qfind at `internal/libc/errno.go:61` with
`undefined: unix.ECHRNG` and 44 more errors. The user had been given performance
claims they could not verify because the binary would not build on their machine.

## 1. The misreading that caused all of it

CLAUDE.md asks for "100% congruence with GNU find". Congruence is an external
comparator: the two tools, superimposed, present identical surfaces (stdout, stderr,
exit status, and the three ordering rules the spec suite pins). Earlier sessions read
it as "port the findutils C source to Go" and copied GNU's and glibc's internals
function by function. The Go code says so itself: 248 comments reference `pred.c`,
`parser.c`, fts, glibc or gnulib as justification. Because glibc is Linux, Linux
leaked into untagged files everywhere, and the extra machinery costs speed.

The black-box suite in `spec/` never asked for any of this. It is declared Linux-only
(`spec/README.md:274`) but pins behaviour, not implementation.

## 2. Linux-only code, item by item

| # | What | Where | Forced by a spec? | Verdict |
|---|------|-------|-------------------|---------|
| 1 | Full 133-row glibc `strerror` table incl. `EL2NSYNC`, `ECHRNG`, XENIX entries | `internal/libc/errno.go` (now `errno_linux.go`) | No. Specs pin ~8 common messages ("No such file or directory", "Permission denied", "Too many levels of symbolic links", "Is a directory" at `spec/support/shared_examples/printing.rb:49`, "Directory not empty" at `spec/find/actions/delete_spec.rb:88`, "No space left on device"). No Linux-only errno appears in `spec/`. | Mimicry. Replace with a short map. |
| 2 | Raw `unix.Stat_t` fields (`Dev` uint64/int32, `Mode` uint32/uint16, `Rdev`, `Nlink`, `Blksize`) used in 11 files with Linux widths | `internal/find/{entry,ls,parse_numeric,pred_access,pred_misc,pred_numeric,pred_types,printf_render,sharefile,traverse,visit}.go` | Yes, indirectly: `-ls`, `-printf %D %i %n %b`, `-samefile`, `-xdev`, `-cmin`, `-newerXY` need dev/ino/nlink/ctime that `os.FileInfo` does not expose. | Legitimate need, wrong packaging. Isolate behind GOOS-suffixed accessors. |
| 3 | `getdents(2)` for `-empty` (copying "pred_empty's openat/fdopendir/readdir/closedir sequence") | `internal/find/pred_numeric.go:56-80` | No. `spec/find/tests/empty_spec.rb` checks results only. | Mimicry. Use `(*os.File).Readdirnames(1)`. |
| 4 | `TCGETS` ioctl for isatty | `internal/find/options.go:94-97` | Partly. PTY-driven specs exist via `spec/support/find_helpers.rb:6` (`warn_spec.rb`, `traversal_spec.rb`, `unusual_filenames_spec.rb`), so tty detection is needed, the ioctl is not. | Mimicry. Use a `ModeCharDevice` check or `x/term`. |
| 5 | `/proc/self/fd/N` re-rooting for paths beyond ~3,800 bytes | `internal/find/deep.go:80` | No spec builds a path past PATH_MAX. | Linux-only design dependency arising from unmodified fastwalk; pinned by nothing. Keep as Linux-only, document, or drop. |
| 6 | `/proc/self/fd` listing to decide which fds children inherit | `internal/find/launch.go:278` | Indirectly (fd-inheritance specs). | Needs an OS pair: `/dev/fd` on Darwin. |
| 7 | `/proc/self/mountinfo` for `-fstype` and `%F` | `internal/mounts/mounts.go` | Yes: `spec/find/tests/fstype_spec.rb`; the harness itself reads mountinfo at `spec/support/mount_table.rb:6`. | Legitimate per-OS need. Darwin pair via `getfsstat(2)`. |
| 8 | ARG_MAX = RLIMIT_STACK/4 clamped, glibc's `sysconf` formula | `internal/libc/sysconf.go` (now `sysconf_linux.go`) | Yes: `spec/find/actions/exec_spec.rb:191` sets `rlimit_stack: 512 KiB` and expects "The environment is too large for exec()." | Legitimate on Linux. Darwin uses `kern.argmax` sysctl. |

Summary: 2 items are forced by specs (7, 8), 1 is a genuine need packaged wrongly (2),
1 is an fd-listing detail needing a pair (6), 1 is a fastwalk-driven Linux dependency
nothing pins (5), and 3 are pure mimicry (1, 3, 4).

## 3. Process failures

- Nothing ever ran `GOOS=darwin go build ./...`. The Rakefile builds only for the host
  (`Rakefile:171`). A one-line cross-compile gate would have caught this on day one.
- OS-specific identifiers were placed in unsuffixed files, against Go convention
  (`_linux.go` / `_darwin.go` pairs).
- Performance claims from the previous session (2.5× GNU on ext4, 1.2× on virtiofs)
  were reported to a user who could not build the binary to check them. Unverified by
  the user as of this writing.

## 4. State of the working tree right now (uncommitted, partially applied)

A build-tag split was started before the user stopped work, and one command was
rejected mid-run, leaving the tree **broken on Linux as well**:

- `internal/mounts/mounts.go:40: undefined: os` — `unescape` was left in `mounts.go`
  while its imports were moved to `mounts_linux.go`.
- New files: `internal/libc/{errno_linux,errno_other,sysconf_other}.go`,
  `internal/find/{sys_linux,sys_other}.go`, `internal/mounts/{mounts_linux,mounts_darwin}.go`;
  `sysconf.go` renamed to `sysconf_linux.go` with a new `sysconf.go` holding `legacyArgMax`.
- Casts `uint64(st.Dev)` / `uint32(st.Mode)` / `uint64(st.Rdev)` added at ~30 sites in
  `internal/find`.
- This direction (hide the mimicry per OS) was judged wrong by the user. Either
  `git checkout -- . && git clean -fd internal` to discard it, or finish only the parts
  that stay under the de-mimicry plan (items 2, 6, 7, 8 above).

## 5. Agreed direction (not yet authorised to start)

1. Strip mimicry: short errno message map; `Readdirnames` for `-empty`; char-device
   isatty; delete the `pred.c`/`parser.c`/glibc justification comments.
2. Keep OS-specific code only for raw stat fields, fd listing, mount table and ARG_MAX,
   each as a `_linux.go` / `_darwin.go` pair.
3. Add `GOOS=darwin GOARCH=arm64 go build ./...` (and amd64) to `rake` as a gate.
4. Re-run the suite: must still read 15 `rlimit_nproc` failures and 547 pending as uid
   1000, parity with GNU as root. Then have the user rebuild on macOS and time it.

## 6. Unrelated untracked files noticed

`dist_find_stderr_output.log` and `find_stderr_output.log` at the repo root are
untracked and not part of the build. Origin unknown to this session.

## 7. Files referencing findutils/glibc/gnulib internals

Pattern: C file names (`parser.c`, `pred.c`, …), `fts_*`/`FTS_*`, glibc, gnulib, findutils, GNU, Savannah. 95 of 101 Go files match. Most-cited: parser.c 71, pred.c 35, tree.c 33, util.c 24, defs.h 17, print.c 16.

Only six files are clean: `internal/libc/ctime.go`, `internal/datetime/lex.go`, `internal/find/workers.go`, and the three new `sys_*`/`sysconf_other.go` files.

| Package | Files | Refs |
|---|---|---|
| internal/find | 52 | 370 |
| internal/regex | 16 | 73 |
| internal/libc | 12 | 43 |
| internal/fnmatch | 4 | 18 |
| internal/datetime | 4 | 17 |
| internal/mounts | 3 | 8 |
| internal/modechange | 1 | 1 |
| internal/idcache | 1 | 1 |
| internal/filemode | 1 | 1 |
| cmd/qfind | 1 | 1 |

Full per-file counts: `grep -rcE '\b[a-z_0-9]+\.[ch]\b|fts_|FTS_|glibc|gnulib|findutils|GNU|Savannah' --include=*.go cmd internal`.
