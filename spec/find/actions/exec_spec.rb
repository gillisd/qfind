RSpec.describe "find -exec" do
  shared_examples "a search passing every file to the command" do
    it { is_expected.to be_successful }

    it "prints every file" do
      expect(result).to print_lines(*names)
    end

    it { is_expected.to diagnose_nothing }
  end

  shared_context "with test commands in a directory on PATH" do
    let(:search_path) { { "PATH" => "#{sandbox.join("bin")}:#{ENV.fetch("PATH")}" } }

    before do
      sandbox.mkdir("bin")
      sandbox.write("bin/tstcmd", "#!/bin/sh\nprintf \"%s\\n\" \"$@\"\n")
      sandbox.write("bin/cntcmd", "#!/bin/sh\necho \"$#\"\n")
      sandbox.chmod(0o755, "bin/tstcmd", "bin/cntcmd")
    end
  end

  it_behaves_like "an action running a command from the starting directory", "-exec"

  context "with a name test selecting one of two directories", upstream: "find.posix/exec-one.exp" do
    subject(:result) { find("tmp", "-name", "fred", "-exec", "echo", "{}", ";") }

    before { sandbox.mkdir("tmp/fred", "tmp/jim") }

    it_behaves_like "a successful search", "tmp/fred"
  end

  context "with -L and a command listing the file", upstream: "find.posix/sv-bug-27563-exec.exp" do
    subject(:result) { find("-L", "tmp", "-name", "yyyy", "-exec", "ls", "{}", ";") }

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/yyyy")
    end

    it_behaves_like "a successful search", "tmp/yyyy"
  end

  context "with a command that succeeds followed by -quit", upstream: "find.gnu/exec-one-rtn-success.exp" do
    subject(:result) { find("tmp", "-exec", "true", ";", "-printf", "yep\n", "-quit") }

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/file")
    end

    it_behaves_like "a successful search", "yep"
  end

  context "with a command that fails followed by -o and -quit", upstream: "find.gnu/exec-one-rtn-fail.exp" do
    subject(:result) { find("tmp", "-exec", "false", ";", "-o", "(", "-printf", "yep\n", "-quit", ")") }

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/file")
    end

    it_behaves_like "a successful search", "yep"
  end

  context "with a command line that succeeds followed by -quit", upstream: "find.gnu/exec-many-rtn-success.exp" do
    subject(:result) { find("tmp", "-exec", "true", "{}", "+", "-printf", "yep\n", "-quit") }

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/file")
    end

    it_behaves_like "a successful search", "yep"
  end

  context "with a command line that fails followed by -quit", upstream: "find.gnu/exec-many-rtn-failure.exp" do
    subject(:result) { find("tmp", "-exec", "false", "{}", "+", "-printf", "yep\n", "-quit") }

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/file")
    end

    it_behaves_like "a failed search", "yep"
  end

  context "with 7200 files searched at -O3", upstream: "find.posix/exec-nogaps.exp" do
    let(:optimisation_level) { 3 }
    let(:names) { (1..7200).map { "tmp/a/#{it}" } }

    before do
      sandbox.mkdir("tmp/a")
      sandbox.touch(*names)
      sandbox.write("tmp/list.sh", "#! /bin/sh\nfor f; do echo \"$f\"; done\n")
      sandbox.chmod(0o500, "tmp/list.sh")
    end

    context "with a command per file" do
      subject(:result) { find("tmp/a/", "-type", "f", "-exec", "echo", "{}", ";") }

      it_behaves_like "a search passing every file to the command"
    end

    context "with a command line for all the files" do
      subject(:result) { find("tmp/a/", "-type", "f", "-exec", "sh", "tmp/list.sh", "{}", "+") }

      it_behaves_like "a search passing every file to the command"
    end
  end

  context "with a command found through PATH", upstream: "tests/find/exec-plus-last-file.sh" do
    subject(:result) { find("bin", "-maxdepth", "0", "-exec", "tstcmd", "{}", "+", env: search_path) }

    include_context "with test commands in a directory on PATH"

    it "runs it" do
      expect(result).to print_exactly("bin\n")
    end
  end

  context "with 3719 file names filling the command line up to the last one",
          upstream: "tests/find/exec-plus-last-file.sh" do
    subject(:result) { find("RashuBug", "-type", "f", "-exec", "tstcmd", "{}", "+", env: search_path) }

    include_context "with test commands in a directory on PATH"

    let(:names) do
      [
        *(1..901).map { format("RashuBug/abcdefghijklmnopqrstuv%04d", it) },
        *(902..3719).map { format("RashuBug/abcdefghijklmnopqrstu%04d", it) },
      ]
    end

    before do
      sandbox.mkdir("RashuBug")
      sandbox.touch(*names)
    end

    it_behaves_like "a search passing every file to the command"

    context "when the command counts its arguments instead" do
      subject(:result) { find("RashuBug", "-type", "f", "-exec", "cntcmd", "{}", "+", env: search_path) }

      it "runs it a second time for the one file left over" do
        expect(result).to print_lines("3718", "1").in_order
      end
    end
  end

  context "with the braces inside a larger argument before a plus sign", upstream: "find.posix/sv-bug-30777.exp" do
    subject(:result) { find("tmp", "-exec", "ls", "FOO{}", "+") }

    before { sandbox.mkdir("tmp") }

    it_behaves_like "a rejected command line", "find: missing argument to `-exec'"
  end

  context "with a plus sign that does not follow a lone pair of braces", upstream: "tests/find/sv-bug-66365-exec.sh" do
    subject(:result) { find(".", "-prune", "-exec", "echo", "x{}", "+", ";") }

    it "passes it on to a command run for each file" do
      expect(result).to print_exactly("x. +\n")
    end
  end

  context "with an argument between the braces and the plus sign" do
    subject(:result) { find("file", "-exec", "echo", "{}", "x", "+") }

    it_behaves_like "a rejected command line", "find: missing argument to `-exec'"
  end

  context "with two lone pairs of braces before the plus sign" do
    subject(:result) { find("file", "-exec", "echo", "{}", "{}", "+") }

    it_behaves_like "a rejected command line", "find: Only one instance of {} is supported with -exec ... +"
  end

  context "with braces inside an earlier argument of a command line" do
    subject(:result) { find("file", "-exec", "echo", "x{}", "{}", "+") }

    it_behaves_like "a rejected command line", "find: Only one instance of {} is supported with -exec ... +"
  end

  context "with a semicolon after the plus sign" do
    subject(:result) { find("file", "-exec", "echo", "{}", "+", ";") }

    it_behaves_like "a rejected command line", "find: paths must precede expression: `;'"
  end

  context "with an environment leaving no room for a command line" do
    subject(:result) do
      find("file", "-exec", "echo", "{}", ";",
           env: { "PADDING" => "x" * 130_000 }, unsetenv_others: true, rlimit_stack: 512 * 1024)
    end

    it_behaves_like "a rejected command line", "find: The environment is too large for exec()."
  end

  context "with data on standard input" do
    subject(:result) { find("file", "-exec", "cat", ";", stdin: "data\n") }

    before { sandbox.touch("file") }

    it "lets the command read it" do
      expect(result).to print_exactly("data\n")
    end
  end

  context "with -print after a command per file" do
    subject(:result) { find("a", "b", "-exec", "echo", "run", "{}", ";", "-print") }

    before { sandbox.touch("a", "b") }

    it "runs the command as each file is visited" do
      expect(result).to print_lines("run a", "a", "run b", "b").in_order
    end
  end

  context "with -print after a command line" do
    subject(:result) { find("a", "b", "-exec", "echo", "run", "{}", "+", "-print") }

    before { sandbox.touch("a", "b") }

    it "runs the command once every file has been visited" do
      expect(result).to print_lines("a", "b", "run a b").in_order
    end
  end

  context "with initial arguments before the braces of a command line" do
    subject(:result) { find("a", "b", "c", "-exec", "echo", "files:", "{}", "+") }

    before { sandbox.touch("a", "b", "c") }

    it "appends the file names after them" do
      expect(result).to print_exactly("files: a b c\n")
    end
  end

  context "with a command line counting its arguments" do
    subject(:result) { find("dir", "-type", "f", "-exec", "./count", "{}", "+") }

    before do
      sandbox.mkdir("dir")
      sandbox.touch(*(1..10).map { "dir/#{it}" })
      sandbox.write("count", "#!/bin/sh\necho \"$#\"\n")
      sandbox.chmod(0o755, "count")
    end

    it "runs the command once for all ten files" do
      expect(result).to print_exactly("10\n")
    end
  end

  context "with a command line for a file below the starting point" do
    subject(:result) { find("dir", "-name", "deep", "-exec", "./where", "{}", "+") }

    before do
      sandbox.mkdir("dir/sub")
      sandbox.touch("dir/sub/deep")
      sandbox.write("where", "#!/bin/sh\npwd -P\n")
      sandbox.chmod(0o755, "where")
    end

    it "runs the command in the directory find was started from" do
      expect(result).to print_lines(sandbox.root.to_s)
    end
  end

  context "with a command line and no matching file" do
    subject(:result) { find("file", "-name", "none", "-exec", "echo", "run", "{}", "+") }

    before { sandbox.touch("file") }

    it "never runs the command" do
      expect(result).to print_nothing
    end
  end

  context "with a command line pending at -quit" do
    subject(:result) { find("file", "-exec", "echo", "run", "{}", "+", "-quit") }

    before { sandbox.touch("file") }

    it "runs the command before exiting" do
      expect(result).to print_exactly("run file\n")
    end
  end

  context "with a command line pending when -ok cannot write its question" do
    subject(:result) do
      find_with_stdin_from("answers", "file", "-exec", "sh", "-c", "touch ran", "sh", "{}", "+", "-ok", "true", ";",
                           err: "/dev/full", env: { "GNU_FINDUTILS_FD_LEAK_CHECK" => nil })
    end

    before do
      sandbox.touch("file")
      sandbox.write("answers", "y\n")
    end

    it "exits without running it" do
      expect { result }.not_to change { sandbox.exist?("ran") }.from(false)
    end
  end

  context "with a command line that fails" do
    subject(:result) { find("a", "b", "-exec", "false", "{}", "+", "-print") }

    before { sandbox.touch("a", "b") }

    it "still evaluates to true" do
      expect(result).to print_lines("a", "b")
    end

    it { is_expected.to have_exit_status(1) }
  end

  context "with a command line naming a command that does not exist" do
    subject(:result) { find("file", "-exec", "no-such-command", "{}", "+") }

    before { sandbox.touch("file") }

    it { is_expected.to diagnose("find: 'no-such-command': No such file or directory") }

    it { is_expected.to have_exit_status(1) }
  end

  context "with a command line killed by a signal" do
    subject(:result) { find("file", "-exec", "sh", "-c", "kill -TERM $$", "sh", "{}", "+") }

    before { sandbox.touch("file") }

    it { is_expected.to diagnose("find: 'sh' terminated by signal 15") }

    it { is_expected.to have_exit_status(1) }
  end

  context "with a command line when no more processes can be created", :unprivileged do
    subject(:result) { find("a", "b", "-exec", "true", "{}", "+", "-print", rlimit_nproc: 0) }

    before { sandbox.touch("a", "b") }

    it { is_expected.to diagnose("find: cannot fork: Resource temporarily unavailable") }

    it { is_expected.to have_exit_status(1) }

    it "has printed every file by the time the command line is due" do
      expect(result).to print_lines("a", "b")
    end
  end
end
