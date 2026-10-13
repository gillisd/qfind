RSpec.describe "find -fprint" do
  it_behaves_like "an action writing to an output file", "-fprint"

  context "with /dev/stdout as the file", upstream: "find.gnu/print_stdout.exp" do
    subject(:result) { find("tmp", "-fprint", "/dev/stdout") }

    before { sandbox.touch("tmp") }

    it_behaves_like "a successful search", "tmp"
  end

  context "with a read-only file inside the searched directory", :unprivileged,
          upstream: "find.gnu/fprint-unwritable.exp" do
    subject(:result) { find("tmp", "-fprint", "tmp/unwritable") }

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/unwritable")
      sandbox.chmod(0o444, "tmp/unwritable")
    end

    it_behaves_like "a rejected command line", "find: 'tmp/unwritable': Permission denied"
  end

  context "with a regular file" do
    subject(:written) { find("file", "-fprint", "out").then { sandbox.read("out") } }

    before { sandbox.touch("file") }

    it "writes the name followed by a newline" do
      expect(written).to eq("file\n")
    end
  end

  context "with a directory hierarchy" do
    subject(:written) { find("dir", "-fprint", "out").then { sandbox.read("out") } }

    before do
      sandbox.mkdir("dir/sub")
      sandbox.touch("dir/sub/file")
    end

    it "writes every name on a line of its own" do
      expect(written.lines).to contain_exactly("dir\n", "dir/sub\n", "dir/sub/file\n")
    end
  end

  context "with the file inside the searched directory" do
    subject(:written) { find(".", "-fprint", "out").then { sandbox.read("out") } }

    it "lists the file itself because it is created before the search starts" do
      expect(written).to eq(".\n./out\n")
    end
  end

  context "with unusual characters in a name" do
    subject(:written) { find("a\nb\tc#{1.chr}d", "-fprint", "out").then { sandbox.read("out") } }

    before { sandbox.touch("a\nb\tc#{1.chr}d") }

    it "writes the name as-is" do
      expect(written).to eq("a\nb\tc#{1.chr}d\n")
    end
  end

  context "with /dev/stdout as the file and -print" do
    subject(:result) { find("a", "b", "-print", "-fprint", "/dev/stdout") }

    before { sandbox.touch("a", "b") }

    it "shares standard output with -print" do
      expect(result).to print_exactly("a\na\nb\nb\n")
    end
  end

  context "with /dev/stderr as the file" do
    subject(:result) { find("a", "missing", "b", "-fprint", "/dev/stderr") }

    before { sandbox.touch("a", "b") }

    it { is_expected.to print_nothing }

    it "shares standard error with the diagnostics" do
      expect(result).to have_attributes(stderr: "a\nfind: 'missing': No such file or directory\nb\n")
    end
  end

  context "with -fprint0 writing to the same file" do
    subject(:written) { find("file", "-fprint", "out", "-fprint0", "out").then { sandbox.read("out") } }

    before { sandbox.touch("file") }

    it "writes both outputs through one stream" do
      expect(written).to eq("file\nfile#{0.chr}")
    end
  end

  context "with another -fprint writing to a symbolic link to the same file" do
    subject(:written) { find("file", "-fprint", "out", "-fprint", "alias").then { sandbox.read("out") } }

    before do
      sandbox.touch("file", "out")
      sandbox.symlink("out", "alias")
    end

    it "recognises the file and writes both outputs through one stream" do
      expect(written).to eq("file\nfile\n")
    end
  end
end
