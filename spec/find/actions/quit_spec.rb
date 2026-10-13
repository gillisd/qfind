RSpec.describe "find -quit" do
  context "with -depth and a second starting point", upstream: "find.gnu/quit.exp" do
    subject(:result) { find("tmp/top/one", "tmp/top/two", "-depth", "-print", "-name", "one", "-quit") }

    before { sandbox.mkdir("tmp/top/one/foo", "tmp/top/two") }

    it_behaves_like "a successful search", "tmp/top/one/foo", "tmp/top/one"

    it "stops right after the first match" do
      expect(result).to print_lines("tmp/top/one/foo", "tmp/top/one").in_order
    end
  end

  context "with several starting points" do
    subject(:result) { find("foo", "bar", "-print", "-quit") }

    before { sandbox.mkdir("foo", "bar") }

    it "processes only the first one" do
      expect(result).to print_exactly("foo\n")
    end
  end

  context "with several files matching" do
    subject(:result) { find(".", "-name", "needle", "-print", "-quit") }

    before do
      sandbox.mkdir("haystack/deeper")
      sandbox.touch("haystack/needle", "haystack/deeper/needle")
    end

    it "prints only the first match" do
      expect(result).to print_lines(a_string_ending_with("/needle"))
    end
  end

  context "when it is the only action and so runs before the implicit -print" do
    subject(:result) { find("file", "-quit") }

    before { sandbox.touch("file") }

    it_behaves_like "a successful search"
  end

  context "with a missing starting point that is never reached" do
    subject(:result) { find("file", "missing", "-quit") }

    before { sandbox.touch("file") }

    it_behaves_like "a successful search"
  end

  context "with an error before it is reached" do
    subject(:result) { find("missing", "file", "-quit") }

    before { sandbox.touch("file") }

    it_behaves_like "a failed search"

    it { is_expected.to diagnose("find: 'missing': No such file or directory") }
  end

  context "with a command line pending for -exec ... {} +" do
    subject(:result) { find("a", "b", "stop", "-name", "stop", "-quit", "-o", "-exec", "echo", "{}", "+") }

    before { sandbox.touch("a", "b", "stop") }

    it "runs the command before exiting" do
      expect(result).to print_exactly("a b\n")
    end
  end

  context "with a command line pending for -execdir ... {} +" do
    subject(:result) { find("dir", "-depth", "-name", "dir", "-quit", "-o", "-execdir", "echo", "{}", "+") }

    before do
      sandbox.mkdir("dir")
      sandbox.touch("dir/file")
    end

    it "runs the command before exiting" do
      expect(result).to print_exactly("./file\n")
    end
  end

  context "with a pending command line that fails" do
    subject(:result) { find("a", "stop", "-name", "stop", "-quit", "-o", "-exec", "false", "{}", "+") }

    before { sandbox.touch("a", "stop") }

    it { is_expected.to have_exit_status(1) }
  end

  context "with an output file" do
    subject(:written) { find("a", "b", "-fprint", "out", "-quit").then { sandbox.read("out") } }

    before { sandbox.touch("a", "b") }

    it "flushes the file before exiting" do
      expect(written).to eq("a\n")
    end
  end
end
