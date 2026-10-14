# qfind

IMPORTANT: DO NOT WRITE TO /tmp. EVER!
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
