# Black-box specs for GNU find 4.11.0

This directory holds a black-box RSpec suite for GNU `find` from findutils 4.11.0. Comments are not
allowed in the specs (see `CLAUDE.md`), so the context behind them is collected here.

## Running

| Command | Effect |
|---|---|
| `bundle exec rake` | Builds `dist/find` and the references if missing, then runs the specs and rubocop |
| `bundle exec rspec` | Runs the specs only (about 19,200 examples, about one minute) |
| `bundle exec rspec --tag upstream` | Runs only the examples translated from the findutils test suite |
| `FIND=gfind bundle exec rspec` | Tests another GNU find binary (a path, or a name looked up on `PATH`) |
| `RUN_EXPENSIVE_TESTS=yes bundle exec rspec` | Also runs the 200,000-entry memory test (same variable as upstream) |

Expected result as an unprivileged user on Linux: 0 failures and 547 pending.

| Pending | Reason |
|---|---|
| 482 | `:root` groups: real `mount`/`umount`, `mknod` and `chown` |
| 48 | Need a file with inode number 0 (only GNU/Hurd has one) |
| 12 | Need device files, which `mknod` refuses to create here |
| 4 | Known 4.11.0 bugs, written as `pending` examples (see below) |
| 1 | `:expensive`; passes in about 4.5 s with `RUN_EXPENSIVE_TESTS=yes` |

Running the suite as root swaps the `:root` and `:unprivileged` groups; the expected result is then 0
failures and 554 pending. The `:root` groups really mount file systems and create devices, all inside the
sandbox, and unmount them afterwards. Under `sudo`, keep the user's gems:
`sudo env PATH="$PATH" GEM_HOME="$(gem env user_gemhome)" GEM_PATH="$(gem env gempath)" bundle exec rspec`.

Each example gets its own sandbox directory from Ruby's `Dir.mktmpdir`, so it honours `TMPDIR`. Keep it
on a local POSIX file system such as tmpfs. The suite refuses to start when `TMPDIR` is on virtiofs: there
an unreadable directory gives "No such file or directory" instead of "Permission denied", even to root, and
cannot be made readable again, so the permission specs fail and their sandboxes are left behind. On this
machine `/workspace` is virtiofs; mount a tmpfs for the sandboxes, for example
`sudo mount -t tmpfs -o size=1g,uid=$(id -u),gid=$(id -g) tmpfs DIR`, and set `TMPDIR=DIR`.

## How the CLAUDE.md rules are enforced

- **Black box only.** `spec_helper.rb` uses `mock_with :nothing`, so `double`, `allow` and `anything` do
  not exist. Every example reaches find through the helpers below; a runtime audit confirmed that each of
  the 18,671 examples that run spawns find at least once. The 543 skipped examples (every pending one
  except the 4 known bugs, whose bodies run) were not audited.
- **One expectation per example.** Enforced by rubocop-rspec (`RSpec/MultipleExpectations`). There are
  no compound matchers (`.and`/`.or`).
- **No comments.** Checked with Ripper over every file in `spec/`. This file is the only place for notes.

## Architecture (`spec/support`)

- **`GnuFind`** runs the binary with `argv[0]` set to `find`, so diagnostics read `find: ...` exactly as
  upstream expects.
  - Environment: `LC_ALL=C`, `TZ=UTC`, `GNU_FINDUTILS_FD_LEAK_CHECK=1` (as upstream's DejaGnu
    `config/unix.exp` sets it); `POSIXLY_CORRECT`, `LANG` and `LANGUAGE` unset. Override per call with
    `env:`; a `nil` value unsets a variable.
  - stdin is a pipe by default, so find defaults to `-nowarn`. Extra keyword arguments go to
    `Process.spawn` (for example `rlimit_nofile:`, `rlimit_as:`, `rlimit_nproc:`, `umask:`).
- **Helpers** available in every example: `find`, `find_on_terminal` (stdin is a pseudo-terminal),
  `find_with_terminal_output` (stdout is a raw-mode pseudo-terminal), `find_with_stdin_from(path, ...)`
  and `find_with_stdout_to(path, ...)` (for example `/dev/full`).
- **`FindResult`** holds `stdout`, `stderr` and `status`, plus `lines`, `records` (NUL-separated) and
  `diagnostics` (stderr lines).
- **`Sandbox`** is a fresh directory per example with fixture builders (`mkdir`, `touch`, `write`,
  `symlink`, `link`, `chmod`, `utime`, `lutime`, `truncate`, `mkfifo`, `socket`). Teardown makes every
  directory traversable again before removing it, so `chmod 000` fixtures are safe.
- **Matchers:** `be_successful`, `have_exit_status`, `print_lines` (order-insensitive like upstream;
  `.in_order` for ordered output), `print_nothing`, `print_exactly` (exact bytes), `diagnose` (some stderr
  line matches), `diagnose_exactly` (all stderr lines; `.in_order`), `diagnose_nothing`.
- **Shared examples** in `searches.rb`. Each runs at `-O0`, `-O1`, `-O2` and `-O3`, because upstream's
  harness runs every test at every optimisation level and requires identical results:
  - "a successful search": exit 0, the given lines, empty stderr;
  - "a successful search that warns": exit 0, the given lines, a given warning;
  - "a failed search": exit 1, the given lines;
  - "a rejected command line": exit 1, no output, a given diagnostic.

  The other files in `shared_examples/` hold behaviour shared by synonyms or related primaries, for
  example `-path`/`-wholename`, `-a`/`-and` or `-exec`/`-ok`.
- **Group macro** `at_every_optimisation_level do ... end` repeats custom examples at each level.
- **Oracles** never use find itself:
  - `MountTable` reads `/proc/self/mountinfo` for `-fstype` and `%F`;
  - `Locales` reads `locale -a`;
  - expected inodes, sizes, blocks, owners and times come from `File.stat`/`File.lstat`, `Etc` and
    `Time#strftime`.
- **Metadata gates** live in `spec_helper.rb`:
  - `:root` (skipped unless uid 0);
  - `:unprivileged` (skipped as root, since root bypasses permission checks);
  - `:expensive`;
  - `locale: "en_US.UTF-8"` (skipped when the locale is not installed).

## Layout

The files mirror the man page: `options/` (`-H` `-L` `-P` `-D` `-O` `--help` `--version`),
`global_options/`, `positional_options/`, `tests/` (one file per test primary), `actions/` (one file per
action; `-printf` is split into general, escapes, names, attributes and times), `operators/`, plus
`starting_points_spec.rb`, `expressions_spec.rb` (unknown predicates, "paths must precede expression"),
`traversal_spec.rb` and `unusual_filenames_spec.rb` (quoting on terminals versus pipes).

## Translation of the upstream test suite

All 150 upstream tests for find are translated: 124 DejaGnu tests from `find/testsuite/find.gnu` and
`find.posix`, 25 shell tests from `tests/find`, and the find part of `tests/misc/help-version.sh`.

- Every translated group carries `upstream: "<path>"` metadata, for example
  `upstream: "find.gnu/name-period.exp"` or `upstream: "tests/find/newer.sh"`.
- These groups account for 4,748 of the 19,214 examples (24.7%). The rest is new coverage of the
  public surface described in `find.1`.
- To find the spec for an upstream test, run `grep -rn 'upstream: "find.gnu/printf.exp"' spec`.

How the upstream harness semantics map to the shared examples:

| Upstream | RSpec |
|---|---|
| DejaGnu `find_start p {...}` (exit 0, sorted stdout compared with the `.xo` file, run at `-O0` to `-O3`) | "a successful search" |
| DejaGnu `find_start f {...}` (non-zero exit) | "a failed search", or "a rejected command line" for parse errors |
| Shell tests (default level only; `returns_ 1`, `compare`, `grep`) | Plain examples |

- Tcl `exec` fails whenever the command writes to stderr. A passing upstream test therefore also
  asserted empty stderr, unless it redirected `2>/dev/null`; "a successful search" keeps that check.
- Parse errors now check the exact diagnostic where upstream only checked for a non-zero exit.

Fixtures adapted to keep tests hermetic and deterministic:

- Sandboxes replace upstream's `tmp` directory in the build tree. The `exec-*-rtn-*` tests searched
  `/tmp`; they now search the sandbox.
- The `LINK`/`BROKEN` fixture links to a file in the sandbox instead of `/etc/passwd`.
- `newer.sh` used `sleep 1`. Access and modification times are now set with `utime`, and increasing
  change times are obtained by recreating a file until its ctime advances.
- `ulimit -n` and `ulimit -v` became the `rlimit_nofile` and `rlimit_as` spawn options.
- The `mount-vs-xdev.sh` mount point is chosen at runtime by comparing device numbers. The readdir race
  uses a Ruby thread. The OOM test finds the minimal address-space limit with `rlimit_as`.
- `perm.exp` and `perm-X.exp` set modes explicitly instead of relying on umask 022.
- The manual's `[%h][%f]` example uses `/dev` and `/dev/null` instead of `/tmp` paths.
- `inode-zero.sh` is translated but skipped unless `/dev/console` has inode 0.

Defects noticed in the upstream tests themselves:

- `arg-nan.sh` has a typo (`> outid not-a-number argument >out`) that passes stray arguments.
- `mount-vs-xdev.sh` compares `out-m` where it means `out-mx`.
- `name-slash.sh` compares `out` instead of `err` in its no-warnings branch.

## Known 4.11.0 bugs

A clear defect is never specified as correct behaviour.

- When the documented behaviour is known, the spec is an RSpec `pending` example of that behaviour. It
  is reported as pending while the bug exists, and fails with "Expected pending ... to fail" once fixed,
  which is the cue to remove the `pending` line.
- When the correct output is not defined anywhere, the spec checks only what is certainly right, such
  as the rejection, and not the faulty text.

Behaviour that merely differs from loose wording in the man page is specified as it is (next section).

- **`-ok` and `-okdir` close the command's standard input** (4 pending examples) instead of
  redirecting it from `/dev/null`.
  The reopened `/dev/null` ends up on a descriptor of 3 or higher, so under
  `GNU_FINDUTILS_FD_LEAK_CHECK` every confirmed command is aborted ("terminated by signal 6"). Most
  `-ok`/`-okdir` examples therefore run with that variable unset.
- **Unfinished directive at the end of a `-printf` format.** `-printf 'x %-5'` is rejected with a "format
  directive is reserved" error that contains a raw NUL byte. The spec checks only that it is rejected.
- **Wrong item in a `-D` warning.** For a list with an unknown item, "Ignoring unrecognised debug flag"
  names the first item of the list rather than the unknown one. The spec checks only the message prefix.

## Man page versus actual 4.11.0 behaviour

The specs describe what the binary does. These are the places where that differs from `find.1`, or goes
beyond it, as found while writing them.

Options and global options:
- `-ignore_readdir_race` still diagnoses a starting point that does not exist (exit 1). Any `-execdir` or
  `-okdir` later on the command line silently switches it off again.
- `-noleaf` has no effect in this fts-based build.
- `-maxdepth`/`-mindepth`:
  - the error asks for a "positive" integer although 0 is accepted;
  - `-0` is rejected;
  - leading zeros are decimal (`010` is ten);
  - values above `INT_MAX` give "Numerical result out of range" without naming the option.
- The warning about a global option placed after a non-option argument:
  - appears only when warnings are on and `POSIXLY_CORRECT` is unset;
  - is not enabled by a `-warn` placed after the option;
  - names the first non-option argument, counting actions and operators as such;
  - is also triggered by `-help`, `-version` and `-files0-from`.
- `-O0` is not equivalent to `-O1`: only name tests are moved forward, as `-D opt` shows. `--version`
  reports `CBO(level=0)`. Levels 4 to 65535 are accepted silently; 65536 and above are rejected.
- The `-D` list in the man page omits `time`.
- `-H` and `-L` fall back to the link itself only for broken links. A loop ("Too many levels of
  symbolic links") or an unreachable target ("Permission denied") is an error with exit 1, and is fatal
  for a `-newer` reference.
- `-follow` also affects tests written before it, and the starting points. Only the reference files of
  `-newer`, `-anewer`, `-cnewer` and `-newerXY` depend on where it appears.
- Under `-L` or `-follow`, `-lname` and `-ilname` still match broken links; they are not always false.
- `-nowarn` does not silence these warnings:
  - a `-path`/`-wholename` pattern ending in `/`;
  - `-perm /000`;
  - the dash-prefixed operators `-(`, `-!`, `-,`, `-)`;
  - `-printf` format warnings.

  With `POSIXLY_CORRECT` set, `-warn` does not enable the usual command-line warnings, but the
  dash-prefixed operator warning still appears.
- ERRORS section:
  - find prints "unknown predicate", not "invalid predicate";
  - `find . -print )` gives "you have too many ')'";
  - "unexpected extra predicate" appears to be unreachable.
- `-files0-from` with `-ok`/`-okdir`: only a list read from standard input, or from the same file as
  standard input, is rejected. A separate list file works.
- Birth time is not available in this build. Every `-newerXY` combination involving `B` is rejected
  with "This system does not provide a way to find the birth time of a file.". `%Bk` prints nothing,
  and `%B@` prints `-1.-000000010`.

Tests:
- `-perm +mode`:
  - `+111` is rejected as an invalid mode;
  - a valid symbolic mode such as `+w` is accepted as an exact mode (`0222`);
  - `POSIXLY_CORRECT` changes nothing.
- Symbolic `-perm -mode` and `/mode` without `u`/`g`/`o` are accepted (`-perm -+w` is `-222`).
- `-xtype` on a starting point under `-H` checks the link itself.
- A self-referencing link under `-L` is reported as "Too many levels of symbolic links" with exit 1
  instead of being treated as broken.
- `-context`: the binary links libselinux. Without SELinux it is rejected when the command line is
  parsed, and the missing-argument check comes first.
- `-regextype help` is an error listing the valid types. `ed`, `sed` and `posix-minimal-basic` are
  accepted although the Texinfo manual does not list them. As in Emacs, `.` does not match a newline by
  default, contrary to the Texinfo manual. A regex ending in `/` can match a starting point given with a
  trailing slash.
- Numeric arguments accept a sign followed by `+` or leading blanks (`-size ++1`, `" 1"`). No spec
  covers this.

Actions:
- `-exec ... ;` and `-ok`: if the command cannot be run (not found, permission denied) or is killed by
  a signal, the test is false but find still exits 0. The `+` form makes find exit 1.
- `-quit` always runs pending `-exec ... {} +` command lines; only fatal errors drop them. The man page
  says they may not run.
- The "`{}` must appear by itself" diagnostic is unreachable. `{} x +` gives "missing argument to
  `-exec'", and `{} + ;` gives "paths must precede expression".
- `-execdir ... {} +` never groups files from different starting points, even in the same directory;
  `-exec ... {} +` does. With `PATH` unset, the `-execdir` safety check is skipped. A starting point of
  `.` runs the command in `.` itself with `./.`.
- `-ok` answers: no findutils translations are installed, so the system's yes/no patterns are used with
  or without `POSIXLY_CORRECT`. With `LC_MESSAGES=en_US.UTF-8`, `1` counts as yes.
- `-ls`/`-fls`:
  - escape a vertical tab as octal `\013` but a backspace as `\b`;
  - show the target under the link's name with `-L`, without `-> target`;
  - show the time of day only for recent timestamps;
  - treat `FIND_BLOCK_SIZE` in the environment as a fatal error.
- `-fprint`, `-fprint0`, `-fls` and `-fprintf` open their files while the command line is parsed, so
  `find . -fprint out` lists `./out`. `-fprintf out` without a format reports "invalid argument" rather
  than "missing argument".
- `-printf`, directives and escapes:
  - an unknown `%x` prints both `%` and `x` and warns;
  - `%5%` prints `%5`;
  - a trailing `%` is a clean error, not undefined behaviour;
  - `\NNN` above 0377 wraps modulo 256;
  - `\c` also stops parsing the rest of the format.
- `-printf`, printf(3) syntax and flags:
  - only flags, width and precision are supported: `%*d` and `%1$p` warn, and `%ld` is `%l` followed by
    a literal `d`;
  - only `-`, `+`, space and `#` are flags, and `0` counts only as the first digit of a width;
  - `%S` honours `#` and `+`, while `%m` ignores `+`.
- `-printf`, name and file directives:
  - `%h` is empty only when the only slash is a leading one;
  - `%f` keeps one trailing slash;
  - `%a`, `%c` and `%t` include a 10-digit fraction of a second;
  - `%S` of an empty file prints `1`;
  - `%Y` on a link whose target cannot be examined prints `?` and a diagnostic, but find exits 0;
  - `%Z` on a file without an SELinux label prints a "getfilecon failed" diagnostic and find exits 1.

## Verification status and assumptions

- Verified at commit `5af8dad` as uid 1000:
  - `bundle exec rake` exits 0 (rubocop: 113 files, no offenses);
  - the suite also passed with several random seeds, including two concurrent runs;
  - no sandbox directories were left behind.
- Verified at commit `d1cc144` with `TMPDIR` on tmpfs: 0 failures and 547 pending as uid 1000, and 0
  failures and 554 pending as root. The root run exercises the `:root` groups (bind mounts, an ext2
  loopback mount, `mknod` and `chown` to an unused uid) and left no mount, loop device or sandbox behind.
- Platform assumptions: Linux, `/proc`, `/dev/null`, `/dev/full`, the `C.UTF-8` and `en_US.UTF-8`
  locales (gated), and a mount point of another device under `/` for `mount-vs-xdev.sh` (`/dev` here;
  skipped when none exists).
- Only find is covered. The upstream `xargs` and `locate` tests are out of scope, because `dist/` only
  ships find.
