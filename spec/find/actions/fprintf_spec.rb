RSpec.describe "find -fprintf" do
  it_behaves_like "an action writing to an output file", "-fprintf", "%p\n"

  context "without a format" do
    subject(:result) { find("file", "-fprintf", "out") }

    before { sandbox.touch("file") }

    it_behaves_like "a rejected command line", "find: invalid argument `out' to `-fprintf'"

    it "does not create the file" do
      expect { result }.not_to change { sandbox.exist?("out") }.from(false)
    end
  end

  context "with a format of directives and escapes" do
    subject(:written) { find("file", "-fprintf", "out", "%s %m %y\t%p\n").then { sandbox.read("out") } }

    before do
      sandbox.write("file", "x" * 42)
      sandbox.chmod(0o640, "file")
    end

    it "writes the format interpreted as -printf does" do
      expect(written).to eq("42 640 f\tfile\n")
    end
  end

  context "with a format without a newline" do
    subject(:written) { find("a", "b", "-fprintf", "out", "%p").then { sandbox.read("out") } }

    before { sandbox.touch("a", "b") }

    it "writes no separator between files" do
      expect(written).to eq("ab")
    end
  end

  context "with the file inside the searched directory" do
    subject(:written) { find(".", "-fprintf", "out", "%p\n").then { sandbox.read("out") } }

    it "lists the file itself because it is created before the search starts" do
      expect(written).to eq(".\n./out\n")
    end
  end

  context "with unusual characters in a name" do
    subject(:written) { find("a\nb\tc#{1.chr}d", "-fprintf", "out", "%p\n").then { sandbox.read("out") } }

    before { sandbox.touch("a\nb\tc#{1.chr}d") }

    it "writes the name as-is" do
      expect(written).to eq("a\nb\tc#{1.chr}d\n")
    end
  end

  context "with /dev/stdout as the file" do
    subject(:result) { find("a", "b", "-fprintf", "/dev/stdout", "<%p>\n") }

    before { sandbox.touch("a", "b") }

    it_behaves_like "a successful search", "<a>", "<b>"
  end

  context "with /dev/stdout as the file and -print on both sides" do
    subject(:result) { find("a", "-print", "-fprintf", "/dev/stdout", "<%p>\n", "-print") }

    before { sandbox.touch("a") }

    it "shares standard output with -print" do
      expect(result).to print_exactly("a\n<a>\na\n")
    end
  end

  context "with /dev/stdout as the file when standard output is full" do
    subject(:result) { find_with_stdout_to("/dev/full", "a", "-fprintf", "/dev/stdout", "%p\n") }

    before { sandbox.touch("a") }

    it { is_expected.to have_exit_status(1) }
    it { is_expected.to diagnose("find: 'standard output': No space left on device") }
  end

  context "with /dev/stdout as the file when standard output is a terminal" do
    subject(:result) { find_with_terminal_output("bell#{7.chr}name", "-fprintf", "/dev/stdout", "%p\n") }

    before { sandbox.touch("bell#{7.chr}name") }

    it_behaves_like "a successful search", "bell?name"
  end

  context "with /dev/stderr as the file" do
    subject(:result) { find("a", "missing", "b", "-fprintf", "/dev/stderr", "<%p>\n") }

    before { sandbox.touch("a", "b") }

    it { is_expected.to print_nothing }

    it "shares standard error with the diagnostics" do
      expect(result).to have_attributes(stderr: "<a>\nfind: 'missing': No such file or directory\n<b>\n")
    end
  end

  context "with -print ahead of a file that cannot be written", :unprivileged do
    subject(:result) { find("file", "-print", "-fprintf", "out", "%p\n") }

    before do
      sandbox.touch("file", "out")
      sandbox.chmod(0o444, "out")
    end

    it_behaves_like "a rejected command line", "find: 'out': Permission denied"
  end

  context "with -fprint writing to the same file" do
    subject(:written) { find("file", "-fprint", "out", "-fprintf", "out", "<%p>\n").then { sandbox.read("out") } }

    before { sandbox.touch("file") }

    it "writes both outputs through one stream" do
      expect(written).to eq("file\n<file>\n")
    end
  end

  context "with another -fprintf writing to the same file through a symbolic link",
          upstream: "find.gnu/fprintf-samefile.exp" do
    subject(:result) { find("tmp/top", "-fprintf", "tmp/file", "1: %p\n", "-fprintf", "tmp/same", "2: %p\n") }

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/file", "tmp/top")
      sandbox.symlink("file", "tmp/same")
    end

    it_behaves_like "a successful search"

    it "writes both lines to the file" do
      expect { result }.to change { sandbox.read("tmp/file") }.from("").to("1: tmp/top\n2: tmp/top\n")
    end
  end
end
