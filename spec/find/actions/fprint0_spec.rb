RSpec.describe "find -fprint0" do
  it_behaves_like "an action writing to an output file", "-fprint0"

  context "with /dev/stdout as the file", upstream: "find.gnu/fprint0_stdout.exp" do
    subject(:result) { find("tmp", "-fprint0", "/dev/stdout") }

    before { sandbox.touch("tmp") }

    it_behaves_like "a successful search printing exactly", "tmp#{0.chr}"
  end

  context "with a regular file" do
    subject(:written) { find("file", "-fprint0", "out").then { sandbox.read("out") } }

    before { sandbox.touch("file") }

    it "writes the name followed by a null character" do
      expect(written).to eq("file#{0.chr}")
    end
  end

  context "with a directory hierarchy" do
    subject(:written) { find("dir", "-fprint0", "out").then { sandbox.read("out") } }

    before do
      sandbox.mkdir("dir/sub")
      sandbox.touch("dir/sub/file")
    end

    it "separates the names with null characters" do
      expect(written.split(0.chr)).to contain_exactly("dir", "dir/sub", "dir/sub/file")
    end
  end

  context "with a newline in a name" do
    subject(:written) { find("a\nb", "-fprint0", "out").then { sandbox.read("out") } }

    before { sandbox.touch("a\nb") }

    it "writes the exact name" do
      expect(written).to eq("a\nb#{0.chr}")
    end
  end

  context "with /dev/stdout as the file and -print" do
    subject(:result) { find("a", "b", "-print", "-fprint0", "/dev/stdout") }

    before { sandbox.touch("a", "b") }

    it "shares standard output with -print" do
      expect(result).to print_exactly("a\na#{0.chr}b\nb#{0.chr}")
    end
  end

  context "with /dev/stderr as the file" do
    subject(:result) { find("a", "missing", "b", "-fprint0", "/dev/stderr") }

    before { sandbox.touch("a", "b") }

    it { is_expected.to print_nothing }

    it "shares standard error with the diagnostics" do
      expect(result).to have_attributes(stderr: "a#{0.chr}find: 'missing': No such file or directory\nb#{0.chr}")
    end
  end

  context "with /dev/stdout as the file when standard output is a terminal" do
    subject(:result) { find_with_terminal_output("a\nb#{1.chr}c", "-fprint0", "/dev/stdout") }

    before { sandbox.touch("a\nb#{1.chr}c") }

    it "still writes the exact name" do
      expect(result).to print_exactly("a\nb#{1.chr}c#{0.chr}")
    end
  end

  context "with -fprint writing to the same file" do
    subject(:written) { find("file", "-fprint0", "out", "-fprint", "out").then { sandbox.read("out") } }

    before { sandbox.touch("file") }

    it "writes both outputs through one stream" do
      expect(written).to eq("file#{0.chr}file\n")
    end
  end
end
