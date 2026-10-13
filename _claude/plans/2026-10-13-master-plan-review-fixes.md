# Master Plan Review Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix all 18 issues the PR #1 review found in `_claude/plans/master_plan.md` and `spec/README.md`, so that every command, script and code block in the qfind plan does what its text says, and so that the CLAUDE.md the plan installs keeps the user's git rule.

**Architecture:** Each fix is a unified diff, made with `diff -u --label` against the text the previous fix leaves, and stored in git-ignored `.scratch/patches/`. Patches 01–11 chain on `_claude/plans/master_plan.md`; patch 12 changes `spec/README.md`. Each task writes a check in git-ignored `.scratch/fixcheck/`, runs it to see it fail, applies its patch with `git apply`, and runs it again to see it pass. The checks test the plan's own text: each one pulls a command, script or Go function out of the plan and runs it, against the GNU find oracle where behaviour matters.

**Tech Stack:** bash 5.3, GNU find 4.11.0 (`findutils-4.11.0/find/find`, which `dist/find` still hard-links), Go 1.27.2 (`GOTOOLCHAIN=local`, standard library only), Ruby with Bundler, RSpec 3.13, rubocop 1.92, git.

**Spec:** The PR #1 review in this session. Issue A was posted at https://github.com/gillisd/qfind/pull/1#issuecomment-6149639983, and every scored issue is in the Findings table below. The document being fixed is `_claude/plans/master_plan.md` at `2475eee`.

## Findings

Line numbers are at the PR head, `2475eee5127e734079ee07ab9b2460e6c0c541b2`. "plan" is `_claude/plans/master_plan.md`.

| Issue | Score | What is wrong | File and lines | Fixed by |
|---|---|---|---|---|
| A | 80 | Every `go vet ./...` gate loads `references/go/test`, because Go ignores `.gitignore`; `gofmt -l .` walks every clone | plan L207, L244-245, L407 and 11 more `go vet ./...` gates, L12077; cause `Rakefile` L155 | patch 01, Task 1 |
| B | 75 | 31 RSpec gates pipe to `tail -2`/`tail -3`, which shows only RSpec's random-order seed footer, never the summary | plan L237-239 and 30 more gates | patch 02, Task 2 |
| C | 75 | The final `rake` stops at the 15 accepted failures, so rubocop never runs, yet the gate expects it clean | plan L316, L368, L12068-12074 | patch 03, Task 2 |
| J | 75 | The wrappers force `TZ=UTC`, so the Europe/Berlin pass compares UTC with UTC | plan L332, L336, L11262 | patch 06, Task 4 |
| D | 65 | The installed CLAUDE.md, the Goal and the Spec line say "0 failures"; the plan's own end state is 15 failures, 547 pending | plan L376, L5, L27 | patch 05, Task 3 |
| H | 65 | `pipes.sh` reads the subshell's status (head's 0), not find's 141 | plan L3544-3547 | patch 08, Task 4 |
| K | 60 | `many.sh` never reads the `-fprint` file, which both finds write at once | plan L4426-4427 | patch 09, Task 4 |
| P | 55 | `many.sh` and `bytes.sh` sort `-print0` output with plain `sort`, which leaves NUL records in output order | plan L4426-4427, L9103-9104 | patch 09, Task 4 |
| E | 50 | The installed CLAUDE.md says "Do not provide any order guarantees" with no GNU qualifier | plan L361 | patch 05, Task 3 |
| F | 50 | Step 9 copies the plan from `/home/dev/.claude/plans/…`, and CLAUDE.md points at that copy instead of the committed plan | plan L210, L224, L364, L402-410 | patch 04, Task 3 |
| I | 50 | `-O2 -D opt` after the starting point is an unknown predicate | plan L11711 | patch 07, Task 4 |
| R | 50 | `recordInheritedFDs` records its own `/proc/self/fd` listing descriptor, so a directory fd that reuses the number leaks into `-exec` children | plan L7243-7248 | patch 11, Task 5 |
| N | 40 | The README says the audit covered all 19,214 examples; it covered the 18,671 that run | `spec/README.md` L35-37 | patch 12, Task 6 |
| Q | 40 | `fatalf` prints before it takes the exit lock, so concurrent fatal errors print more than once | plan L145, L1053-1058, L1066-1096 | patch 10, Task 5 |
| G | 35 | An unquoted `)` makes a Task 3 spot check a shell syntax error | plan L3530 | patch 07, Task 4 |
| M | 25 | Task 1 calls the already-committed Rakefile references an uncommitted change | plan L222, L288 | patch 04, Task 3 |
| O | 25 | "The RSpec suite is the only test bed" sits next to required `.scratch/diff` checks | plan L102, L360 | patch 05, Task 3 |
| L | 5 | The installed CLAUDE.md paraphrases away the user's own spec rules | plan L378-381 | patch 05, Task 3 |

Patch 05 also carries the `## Git` rule of the current `CLAUDE.md` ("Make every commit a clean, new commit. No amending and no force pushing.", added in `d438505` after the review) into the CLAUDE.md that Task 1 installs. Task 1 replaces the whole file, and without this it would drop the rule.

Patches verified: all 12 apply in order with `git apply --cached` to an index built from `HEAD`, and the result is byte-identical to the copies they were made from (`.scratch/patches/verify`). Each check was run on the text before and after its patch: it failed before and passed after. All 12 checks pass together on the final text. The fixed plan grows from 12,079 to 12,118 lines (121 insertions, 82 deletions); `spec/README.md` gains one line.

**Caveats found while generating the patches:**
- `git apply --check a.patch b.patch` checks every patch against the original file, so a chained patch fails it. `verify` applies the patches in order to a scratch index instead, and execution applies one patch per step.
- The planning-with-patches skill's `patch-*` scripts use `/tmp/patches`. This plan's workspace is `.scratch/patches/`, because scratch files never go under `/tmp` here.
- `pipes.sh` inherits the login shell's `LC_ALL=en_US.UTF-8`, so GNU find quotes `‘standard output’` and the plan's expected ASCII line never appears. This was not one of the review's issues; patch 08 sets `LC_ALL=C` there too.
- The `)` in the Task 3 spot check is meant for find (it tests the unmatched-`)` diagnostic), so patch 07 quotes it instead of deleting it.
- Patch 10 keeps `exit` as the entry point for its ten callers and moves its body into `exitClaimed`. Task 15's profiling hook runs "just before `os.Exit`", so its sentence now names `Run.exitClaimed`.
- `sed -i` drops the execute bit on this file system; if a check or helper stops being executable, `chmod +x` it.

## Global Constraints

- Change only `_claude/plans/master_plan.md` and `spec/README.md`. Patches, checks and their scratch output live in git-ignored `.scratch/` (`.scratch/patches/`, `.scratch/fixcheck/`). Nothing goes under `/tmp`.
- Start from a clean working tree on `dev` in which `_claude/plans/master_plan.md` and `spec/README.md` are still as they were at `2475eee5127e734079ee07ab9b2460e6c0c541b2`. Later commits that touch other files, such as this plan's own commit and the `CLAUDE.md` git rule, do not matter.
- Apply the patches in numeric order with `git apply`, one per step. Patches 01–11 chain: each was made against the file as the previous patch leaves it, so none applies out of order or twice. If `git apply` fails, stop and report; never edit a hunk by hand.
- The checks read `_claude/plans/master_plan.md` and `spec/README.md` by default. `PLAN=` and `README=` point them at other copies.
- `references/go` must exist (`rake references:setup` clones it), and `dist/find` must still be the GNU oracle, as it is at `2475eee`.
- Go checks build with go1.27.2 and the standard library only: no network, `go 1.27` in their `go.mod`, no Go 1.28 syntax.
- One commit per task, on `dev`, with a short imperative subject in the repository's style. Do not push before Task 6, and push only after the user confirms there, because pushing updates the open PR #1.

## Review Focus

Five conditions that the fixes could break and that no single finding names. Each is pinned by a check in the task that owns the code.

1. **RSpec's other summary shapes.** "1 example, 1 failure" and "0 examples, 0 failures, 1 error occurred outside of examples" must match the gates' grep too. (Task 2's summary check runs both.)
2. **qfind's own packages under `ignore ./references`.** `go vet ./...` must still vet `cmd/` and `internal/`. (Task 1's check plants a vet error in `internal/find` and requires vet to report it.)
3. **This machine's shell environment.** The login shell exports `LC_ALL=en_US.UTF-8` and leaves `TZ` unset. The wrappers must still default to UTC and keep `LC_ALL=C`. (Task 4's wrapper check runs under that environment.)
4. **A fatal error inside `fatalAt`'s commit.** That is a second fatal call on the same goroutine. The exit claim must not deadlock it; it must exit 1 with its own message. (Task 5's Go check, "nested" case.)
5. **`pipes.sh`'s `/dev/full` line.** It must keep its subshell, because a bare `exec` there would replace the script and skip the second binary. (Task 4's pipes check requires both binaries' sections and both `exit=1` lines.)

---

### Task 1: Keep the plan's Go gates out of `references/` (issue A)

**Files:**
- Modify: `_claude/plans/master_plan.md` — File Structure go.mod line (L207), Task 1 Step 2 (L241-248), Task 15 Step 7 (L12077), via patch 01
- Create (git-ignored): `.scratch/fixcheck/01-go-vet.sh`; `.scratch/patches/` and `.scratch/fixcheck/` contents if missing (Step 1)

**Interfaces:**
- Consumes: `_claude/plans/master_plan.md` and `spec/README.md` as they were at `2475eee`.
- Produces: the master plan's Task 1 Step 2 runs `go mod edit -ignore=./references` after `go mod edit -go=1.27`, and Task 15's last gate is `gofmt -l cmd internal; go vet ./...`. Patches 02–11 are made against this text.

- [ ] **Step 1: Confirm the starting point and the patch chain**

```bash
git status --short                                  # expect no output
git diff --quiet 2475eee5127e734079ee07ab9b2460e6c0c541b2 -- _claude/plans/master_plan.md spec/README.md && echo unchanged   # expect: unchanged
ls .scratch/patches/[0-9][0-9]-*.patch | wc -l      # expect 12
.scratch/patches/verify
```
Expected: `verify` prints `applied 01-go-vet-scope.patch` through `applied 12-readme-audit-scope.patch`, and nothing that says `FAILED` or `MISMATCH`.

If `.scratch/patches/` or `.scratch/fixcheck/` is missing, first write every block headed **Patch**, **Check** or **Helper** in this plan back to its file, byte for byte:
```bash
mkdir -p .scratch/patches .scratch/fixcheck
awk '/^\*\*(Patch|Check|Helper) `[^`]+`\*\*$/ { match($0, /`[^`]+`/); f = substr($0, RSTART + 1, RLENGTH - 2); if (f !~ /\//) f = ".scratch/patches/" f; next }
     f != "" && /^````[a-z]+$/ { w = 1; next }
     w && /^````$/ { w = 0; close(f); f = ""; next }
     w { print > f }' _claude/plans/2026-10-13-master-plan-review-fixes.md
chmod +x .scratch/fixcheck/*.sh .scratch/patches/verify
```

**Helper `.scratch/patches/verify`**

````zsh
#!/usr/bin/env zsh
# verify: apply every NN-*.patch in order to a throwaway index built from HEAD
# (git apply --cached, so the working tree is untouched), then, when the
# working copies the patches were made from exist, check the result matches.
emulate -L zsh
setopt extendedglob
P=/workspace/qfind.master/.scratch/patches
cd /workspace/qfind.master
export GIT_INDEX_FILE=$P/verify.index
rm -f $GIT_INDEX_FILE
git read-tree HEAD || exit 1
for p in $P/[0-9][0-9]-*.patch(N); do
  git apply --cached $p || { print -u2 "FAILED: ${p:t}"; rm -f $GIT_INDEX_FILE; exit 1 }
  print -u2 "applied ${p:t}"
done
ok=0
if [[ -f $P/master_plan.md.new ]]; then
  git show :_claude/plans/master_plan.md | cmp -s - $P/master_plan.md.new && print -u2 "plan: matches working copy" || { print -u2 "plan: MISMATCH"; ok=1 }
fi
if [[ -f $P/README.md.new ]]; then
  git show :spec/README.md | cmp -s - $P/README.md.new && print -u2 "README: matches working copy" || { print -u2 "README: MISMATCH"; ok=1 }
fi
rm -f $GIT_INDEX_FILE
exit $ok
````

- [ ] **Step 2: Write the failing check**

**Check `.scratch/fixcheck/01-go-vet.sh`**

````bash
#!/bin/bash
# Review issue A: the plan's go.mod must keep `go vet ./...` on qfind's own
# packages, away from the git-ignored clones in references/.
set -u
root=/workspace/qfind.master
plan=${PLAN:-$root/_claude/plans/master_plan.md}
m=$root/.scratch/fixcheck/01/mod
fail=0
rm -rf "$m" && mkdir -p "$m/cmd/qfind" "$m/internal/find" "$m/references"
cp -al "$root/references/go" "$m/references/go"
cat > "$m/cmd/qfind/main.go" <<'EOF'
package main

import "qfind/internal/find"

func main() { find.Main() }
EOF
cat > "$m/internal/find/main.go" <<'EOF'
package find

import "fmt"

// Main has a deliberate vet error. vet must report it, which proves the
// ignore directive did not hide qfind's own packages.
func Main() { fmt.Printf("%d\n", "not a number") }
EOF
# The go.mod commands of the plan's Task 1 Step 2. `go get` needs the network
# and changes nothing this check looks at, so it is left out.
awk '/^- \[ \] \*\*Step 2: Create the Go module\*\*/ { s = 1 }
     s && /^go mod / { print }
     s && /^- \[ \] \*\*Step 3:/ { exit }' "$plan" > "$m/gomod.sh"
echo "plan's go.mod commands:"; sed 's/^/  /' "$m/gomod.sh"
(cd "$m" && bash -eu gomod.sh) || { echo "FAIL: the go.mod commands did not run"; exit 1; }
vet=$(cd "$m" && go vet ./... 2>&1)
if grep -q 'found packages' <<<"$vet"; then
  echo "FAIL: go vet ./... loads references/go ($(grep -c 'found packages' <<<"$vet") 'found packages' errors)"
  fail=1
fi
if grep -q 'internal/find/main.go.*Printf format %d' <<<"$vet"; then
  echo "ok: go vet ./... still vets internal/find"
else
  echo "FAIL: go vet ./... did not report the planted error in internal/find"
  fail=1
fi
pkgs=$(cd "$m" && go list ./... 2>/dev/null | tr '\n' ' ')
if [ "$pkgs" = "qfind/cmd/qfind qfind/internal/find " ]; then
  echo "ok: go list ./... = $pkgs"
else
  echo "FAIL: go list ./... = ${pkgs:-<error>}"
  fail=1
fi
if grep -nE '^gofmt -l \.( |;|$)' "$plan"; then
  echo "FAIL: a gate runs gofmt over the whole tree, references/ and .scratch/ included"
  fail=1
fi
rm -rf "$m"
[ "$fail" -eq 0 ] && echo PASS || echo FAIL
exit "$fail"
````

- [ ] **Step 3: Run the check to see it fail**

Run: `.scratch/fixcheck/01-go-vet.sh` (about 20 seconds; it hard-links a copy of `references/go`)
Expected: exit 1, with these lines among the output:
```
FAIL: go vet ./... loads references/go (275 'found packages' errors)
FAIL: go vet ./... did not report the planted error in internal/find
FAIL: go list ./... = <error>
12077:gofmt -l . ; go vet ./...
FAIL: a gate runs gofmt over the whole tree, references/ and .scratch/ included
FAIL
```

- [ ] **Step 4: Apply patch 01**

Run: `git apply .scratch/patches/01-go-vet-scope.patch`

**Patch `01-go-vet-scope.patch`**

````diff
--- a/_claude/plans/master_plan.md
+++ b/_claude/plans/master_plan.md
@@ -204,7 +204,7 @@
 Build and helper files:
 
 ```
-go.mod / go.sum                     module qfind, go 1.27, fastwalk v1.0.14, x/sys v0.48.0; tool goyacc (x/tools v0.50.0)
+go.mod / go.sum                     module qfind, go 1.27, ignore ./references, fastwalk v1.0.14, x/sys v0.48.0; tool goyacc (x/tools v0.50.0)
 Rakefile                            + qfind:build (dist/find); upstream:build no longer links dist/find
 CLAUDE.md                           qfind task statement (Task 1)
 docs/superpowers/plans/2026-10-13-qfind.md   this plan, committed
@@ -243,9 +243,10 @@
 ```bash
 go mod init qfind
 go mod edit -go=1.27
+go mod edit -ignore=./references
 go get github.com/charlievieth/fastwalk@v1.0.14 golang.org/x/sys@v0.48.0
 ```
-Dependencies are pinned to releases at least two weeks old: x/sys v0.48.0 is from 2026-08-31, and Task 12's x/tools v0.50.0 is from 2026-09-08. The module proxy is reachable from this machine.
+The `ignore` directive (Go 1.25+) keeps `./...` out of `references/`, the git-ignored clones that `rake` puts inside this module. `references/go` has no `go.mod` at its root, so without the directive every `go vet ./...` gate would load `references/go/test`, Go's compiler test corpus, and fail on its mixed `package` clauses. Dependencies are pinned to releases at least two weeks old: x/sys v0.48.0 is from 2026-08-31, and Task 12's x/tools v0.50.0 is from 2026-09-08. The module proxy is reachable from this machine.
 
 - [ ] **Step 3: Write the stub entry points**
 
@@ -12074,6 +12075,6 @@
 - rubocop is clean, since the specs are unchanged.
 
 ```bash
-gofmt -l . ; go vet ./...
+gofmt -l cmd internal; go vet ./...
 git add internal cmd && git commit -m "qfind: deep trees via /proc/self/fd, race-free, benchmarked against GNU find"
 ```
````

- [ ] **Step 5: Run the check to see it pass**

Run: `.scratch/fixcheck/01-go-vet.sh`
Expected: exit 0, ending:
```
ok: go vet ./... still vets internal/find
ok: go list ./... = qfind/cmd/qfind qfind/internal/find
PASS
```

- [ ] **Step 6: Commit**

```bash
git add _claude/plans/master_plan.md
git commit -m "Keep the qfind plan's Go gates out of references/"
```

---

### Task 2: Make the plan's RSpec and rake gates show their summaries (issues B, C)

**Files:**
- Modify: `_claude/plans/master_plan.md` — the 31 `bundle exec rspec … | tail -N` gates (L237 to L12031), via patch 02; Task 1 Step 4's default task (L314-316), the installed CLAUDE.md's Setup line (L368) and Task 15 Step 7 (L12068-12074), via patch 03
- Create (git-ignored): `.scratch/fixcheck/02-summary.sh`, `.scratch/fixcheck/03-rake-order.sh`

**Interfaces:**
- Consumes: Task 1's text.
- Produces: every RSpec gate ends in `| grep -E '^[0-9]+ examples?, '`; the default task is `[:setup, "qfind:build", :rubocop, :spec]`; the final gate filters `rake` with `grep -E '^[0-9]+ examples?, |files? inspected'`.

- [ ] **Step 1: Write the failing summary check**

**Check `.scratch/fixcheck/02-summary.sh`**

````bash
#!/bin/bash
# Review issue B: every RSpec gate in the plan must show RSpec's summary
# line, which a `tail -2`/`tail -3` window misses under random ordering.
set -u
root=/workspace/qfind.master
plan=${PLAN:-$root/_claude/plans/master_plan.md}
d=$root/.scratch/fixcheck/02
fail=0
summary="grep -E '^[0-9]+ examples?, '"
tails=$(grep -c 'bundle exec rspec.*| *tail' "$plan")
greps=$(grep -cF "| $summary" "$plan")
echo "RSpec gates piped to tail: $tails; piped to the summary grep: $greps"
[ "$tails" -eq 0 ] || { echo "FAIL: $tails RSpec gates still cut the output with tail"; fail=1; }
[ "$greps" -eq 31 ] || { echo "FAIL: expected all 31 RSpec gates on the summary grep"; fail=1; }
# The grep must find exactly one summary line in every shape RSpec prints it.
rm -rf "$d" && mkdir -p "$d"
cat > "$d/one_spec.rb" <<'EOF'
RSpec.describe "one example" do
  it { expect(1).to eq(1) }
end
EOF
printf 'RSpec.describe "broken" do\n' > "$d/broken_spec.rb"
cd "$root"
expect_summary() { # label, regex the one summary line must match, command...
  local label=$1 want=$2 got
  shift 2
  got=$("$@" 2>&1 | eval "$summary")
  if [ "$(grep -c . <<<"$got")" -eq 1 ] && grep -qE "$want" <<<"$got"; then
    echo "ok: $label -> $got"
  else
    echo "FAIL: $label -> ${got:-<nothing>}"
    fail=1
  fi
}
expect_summary "passing file" '^[0-9]+ examples, 0 failures' bundle exec rspec spec/find/tests/true_spec.rb
expect_summary "failing file" '^[0-9]+ examples, [1-9][0-9]* failures?' env FIND=/bin/false bundle exec rspec spec/find/tests/true_spec.rb
expect_summary "one example" '^1 example, 0 failures$' bundle exec rspec "$d/one_spec.rb"
expect_summary "load error" '^0 examples, 0 failures, 1 error occurred outside of examples$' bundle exec rspec "$d/broken_spec.rb"
[ "$fail" -eq 0 ] && echo PASS || echo FAIL
exit "$fail"
````

- [ ] **Step 2: Run it to see it fail**

Run: `.scratch/fixcheck/02-summary.sh`
Expected: exit 1. The counts fail, while the four real RSpec runs already pass, which shows the pattern itself is right:
```
RSpec gates piped to tail: 31; piped to the summary grep: 0
FAIL: 31 RSpec gates still cut the output with tail
FAIL: expected all 31 RSpec gates on the summary grep
ok: passing file -> 49 examples, 0 failures
ok: failing file -> 49 examples, 25 failures
ok: one example -> 1 example, 0 failures
ok: load error -> 0 examples, 0 failures, 1 error occurred outside of examples
FAIL
```

- [ ] **Step 3: Apply patch 02**

Run: `git apply .scratch/patches/02-rspec-summary-line.patch`

**Patch `02-rspec-summary-line.patch`**

````diff
--- a/_claude/plans/master_plan.md
+++ b/_claude/plans/master_plan.md
@@ -234,7 +234,7 @@
 ```bash
 rm dist/find
 stat -c '%h' findutils-4.11.0/find/find          # expect: 1
-FIND=$PWD/findutils-4.11.0/find/find bundle exec rspec 2>&1 | tail -3
+FIND=$PWD/findutils-4.11.0/find/find bundle exec rspec 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 Expected: `19214 examples, 0 failures, 547 pending`.
 
@@ -321,7 +321,7 @@
 
 ```bash
 bundle exec rake qfind:build && ./dist/find; echo "exit=$?"
-bundle exec rspec 2>&1 | tail -3
+bundle exec rspec 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 Expected: `find: not implemented yet`, `exit=1`; the suite runs to completion with thousands of failures and `547 pending` or more. Record the failure count in the commit message.
 
@@ -438,7 +438,7 @@
 
 - [ ] **Step 1: Confirm the target specs fail**
 
-Run: `bundle exec rspec spec/find/options/help_spec.rb spec/find/options/version_spec.rb 2>&1 | tail -2`
+Run: `bundle exec rspec spec/find/options/help_spec.rb spec/find/options/version_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '`
 Expected: every example fails ("find: not implemented yet").
 
 - [ ] **Step 2: `internal/libc/errno.go` — glibc strerror text**
@@ -1353,8 +1353,8 @@
 
 ```bash
 bundle exec rake qfind:build
-bundle exec rspec spec/find/options/help_spec.rb spec/find/options/version_spec.rb 2>&1 | tail -2
-bundle exec rspec spec/find/options/debug_spec.rb spec/find/options/optimisation_spec.rb -e 'without a level' -e 'not a number' -e 'negative level' -e 'trailing the level' -e 'above 65535' -e 'out of the numeric range' -e 'separate argument' -e 'without a list' -e 'with the list ""' -e 'with the list ","' -e 'with help' 2>&1 | tail -2
+bundle exec rspec spec/find/options/help_spec.rb spec/find/options/version_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
+bundle exec rspec spec/find/options/debug_spec.rb spec/find/options/optimisation_spec.rb -e 'without a level' -e 'not a number' -e 'negative level' -e 'trailing the level' -e 'above 65535' -e 'out of the numeric range' -e 'separate argument' -e 'without a list' -e 'with the list ""' -e 'with the list ","' -e 'with help' 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 Expected:
 - help/version: 0 failures, except the placement-warning example (`find . -name x --help` on a terminal) and the `-foo --help` unknown-predicate examples, which need Task 3.
@@ -1425,7 +1425,7 @@
 - [ ] **Step 1: Record the failing gates**
 
 ```bash
-bundle exec rspec spec/find/operators spec/find/expressions_spec.rb spec/find/tests/true_spec.rb spec/find/tests/false_spec.rb spec/find/actions/print_spec.rb spec/find/actions/print0_spec.rb spec/find/positional_options/warn_spec.rb spec/find/global_options/placement_spec.rb 2>&1 | tail -2
+bundle exec rspec spec/find/operators spec/find/expressions_spec.rb spec/find/tests/true_spec.rb spec/find/tests/false_spec.rb spec/find/actions/print_spec.rb spec/find/actions/print0_spec.rb spec/find/positional_options/warn_spec.rb spec/find/global_options/placement_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 Expected: nearly every example fails.
 
@@ -3518,7 +3518,7 @@
 
 ```bash
 bundle exec rake qfind:build
-bundle exec rspec spec/find/operators spec/find/expressions_spec.rb spec/find/positional_options/warn_spec.rb spec/find/global_options/placement_spec.rb spec/find/options spec/find/tests/true_spec.rb spec/find/tests/false_spec.rb spec/find/actions/print_spec.rb spec/find/actions/print0_spec.rb --format progress 2>&1 | tail -3
+bundle exec rspec spec/find/operators spec/find/expressions_spec.rb spec/find/positional_options/warn_spec.rb spec/find/global_options/placement_spec.rb spec/find/options spec/find/tests/true_spec.rb spec/find/tests/false_spec.rb spec/find/actions/print_spec.rb spec/find/actions/print0_spec.rb --format progress 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 Expected:
 - Every "a rejected command line" example passes.
@@ -3589,7 +3589,7 @@
 - [ ] **Step 1: Record the failing gates**
 
 ```bash
-bundle exec rspec spec/find/traversal_spec.rb spec/find/starting_points_spec.rb spec/find/global_options spec/find/options/symbolic_links_spec.rb spec/find/positional_options/follow_spec.rb spec/find/actions/prune_spec.rb spec/find/actions/quit_spec.rb spec/find/operators spec/find/tests/true_spec.rb spec/find/tests/false_spec.rb --format progress 2>&1 | tail -2
+bundle exec rspec spec/find/traversal_spec.rb spec/find/starting_points_spec.rb spec/find/global_options spec/find/options/symbolic_links_spec.rb spec/find/positional_options/follow_spec.rb spec/find/actions/prune_spec.rb spec/find/actions/quit_spec.rb spec/find/operators spec/find/tests/true_spec.rb spec/find/tests/false_spec.rb --format progress 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 
 - [ ] **Step 2: `paths.go` — fts path spelling over fastwalk paths**
@@ -4403,7 +4403,7 @@
 
 ```bash
 bundle exec rake qfind:build
-bundle exec rspec spec/find/traversal_spec.rb spec/find/starting_points_spec.rb spec/find/global_options spec/find/options/symbolic_links_spec.rb spec/find/positional_options/follow_spec.rb spec/find/actions/prune_spec.rb spec/find/actions/quit_spec.rb spec/find/operators spec/find/tests/true_spec.rb spec/find/tests/false_spec.rb --format progress 2>&1 | tail -2
+bundle exec rspec spec/find/traversal_spec.rb spec/find/starting_points_spec.rb spec/find/global_options spec/find/options/symbolic_links_spec.rb spec/find/positional_options/follow_spec.rb spec/find/actions/prune_spec.rb spec/find/actions/quit_spec.rb spec/find/operators spec/find/tests/true_spec.rb spec/find/tests/false_spec.rb --format progress 2>&1 | grep -E '^[0-9]+ examples?, '
 ruby .scratch/failures.rb .scratch/run.json
 ```
 Expected: every failure names a primary owned by a later task (`-name`, `-type`, `-printf`, `-exec`, `-execdir`, `-ls`, `-delete`, `-regex`, `-newer`, `-mtime`, `-size`, `-perm`, …). Check each one in `rspec --format documentation`. The traversal-only examples must all pass, including:
@@ -4473,7 +4473,7 @@
 - [ ] **Step 1: Failing gates**
 
 ```bash
-bundle exec rspec spec/find/tests/{name,iname,path,wholename,ipath,iwholename,lname,ilname}_spec.rb 2>&1 | tail -2
+bundle exec rspec spec/find/tests/{name,iname,path,wholename,ipath,iwholename,lname,ilname}_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 
 - [ ] **Step 2: `internal/fnmatch/fnmatch.go` — port of `FCT` (fnmatch_loop.c)**
@@ -5012,13 +5012,13 @@
 
 ```bash
 bundle exec rake qfind:build
-bundle exec rspec spec/find/tests/{name,iname,path,wholename,ipath,iwholename,lname,ilname}_spec.rb spec/find/options/optimisation_spec.rb spec/find/operators spec/find/expressions_spec.rb 2>&1 | tail -2
+bundle exec rspec spec/find/tests/{name,iname,path,wholename,ipath,iwholename,lname,ilname}_spec.rb spec/find/options/optimisation_spec.rb spec/find/operators spec/find/expressions_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 Expected:
 - The 8 pattern files pass, except examples that also use `-printf`, `-exec` or `-type` (later tasks).
 - The `-O` "promoting cheap predicate -name file" examples now pass.
 
-Then run the whole suite to confirm no regressions in earlier gates: `bundle exec rspec --format progress | tail -3`.
+Then run the whole suite to confirm no regressions in earlier gates: `bundle exec rspec --format progress | grep -E '^[0-9]+ examples?, '`.
 
 - [ ] **Step 7: Commit**
 
@@ -5047,7 +5047,7 @@
 - [ ] **Step 1: Failing gates**
 
 ```bash
-bundle exec rspec spec/find/tests/type_spec.rb spec/find/tests/xtype_spec.rb 2>&1 | tail -2
+bundle exec rspec spec/find/tests/type_spec.rb spec/find/tests/xtype_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 
 - [ ] **Step 2: `pred_types.go` — `insert_type`, `pred_type`, `pred_xtype` (parser.c 2533, pred.c 925/1080)**
@@ -5242,7 +5242,7 @@
 ```
 ```bash
 bundle exec rake qfind:build
-bundle exec rspec spec/find/tests/type_spec.rb spec/find/tests/xtype_spec.rb spec/find/options/optimisation_spec.rb spec/find/global_options spec/find/options/symbolic_links_spec.rb 2>&1 | tail -2
+bundle exec rspec spec/find/tests/type_spec.rb spec/find/tests/xtype_spec.rb spec/find/options/optimisation_spec.rb spec/find/global_options spec/find/options/symbolic_links_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 Expected:
 - 0 failures in `type_spec.rb`.
@@ -5292,7 +5292,7 @@
 - [ ] **Step 1: Failing gates**
 
 ```bash
-bundle exec rspec spec/find/actions/{printf,printf_escapes,printf_names,printf_attributes,printf_times,fprint,fprint0,fprintf}_spec.rb 2>&1 | tail -2
+bundle exec rspec spec/find/actions/{printf,printf_escapes,printf_names,printf_attributes,printf_times,fprint,fprint0,fprintf}_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 
 - [ ] **Step 2: `sharefile.go` and `output.go` — output files shared by (dev, ino)**
@@ -6678,7 +6678,7 @@
 
 ```bash
 bundle exec rake qfind:build
-bundle exec rspec spec/find/actions/{printf,printf_escapes,printf_names,printf_attributes,printf_times,fprint,fprint0,fprintf,print,print0}_spec.rb spec/find/unusual_filenames_spec.rb spec/find/tests/size_spec.rb spec/find/tests/xtype_spec.rb spec/find/options/symbolic_links_spec.rb 2>&1 | tail -2
+bundle exec rspec spec/find/actions/{printf,printf_escapes,printf_names,printf_attributes,printf_times,fprint,fprint0,fprintf,print,print0}_spec.rb spec/find/unusual_filenames_spec.rb spec/find/tests/size_spec.rb spec/find/tests/xtype_spec.rb spec/find/options/symbolic_links_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 Expected: 0 failures except examples that use `-ls`/`-fls` (Task 9), `-exec` (Task 8), `-size` (Task 11) or `-perm` (Task 13). The `%Z` example passes when SELinux is off (it is skipped otherwise).
 
@@ -6711,7 +6711,7 @@
 - [ ] **Step 1: Failing gates**
 
 ```bash
-bundle exec rspec spec/find/actions/{exec,execdir,ok,okdir}_spec.rb 2>&1 | tail -2
+bundle exec rspec spec/find/actions/{exec,execdir,ok,okdir}_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 
 - [ ] **Step 2: `buildcmd.go` — arg-size accounting (lib/buildcmd.c)**
@@ -7323,7 +7323,7 @@
 
 ```bash
 bundle exec rake qfind:build
-bundle exec rspec spec/find/actions/{exec,execdir,ok,okdir,quit}_spec.rb spec/find/options/debug_spec.rb spec/find/global_options/ignore_readdir_race_spec.rb spec/find/actions/printf_escapes_spec.rb spec/find/tests/links_spec.rb spec/find/tests/perm_spec.rb 2>&1 | tail -3
+bundle exec rspec spec/find/actions/{exec,execdir,ok,okdir,quit}_spec.rb spec/find/options/debug_spec.rb spec/find/global_options/ignore_readdir_race_spec.rb spec/find/actions/printf_escapes_spec.rb spec/find/tests/links_spec.rb spec/find/tests/perm_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 Expected:
 - 0 failures, except the 15 `rlimit_nproc: 0` examples (infeasible; verify each failing description contains "no more processes").
@@ -7352,7 +7352,7 @@
 - [ ] **Step 1: Failing gates**
 
 ```bash
-bundle exec rspec spec/find/actions/{ls,fls,delete}_spec.rb 2>&1 | tail -2
+bundle exec rspec spec/find/actions/{ls,fls,delete}_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 
 - [ ] **Step 2: `ls.go` — `list_file` with static, growing field widths**
@@ -7624,7 +7624,7 @@
 
 ```bash
 bundle exec rake qfind:build
-bundle exec rspec spec/find/actions/{ls,fls,delete}_spec.rb spec/find/global_options/{depth,ignore_readdir_race}_spec.rb spec/find/actions/print_spec.rb 2>&1 | tail -2
+bundle exec rspec spec/find/actions/{ls,fls,delete}_spec.rb spec/find/global_options/{depth,ignore_readdir_race}_spec.rb spec/find/actions/print_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 Expected: 0 failures. In particular:
 - the post-order `-delete` chain;
@@ -7665,7 +7665,7 @@
 - [ ] **Step 1: Failing gates**
 
 ```bash
-bundle exec rspec spec/find/tests/regex_spec.rb spec/find/tests/iregex_spec.rb spec/find/positional_options/regextype_spec.rb 2>&1 | tail -2
+bundle exec rspec spec/find/tests/regex_spec.rb spec/find/tests/iregex_spec.rb spec/find/positional_options/regextype_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 
 - [ ] **Step 2: `syntax.go` and `errors.go`**
@@ -9081,7 +9081,7 @@
 
 ```bash
 bundle exec rake qfind:build
-bundle exec rspec spec/find/tests/regex_spec.rb spec/find/tests/iregex_spec.rb spec/find/positional_options/regextype_spec.rb 2>&1 | tail -2
+bundle exec rspec spec/find/tests/regex_spec.rb spec/find/tests/iregex_spec.rb spec/find/positional_options/regextype_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 Expected: 0 failures across all 1,692 examples.
 
@@ -9140,7 +9140,7 @@
 - [ ] **Step 1: Failing gates**
 
 ```bash
-bundle exec rspec spec/find/tests/{size,links,inum,uid,gid,user,group,nouser,nogroup,samefile,empty,numeric_arguments}_spec.rb 2>&1 | tail -2
+bundle exec rspec spec/find/tests/{size,links,inum,uid,gid,user,group,nouser,nogroup,samefile,empty,numeric_arguments}_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 
 - [ ] **Step 2: `internal/libc/strtol.go`**
@@ -9599,7 +9599,7 @@
 ```
 ```bash
 bundle exec rake qfind:build
-bundle exec rspec spec/find/tests/{size,links,inum,uid,gid,user,group,nouser,nogroup,samefile,empty,numeric_arguments}_spec.rb spec/find/options/debug_spec.rb spec/find/options/optimisation_spec.rb 2>&1 | tail -2
+bundle exec rspec spec/find/tests/{size,links,inum,uid,gid,user,group,nouser,nogroup,samefile,empty,numeric_arguments}_spec.rb spec/find/options/debug_spec.rb spec/find/options/optimisation_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 Expected:
 - 0 failures, except the `-mmin`/`-amin` contexts of `numeric_arguments_spec.rb`, which need Task 12.
@@ -9641,7 +9641,7 @@
 - [ ] **Step 1: Failing gates**
 
 ```bash
-bundle exec rspec spec/find/tests/{amin,atime,cmin,ctime,mmin,mtime,used,newer,anewer,cnewer,newerxy,numeric_arguments}_spec.rb spec/find/positional_options/daystart_spec.rb spec/find/positional_options/follow_spec.rb 2>&1 | tail -2
+bundle exec rspec spec/find/tests/{amin,atime,cmin,ctime,mmin,mtime,used,newer,anewer,cnewer,newerxy,numeric_arguments}_spec.rb spec/find/positional_options/daystart_spec.rb spec/find/positional_options/follow_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 
 - [ ] **Step 2: `internal/libc/strtod.go` — glibc strtod in the C locale, gnulib xstrtod**
@@ -11252,7 +11252,7 @@
 `parseEntryNewerXY = parserEntry{argSpecialParse, "newerXY", parseNewerXY, predNewerXY, fnNewerXY}`.
 ```bash
 bundle exec rake qfind:build
-bundle exec rspec spec/find/tests/{amin,atime,cmin,ctime,mmin,mtime,used,newer,anewer,cnewer,newerxy,numeric_arguments}_spec.rb spec/find/positional_options/daystart_spec.rb spec/find/positional_options/follow_spec.rb spec/find/options/symbolic_links_spec.rb 2>&1 | tail -2
+bundle exec rspec spec/find/tests/{amin,atime,cmin,ctime,mmin,mtime,used,newer,anewer,cnewer,newerxy,numeric_arguments}_spec.rb spec/find/positional_options/daystart_spec.rb spec/find/positional_options/follow_spec.rb spec/find/options/symbolic_links_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 Expected: 0 failures. This includes:
 - `-newermt` with "3 days ago", `@1600000000.123456789`, `2020-01-01T00:00:00Z` and the `%Y-%m-%d %H:%M:%S.%N %z` form;
@@ -11297,7 +11297,7 @@
 - [ ] **Step 1: Failing gates**
 
 ```bash
-bundle exec rspec spec/find/tests/{perm,readable,writable,executable,fstype,context}_spec.rb 2>&1 | tail -2
+bundle exec rspec spec/find/tests/{perm,readable,writable,executable,fstype,context}_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 
 - [ ] **Step 2: `internal/modechange/modechange.go`**
@@ -11708,7 +11708,7 @@
 ```
 ```bash
 bundle exec rake qfind:build
-bundle exec rspec spec/find/tests/{perm,readable,writable,executable,fstype,context}_spec.rb spec/find/options/optimisation_spec.rb spec/find/options/version_spec.rb 2>&1 | tail -2
+bundle exec rspec spec/find/tests/{perm,readable,writable,executable,fstype,context}_spec.rb spec/find/options/optimisation_spec.rb spec/find/options/version_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
 ./dist/find / -maxdepth 0 -O2 -D opt -fstype nosuchfs 2>&1 | grep 'can never succeed'
 ```
 Expected: 0 failures. The last command prints `-fstype nosuchfs can never succeed, substituting -false`.
@@ -11739,7 +11739,7 @@
 
 ```bash
 bundle exec rake qfind:build
-bundle exec rspec --format json --out .scratch/run.json --format progress 2>&1 | tail -3
+bundle exec rspec --format json --out .scratch/run.json --format progress 2>&1 | grep -E '^[0-9]+ examples?, '
 ruby .scratch/failures.rb .scratch/run.json
 ```
 
@@ -12021,7 +12021,7 @@
 
 ```bash
 bundle exec rake qfind:build && .scratch/diff/deep.sh
-bundle exec rspec --format progress 2>&1 | tail -2
+bundle exec rspec --format progress 2>&1 | grep -E '^[0-9]+ examples?, '
 ```
 Expected: `same:` for all five commands, and the suite is unchanged: 15 failures, all `rlimit_nproc`, and 547 pending.
 
@@ -12029,7 +12029,7 @@
 
 ```bash
 go build -race -trimpath -o .scratch/qfind-race ./cmd/qfind
-FIND=$PWD/.scratch/qfind-race bundle exec rspec --format json --out .scratch/race.json --format progress 2>&1 | tail -3
+FIND=$PWD/.scratch/qfind-race bundle exec rspec --format json --out .scratch/race.json --format progress 2>&1 | grep -E '^[0-9]+ examples?, '
 ruby -rjson -e 'puts JSON.parse(File.read(".scratch/race.json"))["examples"].count { it.dig("exception", "message").to_s.include?("DATA RACE") }'
 ```
 Expected: `0`. A race-enabled binary prints `WARNING: DATA RACE` on stderr and exits 66, so a race fails the example it occurs in, and its message carries the report. Expect a few extra failures that are not races: examples near resource limits (`rlimit_nofile`, `rlimit_stack`), where the race runtime's own overhead matters. Read the message of each. Fix every race by putting the shared state behind the lock its comment names.
````

- [ ] **Step 4: Run the summary check to see it pass**

Run: `.scratch/fixcheck/02-summary.sh`
Expected: exit 0; `RSpec gates piped to tail: 0; piped to the summary grep: 31`, the same four `ok:` lines, then `PASS`.

- [ ] **Step 5: Write the failing rake-order check**

**Check `.scratch/fixcheck/03-rake-order.sh`**

````bash
#!/bin/bash
# Review issue C: with the suite failing (as it always does as uid 1000), the
# plan's default rake task must still run rubocop, and the final gate's
# filter must show both rubocop's and RSpec's summary lines.
set -u
root=/workspace/qfind.master
plan=${PLAN:-$root/_claude/plans/master_plan.md}
d=$root/.scratch/fixcheck/03
default=$(grep -m1 '^task default:' "$plan")
filter=$(grep -m1 '^bundle exec rake 2>&1 |' "$plan" | sed -E 's/^bundle exec rake 2>&1 \| //; s/ +#.*$//')
echo "plan's default task: $default"
echo "plan's final-gate filter: $filter"
rm -rf "$d" && mkdir -p "$d/spec"
cat > "$d/Rakefile" <<EOF
require "rspec/core/rake_task"
require "rubocop/rake_task"
RSpec::Core::RakeTask.new
RuboCop::RakeTask.new
task :setup
namespace(:qfind) { task :build }
$default
EOF
cat > "$d/spec/accepted_failure_spec.rb" <<'EOF'
RSpec.describe "an accepted failure" do
  it { expect(1).to eq(2) }
end
EOF
printf 'AllCops:\n  DisabledByDefault: true\n  SuggestExtensions: false\n' > "$d/.rubocop.yml"
out=$(cd "$d" && BUNDLE_GEMFILE=$root/Gemfile bundle exec rake 2>&1 | eval "$filter")
echo "final gate shows:"; sed 's/^/  /' <<<"$out"
fail=0
grep -qE 'files? inspected, no offenses detected' <<<"$out" || { echo "FAIL: rubocop's summary is missing (rubocop never ran, or the filter hid it)"; fail=1; }
grep -qE '^1 example, 1 failure' <<<"$out" || { echo "FAIL: RSpec's summary is missing"; fail=1; }
[ "$fail" -eq 0 ] && echo PASS || echo FAIL
exit "$fail"
````

- [ ] **Step 6: Run it to see it fail**

Run: `.scratch/fixcheck/03-rake-order.sh`
Expected: exit 1. With `:spec` before `:rubocop`, the failing spec aborts `rake` before rubocop, and `tail -5` shows only the abort:
```
plan's default task: task default: [:setup, "qfind:build", :spec, :rubocop]
plan's final-gate filter: tail -5
final gate shows:
  Failed examples:

  rspec ./spec/accepted_failure_spec.rb:2 # an accepted failure is expected to eq 2

  /usr/bin/ruby -I… …/exe/rspec --pattern spec/\*\*\{,/\*/\*\*\}/\*_spec.rb failed
FAIL: rubocop's summary is missing (rubocop never ran, or the filter hid it)
FAIL: RSpec's summary is missing
FAIL
```

- [ ] **Step 7: Apply patch 03**

Run: `git apply .scratch/patches/03-rubocop-before-suite.patch`

**Patch `03-rubocop-before-suite.patch`**

````diff
--- a/_claude/plans/master_plan.md
+++ b/_claude/plans/master_plan.md
@@ -312,9 +312,9 @@
   task build: binary
 end
 ```
-Replace the default task:
+Replace the default task. rubocop runs before the suite: as uid 1000 the suite always ends with the 15 accepted `rlimit_nproc: 0` failures (Global Constraints), and RSpec's rake task aborts `rake` on any failure, so a `:rubocop` after `:spec` would never run.
 ```ruby
-task default: [:setup, "qfind:build", :spec, :rubocop]
+task default: [:setup, "qfind:build", :rubocop, :spec]
 ```
 
 - [ ] **Step 5: Build and confirm the suite runs against the stub**
@@ -366,7 +366,7 @@
 
 ## Setup
 
-Run `rake` to build everything and run the suite.
+Run `rake` to build everything, then run rubocop and the suite. As a non-root user it exits non-zero, because the 15 `rlimit_nproc: 0` examples, which no Go binary can start under, fail by design.
 
 - `dist/find` is qfind: `rake qfind:build` (`go build -trimpath -o dist/find ./cmd/qfind`).
 - GNU find 4.11.0 is built from source as the reference oracle at `findutils-4.11.0/find/find` (`rake upstream:build`). Run the suite against it with `FIND=$PWD/findutils-4.11.0/find/find bundle exec rspec`.
@@ -12067,12 +12067,12 @@
 - [ ] **Step 7: Final verification and commit**
 
 ```bash
-bundle exec rake 2>&1 | tail -5     # setup, qfind:build, spec, rubocop
+bundle exec rake 2>&1 | grep -E '^[0-9]+ examples?, |files? inspected'     # setup, qfind:build, rubocop, spec
 git status --short
 ```
 Expected:
-- `rake` reports `19214 examples, 15 failures, 547 pending`, and the 15 failures are the `rlimit_nproc: 0` examples listed in Global Constraints;
-- rubocop is clean, since the specs are unchanged.
+- rubocop, which runs first, reports `no offenses detected`, since the specs are unchanged;
+- the suite reports `19214 examples, 15 failures, 547 pending`, and the 15 failures are the `rlimit_nproc: 0` examples listed in Global Constraints. They make `rake` itself exit non-zero.
 
 ```bash
 gofmt -l cmd internal; go vet ./...
````

- [ ] **Step 8: Run the rake-order check to see it pass**

Run: `.scratch/fixcheck/03-rake-order.sh`
Expected: exit 0:
```
plan's default task: task default: [:setup, "qfind:build", :rubocop, :spec]
plan's final-gate filter: grep -E '^[0-9]+ examples?, |files? inspected'
final gate shows:
  2 files inspected, no offenses detected
  1 example, 1 failure
PASS
```

- [ ] **Step 9: Commit**

```bash
git add _claude/plans/master_plan.md
git commit -m "Make the qfind plan's RSpec and rake gates show their summaries"
```

---

### Task 3: Make the plan's Task 1 and its CLAUDE.md match the repo and the plan (issues F, M, D, E, O, L)

**Files:**
- Modify: `_claude/plans/master_plan.md` — File Structure (L210), Task 1 Files (L222, L224), Step 4 (L288), the installed CLAUDE.md's plan pointer (L364) and Step 9 (L402-410), via patch 04; Goal (L5), Spec (L27), Global Constraints (L102) and the installed CLAUDE.md (L360-361, L376-382), via patch 05
- Create (git-ignored): `.scratch/fixcheck/04-plan-location.sh`, `.scratch/fixcheck/05-task-statement.sh`

**Interfaces:**
- Consumes: Task 2's text.
- Produces: the installed CLAUDE.md names `_claude/plans/master_plan.md` as the plan; its done criterion reads "15 failures, 547 pending" for a non-root user; it carries, word for word, the user's original spec rules 1–4, the shared_examples line and the current `CLAUDE.md`'s `## Git` rule. Task 1 Step 9 commits `CLAUDE.md Rakefile go.mod go.sum cmd internal` and copies nothing.

- [ ] **Step 1: Write the failing plan-location check**

**Check `.scratch/fixcheck/04-plan-location.sh`**

````bash
#!/bin/bash
# Review issues F and M: Task 1 must match what this PR already committed.
# The plan lives at _claude/plans/master_plan.md (no copy from a home-directory
# path), and both references are already in the Rakefile.
set -u
root=/workspace/qfind.master
plan=${PLAN:-$root/_claude/plans/master_plan.md}
fail=0
if grep -n 'docs/superpowers/plans\|/home/dev/\.claude/plans/' "$plan"; then
  echo "FAIL: the plan still copies itself from outside the repo, or points at that copy"
  fail=1
fi
claude=$(awk '/^- \[ \] \*\*Step 7: Replace `CLAUDE.md`\*\*/ { s = 1; next }
              s && /^```markdown$/ { f = 1; next }
              f && /^```$/ { exit }
              f { print }' "$plan")
ptr=$(grep -o 'The implementation plan is `[^`]*`' <<<"$claude" | sed 's/.*`\(.*\)`/\1/')
if [ -n "$ptr" ] && git -C "$root" ls-files --error-unmatch "$ptr" >/dev/null 2>&1; then
  echo "ok: the installed CLAUDE.md points at the committed plan, $ptr"
else
  echo "FAIL: the installed CLAUDE.md points at '${ptr:-<nothing>}', which is not a committed file"
  fail=1
fi
if grep -n 'The working tree already has one Rakefile change' "$plan"; then
  echo "FAIL: Task 1 still calls the committed Rakefile references an uncommitted change"
  fail=1
fi
for repo in golang/go charlievieth/fastwalk; do
  grep -q "\"$repo\"" "$root/Rakefile" || { echo "FAIL: the Rakefile lacks $repo"; fail=1; }
done
[ "$fail" -eq 0 ] && echo PASS || echo FAIL
exit "$fail"
````

- [ ] **Step 2: Run it to see it fail**

Run: `.scratch/fixcheck/04-plan-location.sh`
Expected: exit 1:
```
210:docs/superpowers/plans/2026-10-13-qfind.md   this plan, committed
224:- Create: `docs/superpowers/plans/2026-10-13-qfind.md` (copy of this plan)
365:The implementation plan is `docs/superpowers/plans/2026-10-13-qfind.md`.
406:mkdir -p docs/superpowers/plans
407:cp /home/dev/.claude/plans/reimplement-all-of-gnu-composed-pony.md docs/superpowers/plans/2026-10-13-qfind.md
409:git add CLAUDE.md Rakefile go.mod go.sum cmd internal docs/superpowers/plans/2026-10-13-qfind.md
FAIL: the plan still copies itself from outside the repo, or points at that copy
FAIL: the installed CLAUDE.md points at 'docs/superpowers/plans/2026-10-13-qfind.md', which is not a committed file
289:The working tree already has one Rakefile change from the user: `charlievieth/fastwalk` was added to the `references` repos. Keep it, and commit it with this task.
FAIL: Task 1 still calls the committed Rakefile references an uncommitted change
FAIL
```

- [ ] **Step 3: Apply patch 04**

Run: `git apply .scratch/patches/04-plan-already-committed.patch`

**Patch `04-plan-already-committed.patch`**

````diff
--- a/_claude/plans/master_plan.md
+++ b/_claude/plans/master_plan.md
@@ -207,7 +207,7 @@
 go.mod / go.sum                     module qfind, go 1.27, ignore ./references, fastwalk v1.0.14, x/sys v0.48.0; tool goyacc (x/tools v0.50.0)
 Rakefile                            + qfind:build (dist/find); upstream:build no longer links dist/find
 CLAUDE.md                           qfind task statement (Task 1)
-docs/superpowers/plans/2026-10-13-qfind.md   this plan, committed
+_claude/plans/master_plan.md        this plan (already committed)
 .scratch/gnufind, .scratch/qf       (git-ignored) run the oracle / qfind with the suite's environment
 .scratch/failures.rb                (git-ignored) failures-per-file summary from `rspec --format json`
 .scratch/diff/*.sh                  (git-ignored) differential checks of the Review Focus
@@ -219,9 +219,8 @@
 
 **Files:**
 - Modify: `CLAUDE.md` (replace whole file)
-- Modify: `Rakefile` (keep the user's `charlievieth/fastwalk` reference; add `qfind` namespace; stop linking GNU find into `dist/`)
+- Modify: `Rakefile` (keep the user's committed `golang/go` and `charlievieth/fastwalk` references; add `qfind` namespace; stop linking GNU find into `dist/`)
 - Create: `go.mod`, `go.sum`, `cmd/qfind/main.go`, `internal/find/main.go`
-- Create: `docs/superpowers/plans/2026-10-13-qfind.md` (copy of this plan)
 - Create (git-ignored): `.scratch/gnufind`, `.scratch/qf`, `.scratch/failures.rb`
 - Memory: `/home/dev/.claude/projects/-workspace-qfind-master/memory/fastwalk-unmodified.md` + `MEMORY.md` line
 
@@ -286,7 +285,7 @@
 
 - [ ] **Step 4: Wire the build into the Rakefile**
 
-The working tree already has one Rakefile change from the user: `charlievieth/fastwalk` was added to the `references` repos. Keep it, and commit it with this task.
+The user has already committed two Rakefile changes: `golang/go` and `charlievieth/fastwalk` are in the `references` repos. Keep both.
 
 Make these changes in `namespace :upstream`:
 - delete the `directory archive.dist_dir` and `file archive.dist_binname` tasks;
@@ -362,7 +361,7 @@
 - This is a streaming problem, so order does not matter (GNU find does not guarantee it either). Do not provide any order guarantees.
 - fastwalk is deliberate: fzf uses it, and fzf is one of the gold-standard CLI tools of the last decade. Use it unmodified and adapt qfind to its model rather than forking it.
 
-The implementation plan is `docs/superpowers/plans/2026-10-13-qfind.md`.
+The implementation plan is `_claude/plans/master_plan.md`.
 
 ## Setup
 
@@ -400,13 +399,12 @@
 ```
 Append to `MEMORY.md`: `- [fastwalk unmodified](fastwalk-unmodified.md) — qfind uses fastwalk as-is like fzf; adapt to its model, don't fork it`
 
-- [ ] **Step 9: Commit the plan copy and scaffolding**
+- [ ] **Step 9: Commit the scaffolding**
 
+The plan is already committed at `_claude/plans/master_plan.md`, so there is nothing to copy.
 ```bash
-mkdir -p docs/superpowers/plans
-cp /home/dev/.claude/plans/reimplement-all-of-gnu-composed-pony.md docs/superpowers/plans/2026-10-13-qfind.md
 gofmt -l cmd internal; go vet ./...
-git add CLAUDE.md Rakefile go.mod go.sum cmd internal docs/superpowers/plans/2026-10-13-qfind.md
+git add CLAUDE.md Rakefile go.mod go.sum cmd internal
 git commit -m "Scaffold qfind: Go module on fastwalk, dist/find build, task statement"
 ```
 
````

- [ ] **Step 4: Run the plan-location check to see it pass**

Run: `.scratch/fixcheck/04-plan-location.sh`
Expected: exit 0: `ok: the installed CLAUDE.md points at the committed plan, _claude/plans/master_plan.md`, then `PASS`.

- [ ] **Step 5: Write the failing task-statement check**

**Check `.scratch/fixcheck/05-task-statement.sh`**

````bash
#!/bin/bash
# Review issues D, E, O and L: the CLAUDE.md that Task 1 installs, and the
# plan's own Goal and Spec lines, must agree with the plan's accepted end
# state, and the installed CLAUDE.md must keep the user's original spec rules
# and the git rule of the current CLAUDE.md.
set -u
root=/workspace/qfind.master
plan=${PLAN:-$root/_claude/plans/master_plan.md}
fail=0
claude=$(awk '/^- \[ \] \*\*Step 7: Replace `CLAUDE.md`\*\*/ { s = 1; next }
              s && /^```markdown$/ { f = 1; next }
              f && /^```$/ { exit }
              f { print }' "$plan")
goal=$(grep -m1 '^\*\*Goal:\*\*' "$plan")
spec=$(grep -m1 '^\*\*Spec:\*\*' "$plan")
testbed=$(grep -m1 '^- \*\*Do not write any tests in Go\.\*\*' "$plan")
has() { # label, text, fixed string the text must contain
  if grep -qF -- "$3" <<<"$2"; then echo "ok: $1"; else echo "FAIL: $1"; fail=1; fi
}
lacks() { # label, text, fixed string the text must not contain
  if grep -qF -- "$3" <<<"$2"; then echo "FAIL: $1"; fail=1; else echo "ok: $1"; fi
}
has "D: installed CLAUDE.md gives the accepted end state" "$claude" "15 failures, 547 pending"
has "D: installed CLAUDE.md names the rlimit_nproc examples" "$claude" "rlimit_nproc: 0"
has "D: the Goal names the accepted examples" "$goal" "rlimit_nproc: 0"
has "D: the Spec line names the accepted examples" "$spec" "rlimit_nproc: 0"
lacks "E: no unqualified 'no order guarantees' rule" "$claude" "Do not provide any order guarantees."
has "E: the order rule keeps GNU's forced order" "$claude" "beyond what GNU semantics force"
has "O: installed CLAUDE.md allows the .scratch/diff checks" "$claude" ".scratch/diff/"
has "O: Global Constraints allow the .scratch/diff checks" "$testbed" ".scratch/diff/"
has "L: installed CLAUDE.md keeps 'first and foremost' the /rspec skill" "$claude" "first and foremost use the /rspec skill"
while IFS= read -r rule; do
  has "L: keeps the user's rule '${rule:0:40}...'" "$claude" "$rule"
done < <(git -C "$root" show HEAD:CLAUDE.md | grep -E '^[1-4]\. |^Use rspec features')
has "git: installed CLAUDE.md has a '## Git' section" "$claude" "## Git"
while IFS= read -r rule; do
  has "git: keeps the user's rule '${rule:0:40}...'" "$claude" "$rule"
done < <(git -C "$root" show HEAD:CLAUDE.md | awk '/^## Git$/ { g = 1; next } g && /^## / { exit } g && NF { print }')
[ "$fail" -eq 0 ] && echo PASS || echo FAIL
exit "$fail"
````

- [ ] **Step 6: Run it to see it fail**

Run: `.scratch/fixcheck/05-task-statement.sh`
Expected: exit 1, with 15 `FAIL:` lines and one `ok:`. The `rlimit_nproc` line already passes, because patch 03 named those examples in the Setup section:
```
FAIL: D: installed CLAUDE.md gives the accepted end state
ok: D: installed CLAUDE.md names the rlimit_nproc examples
FAIL: D: the Goal names the accepted examples
FAIL: D: the Spec line names the accepted examples
FAIL: E: no unqualified 'no order guarantees' rule
FAIL: E: the order rule keeps GNU's forced order
FAIL: O: installed CLAUDE.md allows the .scratch/diff checks
FAIL: O: Global Constraints allow the .scratch/diff checks
FAIL: L: installed CLAUDE.md keeps 'first and foremost' the /rspec skill
FAIL: L: keeps the user's rule '1. These are blackbox tests. This means ...'
FAIL: L: keeps the user's rule '2. 1 expectation per example. No excepti...'
FAIL: L: keeps the user's rule '3. Absolutely no comments allowed in the...'
FAIL: L: keeps the user's rule '4. Constantly reference the rspec source...'
FAIL: L: keeps the user's rule 'Use rspec features like shared_examples,...'
FAIL: git: installed CLAUDE.md has a '## Git' section
FAIL: git: keeps the user's rule 'Make every commit a clean, new commit. N...'
FAIL
```

- [ ] **Step 7: Apply patch 05**

Run: `git apply .scratch/patches/05-task-statement-matches-plan.patch`

**Patch `05-task-statement-matches-plan.patch`**

````diff
--- a/_claude/plans/master_plan.md
+++ b/_claude/plans/master_plan.md
@@ -2,7 +2,7 @@
 
 > **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
 
-**Goal:** Build `qfind`, a parallel Go reimplementation of GNU find 4.11.0 on top of `github.com/charlievieth/fastwalk`, installed as `dist/find`, that passes the existing black-box RSpec suite exactly as GNU find does (0 failures, the same 547 pending examples).
+**Goal:** Build `qfind`, a parallel Go reimplementation of GNU find 4.11.0 on top of `github.com/charlievieth/fastwalk`, installed as `dist/find`, that passes the existing black-box RSpec suite exactly as GNU find does (0 failures, the same 547 pending examples), except for the 15 `rlimit_nproc: 0` examples that no Go binary can start under (Global Constraints).
 
 **Architecture:** Port GNU find's *semantics* faithfully and replace only its *traversal*. The command-line parser (`parser.c`), expression tree builder and optimiser (`tree.c`), predicates (`pred.c`), `-printf`/`-ls` formatting (`print.c`, `listfile.c`), command execution (`exec.c`, `buildcmd.c`) and the per-entry visiting logic (`ftsfind.c consider_visiting`) are ported structure-for-structure into one Go package (`internal/find`). That makes evaluation order, `stat()` timing, diagnostics and `-D` output match GNU at every `-O` level. fts is replaced by fastwalk, used unmodified as fzf uses it. Each starting point is walked in order, and its subtree is walked and evaluated in parallel inside fastwalk's callbacks.
 
@@ -24,7 +24,7 @@
 
 **Tech Stack:** Go 1.27 (installed Fedora go1.27.2; `go.env` pins `GOTOOLCHAIN=local`), `github.com/charlievieth/fastwalk` v1.0.14 (unmodified), `golang.org/x/sys/unix`, cgo enabled (for `os/user` NSS lookups). Ruby 4 + RSpec suite in `spec/` is the only test bed. Rake builds everything.
 
-**Spec:** `spec/` (19,214 examples: 18,667 must pass and 547 must stay pending), `spec/README.md` (harness, quirks, the 4 known 4.11.0 bugs that must stay pending) and `CLAUDE.md`. For behaviour questions use the GNU source in `findutils-4.11.0/` (find/*.c, lib/*.c, gl/lib/*.c) and the oracle binary `findutils-4.11.0/find/find`.
+**Spec:** `spec/` (19,214 examples: 547 must stay pending, and the other 18,667 must pass except the 15 accepted `rlimit_nproc: 0` examples), `spec/README.md` (harness, quirks, the 4 known 4.11.0 bugs that must stay pending) and `CLAUDE.md`. For behaviour questions use the GNU source in `findutils-4.11.0/` (find/*.c, lib/*.c, gl/lib/*.c) and the oracle binary `findutils-4.11.0/find/find`.
 
 ## Context
 
@@ -99,7 +99,7 @@
 - The binary is `dist/find`, built from Go sources by `rake qfind:build` (`go build -trimpath -o dist/find ./cmd/qfind`). Remove `dist/find` before building, because the old file is a hard link to the GNU binary and a copy-on-build would overwrite it.
 - Use Go only. The module is `qfind` with `go 1.27` in `go.mod`. The installed toolchain is Fedora go1.27.2 with `GOTOOLCHAIN=local`, so never declare a newer `go` line and never use Go 1.28 syntax (`p = {1, 2}`).
 - Traversal uses `github.com/charlievieth/fastwalk` v1.0.14, unmodified, and is the only directory walker.
-- **Do not write any tests in Go.** The RSpec suite is the only test bed. Do not edit `spec/` except where a task explicitly says so; none does.
+- **Do not write any tests in Go.** The RSpec suite is the only test bed in the repository. The Review Focus differential checks in git-ignored `.scratch/diff/` only compare qfind with the oracle; they are not tests. Do not edit `spec/` except where a task explicitly says so; none does.
 - Success means `bundle exec rspec` reports **0 failures and 547 pending**, exactly like `FIND=findutils-4.11.0/find/find bundle exec rspec`. The single exception is the 15 `rlimit_nproc: 0` examples (5 contexts × 3), which no Go runtime can start under. Report them; do not hack around them. As uid 1000, the final run therefore reads **15 failures, 547 pending**, and every failure is one of those contexts.
 - Concurrency invariants:
   - every entry's stdout and output-file writes form one atomic transaction;
@@ -357,8 +357,8 @@
 
 - The goal is 100% congruence with GNU find, but with parallelism. It should look, feel and work like GNU find, albeit much faster.
 - An exhaustive black-box RSpec suite has been prepared in `spec/`. It only requires a binary at `dist/find`.
-- Do not write any tests in Go. Rely solely on the spec suite.
-- This is a streaming problem, so order does not matter (GNU find does not guarantee it either). Do not provide any order guarantees.
+- Do not write any tests in Go. Rely solely on the spec suite; the throwaway differential checks against GNU find in git-ignored `.scratch/diff/` are not tests.
+- This is a streaming problem, so order does not matter (GNU find does not guarantee it either). Do not provide any order guarantees beyond what GNU semantics force: starting points in command-line order, a directory before its contents (after them with `-depth`), and one file's actions in sequence.
 - fastwalk is deliberate: fzf uses it, and fzf is one of the gold-standard CLI tools of the last decade. Use it unmodified and adapt qfind to its model rather than forking it.
 
 The implementation plan is `_claude/plans/master_plan.md`.
@@ -373,12 +373,20 @@
 
 ## The spec suite
 
-`spec/` black-boxes `dist/find`, and `spec/README.md` documents its harness and every GNU quirk it pins. qfind is done when `bundle exec rspec` reports 0 failures and 547 pending, exactly as GNU find does. If the specs themselves ever need to change, these rules apply:
+`spec/` black-boxes `dist/find`, and `spec/README.md` documents its harness and every GNU quirk it pins. qfind is done when `bundle exec rspec` matches GNU find: 0 failures and 547 pending, apart from the 15 `rlimit_nproc: 0` examples (5 contexts × 3), which no Go binary can start under. As a non-root user the final run therefore reads 15 failures, 547 pending; report those 15 and do not hack around them. (`spec/README.md`'s "0 failures and 547 pending" is GNU find's result.)
 
-1. These are black-box tests: no stubs or mocks. Every spec shells out to find.
-2. One expectation per example. No exceptions.
-3. Absolutely no comments in the specs; put context in `spec/README.md`.
-4. Use the /rspec skill and constantly reference the rspec source in `references/rspec` for idiomatic use.
+If the specs themselves ever need to change, first and foremost use the /rspec skill, and these rules from the original task apply:
+
+1. These are blackbox tests. This means no stubs or mocks. EVERY spec should actually shell out to find/gfind
+2. 1 expectation per example. No exceptions
+3. Absolutely no comments allowed in the specs. Any comments in specs need to be organized into their own specs. Context that cannot become a spec goes in `spec/README.md`.
+4. Constantly reference the rspec source to ensure idiomatic use. It can be found in references/rspec
+
+Use rspec features like shared_examples, shared_context etc to keep things DRY and behavior based, if needed
+
+## Git
+
+Make every commit a clean, new commit. No amending and no force pushing.
 ```
 
 - [ ] **Step 8: Save the fastwalk decision to memory**
````

- [ ] **Step 8: Run the task-statement check to see it pass**

Run: `.scratch/fixcheck/05-task-statement.sh`
Expected: exit 0, with 16 `ok:` lines and then `PASS`.

- [ ] **Step 9: Commit**

```bash
git add _claude/plans/master_plan.md
git commit -m "Align the qfind plan's Task 1 and CLAUDE.md with the repo and the plan"
```

---

### Task 4: Fix the plan's oracle wrapper and its differential and spot checks (issues J, G, I, H, K, P)

**Files:**
- Modify: `_claude/plans/master_plan.md` — the wrappers in Task 1 Step 6 (L329-336) and the Berlin pass (L11262), via patch 06; the Task 3 and Task 13 spot checks (L3530, L11711), via patch 07; `pipes.sh` (L3535-3547), via patch 08; `many.sh` (L4426-4427) and `bytes.sh` (L9103-9104), via patch 09
- Create (git-ignored): `.scratch/fixcheck/06-wrapper-tz.sh`, `07-spot-checks.sh`, `08-pipes.sh`, `09-many-bytes.sh`

**Interfaces:**
- Consumes: Task 3's text.
- Produces: `.scratch/gnufind` and `.scratch/qf` export `LC_ALL=C TZ=${TZ:-UTC}`; `pipes.sh` exports `LC_ALL=C` and runs the head pipeline without a subshell; `many.sh` prints five `same:` lines (`-print`, `-printf %p:%d\n`, `-print0`, `-fprint stdout`, `-fprint file`) and writes `$D.g.out`/`$D.q.out`; `bytes.sh` sorts `-print0` output with `sort -z`.

- [ ] **Step 1: Write the failing wrapper check**

**Check `.scratch/fixcheck/06-wrapper-tz.sh`**

````bash
#!/bin/bash
# Review issue J: the oracle wrapper that the plan defines must keep a
# caller's TZ (UTC when unset), and must keep LC_ALL pinned to C even though
# the login shell exports LC_ALL=en_US.UTF-8.
set -u
root=/workspace/qfind.master
plan=${PLAN:-$root/_claude/plans/master_plan.md}
d=$root/.scratch/fixcheck/06
rm -rf "$d" && mkdir -p "$d"
awk '/^`\.scratch\/gnufind`/ { s = 1; next }
     s && /^```bash$/ { f = 1; next }
     f && /^```$/ { exit }
     f { print }' "$plan" > "$d/gnufind"
chmod +x "$d/gnufind"
echo "plan's wrapper:"; sed 's/^/  /' "$d/gnufind"
touch -d '2020-06-01 09:00:00 UTC' "$d/f"
fail=0
expect() { # label, wanted output, command...
  local label=$1 want=$2 got
  shift 2
  got=$("$@" 2>&1)
  if [ "$got" = "$want" ]; then echo "ok: $label -> $got"; else echo "FAIL: $label -> $got (want: $want)"; fail=1; fi
}
expect "TZ unset gives UTC" "UTC 09" env -u TZ "$d/gnufind" "$d/f" -printf '%TZ %TH\n'
expect "TZ=Europe/Berlin is kept" "CEST 11" env TZ=Europe/Berlin "$d/gnufind" "$d/f" -printf '%TZ %TH\n'
expect "TZ=JST-9 is kept" "JST 18" env TZ=JST-9 "$d/gnufind" "$d/f" -printf '%TZ %TH\n'
expect "LC_ALL stays C under en_US.UTF-8" "find: '$d/missing': No such file or directory" env LC_ALL=en_US.UTF-8 "$d/gnufind" "$d/missing"
[ "$fail" -eq 0 ] && echo PASS || echo FAIL
exit "$fail"
````

- [ ] **Step 2: Run it to see it fail**

Run: `.scratch/fixcheck/06-wrapper-tz.sh`
Expected: exit 1:
```
ok: TZ unset gives UTC -> UTC 09
FAIL: TZ=Europe/Berlin is kept -> UTC 09 (want: CEST 11)
FAIL: TZ=JST-9 is kept -> UTC 09 (want: JST 18)
ok: LC_ALL stays C under en_US.UTF-8 -> find: '/workspace/qfind.master/.scratch/fixcheck/06/missing': No such file or directory
FAIL
```

- [ ] **Step 3: Apply patch 06**

Run: `git apply .scratch/patches/06-wrappers-keep-tz.patch`

**Patch `06-wrappers-keep-tz.patch`**

````diff
--- a/_claude/plans/master_plan.md
+++ b/_claude/plans/master_plan.md
@@ -326,14 +326,14 @@
 
 - [ ] **Step 6: Scratch helpers (git-ignored)**
 
-`.scratch/gnufind`:
+`.scratch/gnufind` (a caller's `TZ` wins, as an example's `env:` does in the suite; `LC_ALL` stays pinned to `C`, because the login shell exports `LC_ALL=en_US.UTF-8`):
 ```bash
 #!/bin/bash
-export LC_ALL=C TZ=UTC GNU_FINDUTILS_FD_LEAK_CHECK=1
+export LC_ALL=C TZ=${TZ:-UTC} GNU_FINDUTILS_FD_LEAK_CHECK=1
 unset POSIXLY_CORRECT LANG LANGUAGE
 exec -a find /workspace/qfind.master/findutils-4.11.0/find/find "$@"
 ```
-`.scratch/qf`: the same with `/workspace/qfind.master/dist/find`.
+`.scratch/qf`: the same with `/workspace/qfind.master/dist/find`. To reproduce an example that sets `LC_ALL` or `LANG`, run the binary directly with that example's environment instead of a wrapper.
 `.scratch/failures.rb`:
 ```ruby
 require "json"
@@ -11266,7 +11266,7 @@
 - every NaN, exponent and hexadecimal count form;
 - the `-follow` before/after reference-file cases.
 
-Then differential-check `parse_datetime` against the oracle. Its date strings are a long tail, so compare `.scratch/gnufind . -maxdepth 0 -newermt "$s" -printf x` with `.scratch/qf` for 60 strings, under both `TZ=UTC` and `TZ=Europe/Berlin`:
+Then differential-check `parse_datetime` against the oracle. Its date strings are a long tail, so compare `.scratch/gnufind . -maxdepth 0 -newermt "$s" -printf x` with `.scratch/qf` for 60 strings, under both `TZ=UTC` and `TZ=Europe/Berlin` (prefix both wrappers with the zone, e.g. `TZ=Europe/Berlin .scratch/gnufind …`; they keep a caller's `TZ`):
 - `"now"`, `"yesterday"`, `"tomorrow 10am"`, `"last friday"`, `"next month"`, `"2 weeks ago"`, `"1 fortnight"`;
 - `"Jan 1"`, `"1 Jan 2000"`, `"Jan-1-2000"`, `"1/2/03"`, `"2003/1/2"`, `"17.6."`, `"17.6.1992"`;
 - `"20200101"`, `"20200101 +3 days"`, `"10:30pm"`, `"10:30:15.5"`, `"12am"`, `"12pm"`;
````

- [ ] **Step 4: Run the wrapper check to see it pass**

Run: `.scratch/fixcheck/06-wrapper-tz.sh`
Expected: exit 0, with `CEST 11` for Europe/Berlin and `JST 18` for JST-9, `UTC 09` with no TZ, ASCII quotes under `LC_ALL=en_US.UTF-8`, then `PASS`.

- [ ] **Step 5: Write the failing spot-check check**

**Check `.scratch/fixcheck/07-spot-checks.sh`**

````bash
#!/bin/bash
# Review issues G and I: two hand checks in the plan must parse and must show
# what they are there for. They run against the GNU oracle here, which is
# what qfind has to match.
set -u
root=/workspace/qfind.master
plan=${PLAN:-$root/_claude/plans/master_plan.md}
d=$root/.scratch/fixcheck/07
rm -rf "$d" && mkdir -p "$d/dist" && touch "$d/file"
ln -s "$root/findutils-4.11.0/find/find" "$d/dist/find"
fail=0
g=$(grep -m1 '^\./dist/find file -printf x' "$plan")
echo "Task 3 spot check: $g"
if ! err=$(bash -n <<<"$g" 2>&1); then
  echo "FAIL: not valid shell: $err"
  fail=1
else
  out=$(cd "$d" && bash -c "$g" 2>&1)
  if grep -qF "you have too many ')'" <<<"$out"; then echo "ok: find is handed the unmatched ')'"; else echo "FAIL: no unmatched-')' diagnostic in: $out"; fail=1; fi
  if grep -qF "binary operator '-a' with nothing before it" <<<"$out"; then echo "ok: the dangling -a is diagnosed"; else echo "FAIL: no dangling -a diagnostic in: $out"; fail=1; fi
fi
i=$(grep -m1 '^\./dist/find .*-fstype nosuchfs .*can never succeed' "$plan")
echo "Task 13 check: $i"
out=$(cd "$d" && bash -c "$i" 2>&1)
if grep -qF -- '-fstype nosuchfs can never succeed, substituting -false' <<<"$out"; then
  echo "ok: $out"
else
  echo "FAIL: it printed ${out:-nothing}; unfiltered: $(cd "$d" && bash -c "${i%% 2>&1*}" 2>&1 | head -1)"
  fail=1
fi
[ "$fail" -eq 0 ] && echo PASS || echo FAIL
exit "$fail"
````

- [ ] **Step 6: Run it to see it fail**

Run: `.scratch/fixcheck/07-spot-checks.sh`
Expected: exit 1:
```
Task 3 spot check: ./dist/find file -printf x 2>&1; ./dist/find . -print ) ; ./dist/find . -a
FAIL: not valid shell: bash: line 1: syntax error near unexpected token `)'
bash: line 1: `./dist/find file -printf x 2>&1; ./dist/find . -print ) ; ./dist/find . -a'
Task 13 check: ./dist/find / -maxdepth 0 -O2 -D opt -fstype nosuchfs 2>&1 | grep 'can never succeed'
FAIL: it printed nothing; unfiltered: ./dist/find: unknown predicate `-O2'
FAIL
```

- [ ] **Step 7: Apply patch 07**

Run: `git apply .scratch/patches/07-spot-checks-run.patch`

**Patch `07-spot-checks-run.patch`**

````diff
--- a/_claude/plans/master_plan.md
+++ b/_claude/plans/master_plan.md
@@ -3534,7 +3534,7 @@
 List them with `ruby .scratch/failures.rb`. Spot-check by hand:
 ```bash
 ./dist/find -D opt -O3 . -size 0 -type f -name file 2>&1 >/dev/null | grep -c 'promoting cheap'   # 0 until Tasks 5–11 parse those primaries
-./dist/find file -printf x 2>&1; ./dist/find . -print ) ; ./dist/find . -a
+./dist/find file -printf x 2>&1; ./dist/find . -print ')' ; ./dist/find . -a
 ```
 
 - [ ] **Step 13: Review Focus 4 differential check**
@@ -11715,7 +11715,7 @@
 ```bash
 bundle exec rake qfind:build
 bundle exec rspec spec/find/tests/{perm,readable,writable,executable,fstype,context}_spec.rb spec/find/options/optimisation_spec.rb spec/find/options/version_spec.rb 2>&1 | grep -E '^[0-9]+ examples?, '
-./dist/find / -maxdepth 0 -O2 -D opt -fstype nosuchfs 2>&1 | grep 'can never succeed'
+./dist/find -O2 -D opt / -maxdepth 0 -fstype nosuchfs 2>&1 | grep 'can never succeed'
 ```
 Expected: 0 failures. The last command prints `-fstype nosuchfs can never succeed, substituting -false`.
 
````

- [ ] **Step 8: Run the spot-check check to see it pass**

Run: `.scratch/fixcheck/07-spot-checks.sh`
Expected: exit 0:
```
ok: find is handed the unmatched ')'
ok: the dangling -a is diagnosed
ok: -fstype nosuchfs can never succeed, substituting -false
PASS
```

- [ ] **Step 9: Write the failing pipes check**

**Check `.scratch/fixcheck/08-pipes.sh`**

````bash
#!/bin/bash
# Review issue H: the plan's pipes.sh must report find's own exit status when
# head closes the pipe (141, SIGPIPE), and must still run both binaries'
# /dev/full checks (a bare exec there would replace the script).
set -u
root=/workspace/qfind.master
plan=${PLAN:-$root/_claude/plans/master_plan.md}
d=$root/.scratch/fixcheck/08
rm -rf "$d" && mkdir -p "$d"
awk '/^`\.scratch\/diff\/pipes\.sh`:/ { s = 1; next }
     s && /^```bash$/ { f = 1; next }
     f && /^```$/ { exit }
     f { print }' "$plan" > "$d/pipes.sh"
before=$(ls -d "$root"/.scratch/tmp.* 2>/dev/null)
out=$(bash "$d/pipes.sh" 2>&1)
for t in $(ls -d "$root"/.scratch/tmp.* 2>/dev/null); do grep -qxF "$t" <<<"$before" || rm -rf "$t"; done
sed 's/^/  /' <<<"$out"
fail=0
count() { grep -cxF -- "$1" <<<"$out"; }
[ "$(grep -c '^== ' <<<"$out")" -eq 2 ] || { echo "FAIL: pipes.sh did not run both binaries"; fail=1; }
[ "$(count 'exit=1')" -eq 2 ] || { echo "FAIL: want exit=1 from both /dev/full runs"; fail=1; }
[ "$(count "find: 'standard output': No space left on device")" -eq 2 ] || { echo "FAIL: want GNU's /dev/full diagnostic twice"; fail=1; }
[ "$(count 'pipestatus=141')" -eq 2 ] || { echo "FAIL: want pipestatus=141 (SIGPIPE) from both binaries"; fail=1; }
[ "$fail" -eq 0 ] && echo PASS || echo FAIL
exit "$fail"
````

- [ ] **Step 10: Run it to see it fail**

Run: `.scratch/fixcheck/08-pipes.sh`
Expected: exit 1. Both binaries report `pipestatus=0`, and GNU quotes `‘standard output’` under the shell's locale:
```
FAIL: want GNU's /dev/full diagnostic twice
FAIL: want pipestatus=141 (SIGPIPE) from both binaries
FAIL
```

- [ ] **Step 11: Apply patch 08**

Run: `git apply .scratch/patches/08-pipes-sees-find.patch`

**Patch `08-pipes-sees-find.patch`**

````diff
--- a/_claude/plans/master_plan.md
+++ b/_claude/plans/master_plan.md
@@ -3543,12 +3543,13 @@
 ```bash
 #!/bin/bash
 set -u
+export LC_ALL=C   # the suite's locale; the login shell exports en_US.UTF-8
 cd "$(mktemp -d -p /workspace/qfind.master/.scratch)"
 mkdir -p d && touch d/{1..2000}
 for f in /workspace/qfind.master/findutils-4.11.0/find/find /workspace/qfind.master/dist/find; do
   echo "== $f"
-  (exec -a find "$f" d -print > /dev/full); echo "exit=$?"
-  (exec -a find "$f" d -print | head -1 >/dev/null); echo "pipestatus=${PIPESTATUS[0]}"
+  (exec -a find "$f" d -print > /dev/full); echo "exit=$?"   # subshell: a bare exec would replace this script
+  exec -a find "$f" d -print | head -1 >/dev/null; echo "pipestatus=${PIPESTATUS[0]}"   # no subshell: PIPESTATUS must see find itself
 done
 ```
 Expected: both binaries print the same two lines for `/dev/full` (`find: 'standard output': No space left on device`, then `find: write error`), with `exit=1`, and the same pipe status (141).
````

- [ ] **Step 12: Run the pipes check to see it pass**

Run: `.scratch/fixcheck/08-pipes.sh`
Expected: exit 0. Each binary's section shows `find: 'standard output': No space left on device`, `find: write error`, `exit=1` and `pipestatus=141`, then `PASS`.

- [ ] **Step 13: Write the failing many/bytes check**

**Check `.scratch/fixcheck/09-many-bytes.sh`**

````bash
#!/bin/bash
# Review issues K and P: many.sh and bytes.sh must compare records, not
# output order (a correct parallel port prints them in any order), and
# many.sh must compare the -fprint file each binary wrote.
set -u
root=/workspace/qfind.master
plan=${PLAN:-$root/_claude/plans/master_plan.md}
d=$root/.scratch/fixcheck/09
rm -rf "$d" && mkdir -p "$d"
# Stand-in for a correct parallel qfind: GNU find with its records, and its
# -fprint file, shuffled. CORRUPT_FPRINT=1 also appends a bogus record to
# the -fprint file, a negative control for many.sh.
cat > "$d/shuffled-find" <<'EOF'
#!/bin/bash
g=/workspace/qfind.master/findutils-4.11.0/find/find
fprint= z= prev=
for a in "$@"; do
  [ "$prev" = -fprint ] && fprint=$a
  [ "$a" = -print0 ] && z=-z
  prev=$a
done
(exec -a find "$g" "$@") | shuf $z
status=${PIPESTATUS[0]}
if [ -n "$fprint" ] && [ -f "$fprint" ]; then
  shuf -o "$fprint" "$fprint"
  [ -n "${CORRUPT_FPRINT:-}" ] && echo corrupt >> "$fprint"
fi
exit "$status"
EOF
chmod +x "$d/shuffled-find"
script() { # start-of-line pattern of the paragraph that introduces the script
  awk -v pat="$1" '$0 ~ pat { s = 1; next }
                   s && /^```bash$/ { f = 1; next }
                   f && /^```$/ { exit }
                   f { print }' "$plan"
}
# Test-only edits: a 2,000-file tree instead of 200,000, and the stand-in as qfind.
script '^`\\.scratch/diff/many\\.sh`:' |
  sed "s#^D=/workspace/qfind.master/.scratch/diff/manytree#D=$d/manytree#; s#seq 1 2000#seq 1 20#; s#Q=/workspace/qfind.master/dist/find#Q=$d/shuffled-find#" > "$d/many.sh"
script '^Then `\\.scratch/diff/bytes\\.sh`' |
  sed "s#^D=/workspace/qfind.master/.scratch/diff/bytetree#D=$d/bytetree#; s#Q=/workspace/qfind.master/dist/find#Q=$d/shuffled-find#" > "$d/bytes.sh"
fail=0
many=$(bash "$d/many.sh" 2>&1)
echo "many.sh:"; sed 's/^/  /' <<<"$many"
[ "$(grep -c '^same: ' <<<"$many")" -eq 5 ] || { echo "FAIL: many.sh must print five same: lines"; fail=1; }
grep -qx 'same: -print0' <<<"$many" || { echo "FAIL: -print0 records are compared in output order"; fail=1; }
grep -qx 'same: -fprint file' <<<"$many" || { echo "FAIL: the -fprint file is never compared"; fail=1; }
rm -f "$d"/manytree.*
corrupt=$(CORRUPT_FPRINT=1 bash "$d/many.sh" 2>&1)
if grep -qx 'same: -fprint file' <<<"$corrupt"; then echo "FAIL: many.sh passes a corrupt -fprint file"; fail=1; else echo "ok: many.sh catches a corrupt -fprint file"; fi
bytes=$(bash "$d/bytes.sh" 2>&1)
echo "bytes.sh: $(grep -c '^same: ' <<<"$bytes") same, $(grep -c '^DIFF: ' <<<"$bytes") DIFF"
grep '^DIFF: ' <<<"$bytes" | sed 's/^/  /'
[ "$(grep -c '^same: ' <<<"$bytes")" -eq 14 ] || { echo "FAIL: bytes.sh must print 14 same: lines"; fail=1; }
[ "$fail" -eq 0 ] && echo PASS || echo FAIL
exit "$fail"
````

- [ ] **Step 14: Run it to see it fail**

Run: `.scratch/fixcheck/09-many-bytes.sh`
Expected: exit 1:
```
many.sh:
  same: -print
  /dev/fd/63 /dev/fd/62 differ: byte 54, line 1
  same: -printf %p:%d\n
  same: -fprint /workspace/qfind.master/.scratch/fixcheck/09/manytree.out -print
FAIL: many.sh must print five same: lines
FAIL: -print0 records are compared in output order
FAIL: the -fprint file is never compared
ok: many.sh catches a corrupt -fprint file
bytes.sh: 12 same, 2 DIFF
  DIFF: C -print0
  DIFF: C.UTF-8 -print0
FAIL: bytes.sh must print 14 same: lines
FAIL
```
The byte offset in the `differ:` line changes from run to run, because the stand-in shuffles.

- [ ] **Step 15: Apply patch 09**

Run: `git apply .scratch/patches/09-diffs-compare-records.patch`

**Patch `09-diffs-compare-records.patch`**

````diff
--- a/_claude/plans/master_plan.md
+++ b/_claude/plans/master_plan.md
@@ -4431,10 +4431,19 @@
 D=/workspace/qfind.master/.scratch/diff/manytree
 [ -d "$D" ] || { mkdir -p "$D"; for i in $(seq 1 2000); do mkdir "$D/d$i"; (cd "$D/d$i" && touch $(seq -f f%g 1 100)); done; }
 G=/workspace/qfind.master/findutils-4.11.0/find/find Q=/workspace/qfind.master/dist/find
-for args in "-print" "-print0" "-printf %p:%d\n" "-fprint $D.out -print"; do
+for args in "-print" "-printf %p:%d\n"; do
   cmp <(exec -a find $G $D $args | sort) <(exec -a find $Q $D $args | sort) && echo "same: $args"
 done
+cmp <(exec -a find $G $D -print0 | sort -z) <(exec -a find $Q $D -print0 | sort -z) && echo "same: -print0"
+# Each binary writes its own -fprint file, one run after the other, so each
+# file holds only what that binary wrote.
+(exec -a find $G $D -fprint $D.g.out -print) | sort > $D.g.stdout
+(exec -a find $Q $D -fprint $D.q.out -print) | sort > $D.q.stdout
+cmp $D.g.stdout $D.q.stdout && echo "same: -fprint stdout"
+cmp <(sort $D.g.out) <(sort $D.q.out) && echo "same: -fprint file"
 ```
+It must print five `same:` lines; otherwise `cmp` names the first byte that differs.
+
 `.scratch/diff/huge.sh` (peak RSS must stay below 64 MiB for both runs, since files are never deferred):
 ```bash
 #!/bin/bash
@@ -9109,7 +9118,8 @@
 fi
 for loc in C C.UTF-8; do
   for args in "-name *" "-regex .*" "-iregex .*[A-Z].*" "-regex .*[^a].*" "-printf %p|%f|%h\n" "-ls" "-print0"; do
-    if cmp -s <(LC_ALL=$loc exec -a find $G $D $args 2>&1 | sort) <(LC_ALL=$loc exec -a find $Q $D $args 2>&1 | sort)
+    z=; [ "$args" = -print0 ] && z=-z   # NUL-separated records sort with -z
+    if cmp -s <(LC_ALL=$loc exec -a find $G $D $args 2>&1 | sort $z) <(LC_ALL=$loc exec -a find $Q $D $args 2>&1 | sort $z)
     then echo "same: $loc $args"; else echo "DIFF: $loc $args"; fi
   done
 done
````

- [ ] **Step 16: Run the many/bytes check to see it pass**

Run: `.scratch/fixcheck/09-many-bytes.sh`
Expected: exit 0:
```
many.sh:
  same: -print
  same: -printf %p:%d\n
  same: -print0
  same: -fprint stdout
  same: -fprint file
ok: many.sh catches a corrupt -fprint file
bytes.sh: 14 same, 0 DIFF
PASS
```

- [ ] **Step 17: Commit**

```bash
git add _claude/plans/master_plan.md
git commit -m "Fix the qfind plan's oracle wrapper and differential checks"
```

---

### Task 5: Fix the plan's fatal-exit and inherited-descriptor code (issues Q, R)

**Files:**
- Modify: `_claude/plans/master_plan.md` — File Structure `run.go` line (L145), Task 2's `diag.go` (`fatalf` L1053-1058, `exit` L1066-1093) and the `Run` field note (L1096), Task 15's profiling-hook sentence (L12061), via patch 10; Task 8's `recordInheritedFDs` (L7241-7248), via patch 11
- Create (git-ignored): `.scratch/fixcheck/10-dies-once.sh`, `.scratch/fixcheck/11-inherited-fds.sh`

**Interfaces:**
- Consumes: Task 4's text.
- Produces: in the master plan's Go code, `func (r *Run) claimExit()`, `func (r *Run) exitClaimed(status int)` and the `Run` field `exiting atomic.Bool`, which replaces `dying sync.Mutex`. `exit(status int)` keeps its name and signature for its ten callers. `fatalf` calls `claimExit` before `errorf`. `recordInheritedFDs` records only descriptors without `FD_CLOEXEC`.

- [ ] **Step 1: Write the failing fatal-path check**

**Check `.scratch/fixcheck/10-dies-once.sh`**

````bash
#!/bin/bash
# Review issue Q: the plan's fatal path must print one message when several
# goroutines hit a fatal error at once, and a fatal error raised inside
# fatalAt's commit (same goroutine) must still exit, not hang. The plan's
# own fatalf, claimExit, exit, exitClaimed and fatalAt are compiled against
# stand-ins for the rest of Run.
set -u
root=/workspace/qfind.master
plan=${PLAN:-$root/_claude/plans/master_plan.md}
d=$root/.scratch/fixcheck/10
rm -rf "$d" && mkdir -p "$d"
printf 'module diesonce\n\ngo 1.27\n' > "$d/go.mod"
{
  printf 'package main\n\nimport "os"\n\nvar _ = os.Exit\n\n'
  awk '/^func \(r \*Run\) (fatalf|claimExit|exit|exitClaimed|fatalAt)\(/, /^}/' "$plan"
} > "$d/plan.go"
cat > "$d/main.go" <<'EOF'
package main

// Stand-ins for the parts of the plan's Run that its fatal path touches.

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type Output struct{}

// flushFilesQuietly takes as long as flushing a large buffer, which gives a
// second goroutine time to report before the first one exits.
func (o *Output) flushFilesQuietly() { time.Sleep(20 * time.Millisecond) }
func (o *Output) closeAll() error    { return nil }

var errPrevFailed = errors.New("previous write failed")

type Entry struct{}

type Run struct {
	out          *Output
	stderrMu     sync.Mutex
	stderrFailed atomic.Bool
	dying        sync.Mutex  // used by the plan before the fix
	exiting      atomic.Bool // used by the plan after the fix
}

func (r *Run) errorf(err error, format string, a ...any) {
	r.stderrMu.Lock()
	defer r.stderrMu.Unlock()
	fmt.Fprintf(os.Stderr, "find: "+format+"\n", a...)
}

// commit stands in for an -ls render whose write fails, so it dies too.
func (r *Run) commit(e *Entry) { r.fatalf(nil, "Failed to write output (at stage %d)", 1) }

func main() {
	r := &Run{out: &Output{}}
	switch os.Args[1] {
	case "concurrent":
		start := make(chan struct{})
		for range 8 {
			go func() {
				<-start
				r.fatalf(nil, "Cannot read mounted file system list")
			}()
		}
		close(start)
		select {}
	case "nested":
		r.fatalAt(&Entry{}, nil, "Cannot read mounted file system list")
	}
}
EOF
if ! (cd "$d" && go build -o diesonce . 2>&1); then
  echo "FAIL: the plan's fatal path does not compile against the stand-ins"
  exit 1
fi
fail=0
worst=0
for _ in $(seq 1 20); do
  n=$("$d/diesonce" concurrent 2>&1 >/dev/null | grep -c '^find: ')
  [ "$n" -gt "$worst" ] && worst=$n
done
if [ "$worst" -eq 1 ]; then
  echo "ok: 8 goroutines dying at once print one message (20 runs)"
else
  echo "FAIL: one run printed $worst fatal messages"
  fail=1
fi
out=$(timeout 5 "$d/diesonce" nested 2>&1)
rc=$?
if [ "$rc" -eq 1 ] && [ "$out" = "find: Failed to write output (at stage 1)" ]; then
  echo "ok: a write failure inside fatalAt's commit exits 1 with its own message"
else
  echo "FAIL: nested fatal error: exit $rc, output: ${out:-<nothing>}"
  fail=1
fi
if grep -nE 'r\.dying|dying sync\.Mutex' "$plan"; then
  echo "FAIL: the plan still refers to the dying lock"
  fail=1
fi
[ "$fail" -eq 0 ] && echo PASS || echo FAIL
exit "$fail"
````

- [ ] **Step 2: Run it to see it fail**

Run: `.scratch/fixcheck/10-dies-once.sh`
Expected: exit 1. All eight goroutines print, because `fatalf` reports before `exit` takes the lock:
```
FAIL: one run printed 8 fatal messages
ok: a write failure inside fatalAt's commit exits 1 with its own message
1083:	r.dying.Lock()
1103:Add `dying sync.Mutex` to `Run`.
FAIL: the plan still refers to the dying lock
FAIL
```

- [ ] **Step 3: Apply patch 10**

Run: `git apply .scratch/patches/10-fatal-error-prints-once.patch`

**Patch `10-fatal-error-prints-once.patch`**

````diff
--- a/_claude/plans/master_plan.md
+++ b/_claude/plans/master_plan.md
@@ -142,7 +142,7 @@
   main.go            Main: defaults, locale, leading options, tree, needsSerial, start points, cleanup   [2,3,4]  (ftsfind.c main)
   options.go         Options, DebugFlags, SymlinkMode, set_option_defaults, set_follow_state, isatty       [2]  (util.c, parser.c)
   leading.go         -H -L -P -D -O, -D help, process_debug_options, process_optimisation_option          [2]  (util.c)
-  run.go             Run: exit status, locks (evalMu, execMu, dying), shared state                        [2]
+  run.go             Run: exit status, locks (evalMu, execMu), exit claim, shared state                   [2]
   diag.go            errorf/fatalf/exit (close_stdout), writeStderr, debugf, usageFailure, cbyte          [2]  (glibc error, gnulib closeout)
   help.go            --help and --version texts                                                           [2]  (util.c usage, parser.c parse_version)
   predicate.go       Predicate, enums, FuncID, predArgs, apply_predicate                                  [3]  (defs.h, util.c)
@@ -1058,10 +1058,12 @@
 func cbyte(b byte) string { return string([]byte{b}) }
 
 // fatalf is error(EXIT_FAILURE, errnum, …): report, then exit(1) through the
-// same flush-and-close path as a normal exit.
+// same flush-and-close path as a normal exit. It claims the exit before it
+// reports, so only the first fatal error is printed.
 func (r *Run) fatalf(err error, format string, a ...any) {
+	r.claimExit()
 	r.errorf(err, format, a...)
-	r.exit(1)
+	r.exitClaimed(1)
 }
 
 // usageFailure is usage(EXIT_FAILURE): no program prefix on the hint.
@@ -1075,12 +1077,26 @@
 // prints "write error" and forces status 1 (EPIPE included: GNU sets no
 // ignore_EPIPE, and SIGPIPE normally kills the process first).
 //
-// exit is the only way out of the process. The first caller takes dying
-// and never releases it, so a second fatal error on another goroutine
-// blocks instead of printing a second message or exiting with another
-// status (GNU, being single-threaded, dies once).
+// exit is the only way out of the process; see claimExit.
 func (r *Run) exit(status int) {
-	r.dying.Lock()
+	r.claimExit()
+	r.exitClaimed(status)
+}
+
+// claimExit makes the caller the one goroutine that exits the process (GNU,
+// being single-threaded, dies once). The first caller wins. A later caller
+// is on another goroutine, because nothing that runs after a claim dies
+// again, so it blocks until the winner's os.Exit instead of printing a
+// second message or exiting with another status. A write failure inside
+// fatalAt's commit claims first, as GNU would have died at that write.
+func (r *Run) claimExit() {
+	if !r.exiting.CompareAndSwap(false, true) {
+		select {}
+	}
+}
+
+// exitClaimed is exit's work once the caller holds the claim.
+func (r *Run) exitClaimed(status int) {
 	if r.out != nil {
 		r.out.flushFilesQuietly() // Task 7: stdio flushes every FILE at exit
 		if err := r.out.closeAll(); err != nil {
@@ -1100,7 +1116,7 @@
 	os.Exit(status)
 }
 ```
-Add `dying sync.Mutex` to `Run`.
+Add `exiting atomic.Bool` to `Run`.
 
 For this task, add a minimal `Output` in `internal/find/output.go` that buffers stdout through `bufio.NewWriterSize(os.Stdout, 64<<10)`. It exposes:
 - `flushForDiagnostic()`;
@@ -12076,7 +12092,7 @@
 QFIND_CPUPROFILE=.scratch/cpu.out .scratch/qfind-prof /path/to/tree -name '*.go' >/dev/null
 go tool pprof -top .scratch/qfind-prof .scratch/cpu.out | head -30
 ```
-The `QFIND_CPUPROFILE` hook is a few lines in `cmd/qfind/main.go`. When the variable is set, it calls `pprof.StartCPUProfile` and stores `pprof.StopCPUProfile` in an exported `var BeforeExit func()` in `internal/find`. `Run.exit` calls that hook just before `os.Exit`. These are the usual suspects:
+The `QFIND_CPUPROFILE` hook is a few lines in `cmd/qfind/main.go`. When the variable is set, it calls `pprof.StartCPUProfile` and stores `pprof.StopCPUProfile` in an exported `var BeforeExit func()` in `internal/find`. `Run.exitClaimed` calls that hook just before `os.Exit`. These are the usual suspects:
 - lock contention on stdout: commit records in larger batches per entry;
 - `os/user` lookups: these are cached;
 - `Entry` allocations: pool them only if the profile shows it.
````

- [ ] **Step 4: Run the fatal-path check to see it pass**

Run: `.scratch/fixcheck/10-dies-once.sh`
Expected: exit 0:
```
ok: 8 goroutines dying at once print one message (20 runs)
ok: a write failure inside fatalAt's commit exits 1 with its own message
PASS
```

- [ ] **Step 5: Write the failing descriptor check**

**Check `.scratch/fixcheck/11-inherited-fds.sh`**

````bash
#!/bin/bash
# Review issue R: after the plan's recordInheritedFDs and markFDsCloexec, a
# directory opened the way fastwalk opens one (plain open(2), no O_CLOEXEC)
# must be close-on-exec, so that no -exec child inherits it. The plan's own
# functions are compiled against a stand-in for golang.org/x/sys/unix.
set -u
root=/workspace/qfind.master
plan=${PLAN:-$root/_claude/plans/master_plan.md}
d=$root/.scratch/fixcheck/11
rm -rf "$d" && mkdir -p "$d/unix" "$d/walked"
printf 'module fdcheck\n\ngo 1.27\n' > "$d/go.mod"
cat > "$d/unix/unix.go" <<'EOF'
// Package unix stands in for the golang.org/x/sys/unix calls the plan uses.
package unix

import "syscall"

const (
	F_GETFD    = syscall.F_GETFD
	F_SETFD    = syscall.F_SETFD
	FD_CLOEXEC = syscall.FD_CLOEXEC
)

func FcntlInt(fd uintptr, cmd, arg int) (int, error) {
	r, _, errno := syscall.Syscall(syscall.SYS_FCNTL, fd, uintptr(cmd), uintptr(arg))
	if errno != 0 {
		return -1, errno
	}
	return int(r), nil
}
EOF
{
  printf 'package main\n\nimport (\n\t"os"\n\t"strconv"\n\n\t"fdcheck/unix"\n)\n\nvar _, _, _ = os.ReadDir, strconv.Atoi, unix.FcntlInt\n\n'
  awk '/^func \(r \*Run\) (recordInheritedFDs|markFDsCloexec)\(/, /^}/' "$plan"
  awk '/^func openFDs\(/, /^}/' "$plan"
} > "$d/plan.go"
cat > "$d/main.go" <<'EOF'
package main

import (
	"fmt"
	"os"
	"syscall"

	"fdcheck/unix"
)

type Run struct{ inheritedFDs map[int]bool }

func main() {
	r := &Run{}
	r.recordInheritedFDs()
	// fastwalk opens each directory with plain open(2), without O_CLOEXEC.
	fd, err := syscall.Open(os.Args[1], syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
	if err != nil {
		fmt.Println("open:", err)
		os.Exit(2)
	}
	r.markFDsCloexec()
	flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0)
	if err != nil {
		fmt.Println("fcntl:", err)
		os.Exit(2)
	}
	cloexec := flags&unix.FD_CLOEXEC != 0
	fmt.Printf("the walked directory is fd %d, close-on-exec %v\n", fd, cloexec)
	if !cloexec {
		os.Exit(1)
	}
}
EOF
if ! (cd "$d" && go build -o fdcheck . 2>&1); then
  echo "FAIL: the plan's descriptor code does not compile against the stand-ins"
  exit 1
fi
if out=$("$d/fdcheck" "$d/walked"); then
  echo "ok: $out"
  echo PASS
  exit 0
fi
echo "FAIL: $out (an -exec child would inherit it)"
echo FAIL
exit 1
````

- [ ] **Step 6: Run it to see it fail**

Run: `.scratch/fixcheck/11-inherited-fds.sh`
Expected: exit 1: `FAIL: the walked directory is fd 5, close-on-exec false (an -exec child would inherit it)`, then `FAIL`. The fd number depends on what the shell hands down; it is the number `openFDs` read `/proc/self/fd` through.

- [ ] **Step 7: Apply patch 11**

Run: `git apply .scratch/patches/11-only-inherited-fds-stay-open.patch`

**Patch `11-only-inherited-fds-stay-open.patch`**

````diff
--- a/_claude/plans/master_plan.md
+++ b/_claude/plans/master_plan.md
@@ -7271,12 +7271,17 @@
 	return 128 + 6
 }
 
-// recordInheritedFDs remembers the descriptors find started with (fdleak.c
-// remember_non_cloexec_fds). Main calls it first.
+// recordInheritedFDs remembers the descriptors find started with that a
+// child would inherit (fdleak.c remember_non_cloexec_fds). Main calls it
+// first. Close-on-exec and already-closed descriptors are left out, such as
+// the one openFDs read /proc/self/fd through, so a directory that fastwalk
+// later opens under that number is still swept.
 func (r *Run) recordInheritedFDs() {
 	r.inheritedFDs = map[int]bool{}
 	for _, fd := range openFDs() {
-		r.inheritedFDs[fd] = true
+		if flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); err == nil && flags&unix.FD_CLOEXEC == 0 {
+			r.inheritedFDs[fd] = true
+		}
 	}
 }
 
````

- [ ] **Step 8: Run the descriptor check to see it pass**

Run: `.scratch/fixcheck/11-inherited-fds.sh`
Expected: exit 0: `ok: the walked directory is fd 5, close-on-exec true`, then `PASS`.

- [ ] **Step 9: Commit**

```bash
git add _claude/plans/master_plan.md
git commit -m "Fix the qfind plan's fatal-exit and inherited-descriptor code"
```

---

### Task 6: State the audit's real scope, verify everything, and update PR #1 (issue N)

**Files:**
- Modify: `spec/README.md` (L35-37), via patch 12
- Create (git-ignored): `.scratch/fixcheck/12-readme.sh`

**Interfaces:**
- Consumes: Task 5's tree.
- Produces: `spec/README.md` says the audit covered the 18,671 examples that run and skipped 543; `dev` carries six new commits for PR #1.

- [ ] **Step 1: Write the failing README check**

**Check `.scratch/fixcheck/12-readme.sh`**

````bash
#!/bin/bash
# Review issue N: spec/README.md must not claim the runtime audit covered
# all 19,214 examples; it covered the ones that run. The numbers come from
# rspec --dry-run and from the README's own pending table.
set -u
root=/workspace/qfind.master
readme=${README:-$root/spec/README.md}
fail=0
total=$(cd "$root" && bundle exec rspec --dry-run 2>&1 | grep -E '^[0-9]+ examples?, ' | grep -oE '^[0-9]+')
pending=$(awk -F'|' '/^\| Pending \| Reason \|/ { t = 1; next } t && /^\|[ 0-9]+\|/ { s += $2 } t && !/^\|/ { exit } END { print s }' "$readme")
known=$(awk -F'|' '/^\|[ 0-9]+\| Known 4\.11\.0 bugs/ { print $2 + 0 }' "$readme")
skipped=$((pending - known))
ran=$((total - skipped))
group() { LC_NUMERIC=en_US.UTF-8 printf "%'d" "$1"; }
echo "dry run: $total examples; README pending table: $pending, of which $known run as known bugs; skipped: $skipped; run: $ran"
text=$(tr '\n' ' ' < "$readme" | tr -s ' ')
if grep -qF "each of the $(group "$total") examples spawns find" <<<"$text"; then
  echo "FAIL: the README says the audit covered all $(group "$total") examples"
  fail=1
fi
if grep -qF "the $(group "$ran") examples that run spawns find" <<<"$text" && grep -qF "The $skipped skipped examples" <<<"$text"; then
  echo "ok: the README states the audit's real scope ($(group "$ran") run, $skipped skipped)"
else
  echo "FAIL: the README does not say the audit covered the $(group "$ran") examples that run and skipped $skipped"
  fail=1
fi
[ "$fail" -eq 0 ] && echo PASS || echo FAIL
exit "$fail"
````

- [ ] **Step 2: Run it to see it fail**

Run: `.scratch/fixcheck/12-readme.sh`
Expected: exit 1:
```
dry run: 19214 examples; README pending table: 547, of which 4 run as known bugs; skipped: 543; run: 18671
FAIL: the README says the audit covered all 19,214 examples
FAIL: the README does not say the audit covered the 18,671 examples that run and skipped 543
FAIL
```

- [ ] **Step 3: Apply patch 12**

Run: `git apply .scratch/patches/12-readme-audit-scope.patch`

**Patch `12-readme-audit-scope.patch`**

````diff
--- a/spec/README.md
+++ b/spec/README.md
@@ -34,7 +34,8 @@
 
 - **Black box only.** `spec_helper.rb` uses `mock_with :nothing`, so `double`, `allow` and `anything` do
   not exist. Every example reaches find through the helpers below; a runtime audit confirmed that each of
-  the 19,214 examples spawns find at least once.
+  the 18,671 examples that run spawns find at least once. The 543 skipped examples (every pending one
+  except the 4 known bugs, whose bodies run) were not audited.
 - **One expectation per example.** Enforced by rubocop-rspec (`RSpec/MultipleExpectations`). There are
   no compound matchers (`.and`/`.or`).
 - **No comments.** Checked with Ripper over every file in `spec/`. This file is the only place for notes.
````

- [ ] **Step 4: Run the README check to see it pass**

Run: `.scratch/fixcheck/12-readme.sh`
Expected: exit 0: `ok: the README states the audit's real scope (18,671 run, 543 skipped)`, then `PASS`.

- [ ] **Step 5: Commit**

```bash
git add spec/README.md
git commit -m "State the spec audit's real scope in spec/README.md"
```

- [ ] **Step 6: Run every check on the final tree**

```bash
for c in .scratch/fixcheck/[0-9][0-9]-*.sh; do
  printf '%-24s %s\n' "$(basename "$c")" "$(bash "$c" 2>&1 | tail -1)"
done
git diff --stat 2475eee5127e734079ee07ab9b2460e6c0c541b2 -- _claude/plans/master_plan.md spec/README.md
git log --oneline 2475eee5127e734079ee07ab9b2460e6c0c541b2.. -- _claude/plans/master_plan.md spec/README.md
git status --short
```
Expected:
- all 12 checks print `PASS`;
- the diffstat shows `_claude/plans/master_plan.md` with 203 lines changed (121 insertions, 82 deletions) and `spec/README.md` with 2 insertions and 1 deletion;
- the log shows six commits, one per task;
- no uncommitted changes (`.scratch/` is git-ignored).

- [ ] **Step 7: Update PR #1 (ask the user first)**

Pushing updates the open pull request, so confirm with the user before running:
```bash
git push origin dev
```
Expected: `dev` advances on `origin`, and https://github.com/gillisd/qfind/pull/1 shows the six new commits.
