RSpec.describe "find -execdir" do
  include_context "with a directory-reporting command on a secure PATH"

  let(:env) { { "PATH" => secure_path } }

  it_behaves_like "an action running a command from the directory of each file", "-execdir"
  it_behaves_like "an action refusing an insecure PATH", "-execdir"

  context "with a pattern naming one of two sibling directories", upstream: "find.gnu/execdir-one.exp" do
    subject(:result) { find("tmp", "-name", "fred", "-execdir", "echo", "{}", ";", env:) }

    before { sandbox.mkdir("tmp/fred", "tmp/jim") }

    it_behaves_like "a successful search", "./fred"

    context "without the descriptor leak check", upstream: "find.gnu/no-fdleak-test.exp" do
      let(:env) { super().merge("GNU_FINDUTILS_FD_LEAK_CHECK" => nil) }

      it_behaves_like "a successful search", "./fred"
    end
  end

  context "with the braces inside a larger argument", upstream: "find.gnu/sv-bug-17782.exp" do
    subject(:result) { find("tmp", "-name", "foo", "-execdir", "echo", "saw {}", ";", env:) }

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/foo")
    end

    it_behaves_like "a successful search", "saw ./foo"
  end

  context "with -L", upstream: "find.gnu/sv-bug-27563-execdir.exp" do
    subject(:result) { find("-L", "tmp", "-name", "yyyy", "-execdir", "ls", "{}", ";", env:) }

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/yyyy")
    end

    it_behaves_like "a successful search", "./yyyy"
  end

  context "with an empty script next to the matched file" do
    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/foo")
      sandbox.chmod(0o755, "tmp/foo")
    end

    context "with the ; terminator", upstream: "find.gnu/execdir-pwd1.exp" do
      subject(:result) { find("tmp", "-name", "foo", "-execdir", "sh", "./foo", "{}", ";", env:) }

      it_behaves_like "a successful search"
    end

    context "with the + terminator", upstream: "find.gnu/execdir-pwd.exp" do
      subject(:result) { find("tmp", "-name", "foo", "-execdir", "sh", "./foo", "{}", "+", env:) }

      it_behaves_like "a successful search"
    end
  end

  context "with a script and four empty files in each of six directories", upstream: "find.gnu/execdir-multiple.exp" do
    let(:script) do
      <<~SH
        #! /bin/sh
        set -e
        here=`pwd`
        d=`basename $here`

        for arg; do
          echo "$d" "$arg"
        done | LC_ALL=C sort
      SH
    end

    before do
      ("a".."f").each do |directory|
        sandbox.mkdir("tmp/#{directory}")
        sandbox.write("tmp/#{directory}/runme", script)
        sandbox.chmod(0o700, "tmp/#{directory}/runme")
        sandbox.touch("tmp/#{directory}/one", "tmp/#{directory}/two", "tmp/#{directory}/three",
                      "tmp/#{directory}/four")
      end
    end

    ["+", ";"].each do |terminator|
      context "with the #{terminator} terminator" do
        subject(:result) { find("tmp", "-type", "f", "-empty", "-execdir", "sh", "./runme", "{}", terminator, env:) }

        it_behaves_like "a successful search",
                        *("a".."f").to_a.product(["one", "two", "three", "four"]).map { |dir, file| "#{dir} ./#{file}" }
      end
    end
  end

  context "with a limit of 30 open file descriptors", upstream: "tests/find/execdir-fd-leak.sh" do
    subject(:result) { find(".", "-type", "f", "-execdir", "cat", "{}", ";", env:, rlimit_nofile: 30) }

    before do
      sandbox.mkdir("one", "two")
      (3..100).map { format("%03d", it) }.each { sandbox.touch(it, "one/#{it}", "two/#{it}") }
    end

    it "runs the command for every file without leaking a descriptor per directory" do
      expect(result).to be_successful
    end
  end

  context "with a command reading standard input" do
    subject(:result) { find("tmp", "-name", "fred", "-execdir", "cat", ";", env:, stdin: "hello\n") }

    before { sandbox.mkdir("tmp/fred") }

    it "passes the standard input of find on to the command" do
      expect(result).to print_lines("hello")
    end
  end

  context "with the + terminator" do
    context "with a hierarchy", upstream: "find.gnu/execdir-hier.exp" do
      subject(:result) { find("tmp", "-execdir", "echo", "{}", "+", env:) }

      before { sandbox.mkdir("tmp/two") }

      it_behaves_like "a successful search", "./tmp", "./two"
    end

    context "with the root directory", upstream: "find.gnu/execdir-root-only.exp" do
      subject(:result) { find("/", "-mindepth", "0", "-maxdepth", "0", "-execdir", "echo", "==", "{}", "+", env:) }

      it_behaves_like "a successful search", "== /"
    end

    context "with nothing matched", upstream: "find.gnu/sv-bug-12230.exp" do
      subject(:result) { find("tmp", "-false", "-execdir", "echo", "==", "{}", "+", env:) }

      before { sandbox.mkdir("tmp") }

      it_behaves_like "a successful search"
    end

    context "with an unreadable directory left unvisited", :unprivileged,
            upstream: "find.gnu/execdir-in-unreadable.exp" do
      subject(:result) { find("tmp", "-maxdepth", "1", "-execdir", "true", "{}", "+", env:) }

      before do
        sandbox.mkdir("tmp/sub")
        sandbox.chmod(0o755, "tmp")
        sandbox.chmod(0o300, "tmp/sub")
      end

      it_behaves_like "a successful search"
    end

    context "with files in two directories" do
      subject(:result) { find("tmp", "-type", "f", "-execdir", "report", "{}", "+", env:) }

      before do
        sandbox.mkdir("tmp/a", "tmp/b")
        sandbox.touch("tmp/a/one", "tmp/a/two", "tmp/b/three", "tmp/b/four")
      end

      it "runs the command once per directory on the files of that directory" do
        expect(result).to print_lines("./tmp/a: ./one ./two", "./tmp/b: ./four ./three")
      end
    end

    context "with starting points in two directories" do
      subject(:result) { find("a", "sub/b", "-execdir", "report", "{}", "+", env:) }

      before do
        sandbox.mkdir("sub")
        sandbox.touch("a", "sub/b")
      end

      it "runs the command in each directory on its own starting point" do
        expect(result).to print_lines(".: ./a", "./sub: ./b")
      end
    end

    context "with two starting points in the same directory" do
      subject(:result) { find("a", "b", "-execdir", "report", "{}", "+", env:) }

      before { sandbox.touch("a", "b") }

      it "runs the command separately for each starting point" do
        expect(result).to print_lines(".: ./a", ".: ./b")
      end
    end

    context "with more files in one directory than fit on one command line" do
      subject(:result) { find("tmp", "-type", "f", "-execdir", "echo", "{}", "+", env:) }

      let(:names) { Array.new(3_000) { format("%0100d", it) } }

      before do
        sandbox.mkdir("tmp")
        sandbox.touch(*names.map { "tmp/#{it}" })
      end

      it "runs the command several times" do
        expect(result.lines.size).to be > 1
      end

      it "passes every file exactly once" do
        expect(result.stdout.split).to match_array(names.map { "./#{it}" })
      end
    end

    context "with a command that fails" do
      subject(:result) { find("tmp", "-execdir", "false", "{}", "+", "-print", env:) }

      before { sandbox.mkdir("tmp/fred") }

      it "evaluates to true" do
        expect(result).to print_lines("tmp", "tmp/fred")
      end

      it "makes find exit with a failure status" do
        expect(result).to have_exit_status(1)
      end
    end

    context "with a command that does not exist" do
      subject(:result) { find("tmp", "-execdir", "no-such-command", "{}", "+", env:) }

      before { sandbox.mkdir("tmp") }

      it { is_expected.to have_exit_status(1) }
      it { is_expected.to diagnose("find: 'no-such-command': No such file or directory") }
    end

    context "with a command killed by a signal" do
      subject(:result) { find("tmp", "-execdir", "sh", "-c", "kill -9 $$", "sh", "{}", "+", env:) }

      before { sandbox.mkdir("tmp") }

      it { is_expected.to have_exit_status(1) }
      it { is_expected.to diagnose("find: 'sh' terminated by signal 9") }
    end

    context "with the braces inside a larger argument before the +" do
      subject(:result) { find("tmp", "-name", "fred", "-execdir", "echo", "x{}", "+", ";", env:) }

      before { sandbox.mkdir("tmp/fred") }

      it_behaves_like "a successful search", "x./fred +"
    end

    context "with another argument between the braces and the +" do
      subject(:result) { find("tmp", "-execdir", "echo", "{}", "x", "+", env:) }

      it_behaves_like "a rejected command line", "find: missing argument to `-execdir'"
    end

    context "with the braces twice" do
      subject(:result) { find("tmp", "-execdir", "echo", "{}", "{}", "+", env:) }

      it_behaves_like "a rejected command line", "find: Only one instance of {} is supported with -execdir ... +"
    end

    context "with a semicolon following the +" do
      subject(:result) { find("tmp", "-execdir", "echo", "{}", "+", ";", env:) }

      it_behaves_like "a rejected command line", "find: paths must precede expression: `;'"
    end
  end
end
