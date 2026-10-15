# qfind

IMPORTANT: DO NOT WRITE TO /tmp. EVER!
## Task

Reimplement all of GNU find (findutils 4.11.0) in Go as a new tool, **qfind**, specifically on top of the fastwalk Go library (`github.com/charlievieth/fastwalk`, source in `references/fastwalk`).

- The goal is 100% congruence with GNU find, but with parallelism. It should look, feel and work like GNU find, albeit much faster. Congruence is defined in the next section; read it before writing any code.
- An exhaustive black-box RSpec suite has been prepared in `spec/`. It only requires a binary at `dist/find`.
- Do not write any tests in Go. Rely solely on the spec suite; the throwaway differential checks against GNU find in git-ignored `.scratch/diff/` are not tests.
- This is a streaming problem, so order does not matter (GNU find does not guarantee it either). Do not provide any order guarantees beyond what GNU semantics force: starting points in command-line order, a directory before its contents (after them with `-depth`), and one file's actions in sequence.
- fastwalk is deliberate: fzf uses it, and fzf is one of the gold-standard CLI tools of the last decade. Use it unmodified and adapt qfind to its model rather than forking it.

`_claude/plans/master_plan.md` is void. It prescribed porting findutils "structure-for-structure", which produced the code deleted in commit 7f47d8d; do not follow it or revive its approach. `_claude/docs/portability-and-mimicry-issues.md` records what went wrong.

## Congruence: the only rules that matter

Congruent means superimposed: two shapes laid on top of each other with identical surfaces. It is an external comparator. The surface of qfind is exactly what a black-box observer can see, and nothing else:

- the bytes on stdout and stderr;
- the exit status;
- the side effects of actions: children spawned by `-exec`/`-ok`/`-execdir`/`-okdir` and their argv, cwd, environment and descriptors; files removed by `-delete`; files written by `-fprint*`/`-fls`;
- the three ordering constraints listed above.

Anything the suite cannot observe is not part of the surface and is yours to design for speed in idiomatic Go. This leads to hard rules:

1. **Never port C.** Do not transliterate findutils, gnulib or glibc source into Go. No Go function may exist because a C function exists. No `fts` state machine, no `fts_info` enum, no `parser.c`/`tree.c`/`pred.c` structure, no glibc tables, no gnulib hash-table layouts, no faked assertion messages. If you find yourself opening `findutils-4.11.0/` to copy how something is done rather than to check what it prints, stop.
2. **No C citations in Go.** A comment naming a C file, C function, fts flag, glibc or gnulib as the reason code is shaped a certain way is a defect. Cite the spec that pins the behaviour instead (`spec/find/...:line`). The grep `grep -rnE '\b[a-z_0-9]+\.[ch]\b|fts_|FTS_|glibc|gnulib' --include=*.go cmd internal` must return nothing.
3. **Specs justify quirks, not GNU's code.** When a spec pins a GNU quirk, implement it in the simplest Go that yields the observable result. When no spec pins a behaviour, do what is natural and fast in Go; do not go looking in the C for what GNU "would" do.
4. **Reference `findutils-4.11.0/find/find`, the binary, not the source.** The oracle is the built executable: run it, diff its output against qfind's. The differential checks in `.scratch/diff/` exist for that.
5. **Design for fastwalk, not for fts.** qfind is a Go program on fastwalk's parallel callback model that happens to produce find's output. It is not find with the traversal swapped out.

## Portability

qfind must build and run on Linux and macOS. The spec suite is Linux-only (`spec/README.md`, "Platform assumptions"); that is a property of the harness, not a licence for Linux-only code.

1. **Standard library first.** Use `os`, `io/fs`, `path/filepath`, `os/exec` and `golang.org/x/term`. Reach for `golang.org/x/sys/unix` only for what the standard library cannot expose: raw stat fields (`dev`, `ino`, `nlink`, `ctime`, `blocks`, `rdev`) and the mount table.
2. **OS-specific identifiers live in GOOS-suffixed files.** Any `unix.*` constant, syscall or struct field whose existence or type differs by OS goes in `*_linux.go` with a `*_darwin.go` counterpart (or `//go:build !linux`). `unix.Stat_t` field widths differ between Linux and Darwin; access them through such a file.
3. **Cross-compile before claiming anything builds.** `GOOS=darwin GOARCH=arm64 go build ./... && GOOS=darwin GOARCH=amd64 go build ./...` must pass alongside the Linux build, and `rake` must gate on it. A build that was only run on Linux is not a working build; do not report it as one.
4. **No `/proc` without a Darwin path.** `/proc/self/*` is Linux-only; every use needs a Darwin equivalent (`/dev/fd`, `getfsstat`, `sysctl kern.argmax`) or must not exist.

## Setup

Run `rake` to build everything, then run rubocop and the suite. As a non-root user it exits non-zero, because the 15 `rlimit_nproc: 0` examples, which no Go binary can start under, fail by design.

- `dist/find` is qfind: `rake qfind:build` (`go build -trimpath -ldflags=-X=runtime.godebugDefault=asyncpreemptoff=1 -o dist/find ./cmd/qfind`; without the flag, async preemption's signals cut directory reads short on virtiofs).
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
