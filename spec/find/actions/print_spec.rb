RSpec.describe "find -print" do
  context "with a directory hierarchy" do
    subject(:result) { find("dir", "-print") }

    before do
      sandbox.mkdir("dir/sub")
      sandbox.touch("dir/sub/file")
    end

    it_behaves_like "a successful search", "dir", "dir/sub", "dir/sub/file"
  end

  context "with a starting point ending in a slash" do
    subject(:result) { find("dir/", "-print") }

    before do
      sandbox.mkdir("dir/sub")
      sandbox.touch("dir/sub/file")
    end

    it_behaves_like "a successful search", "dir/", "dir/sub", "dir/sub/file"
  end

  context "with an absolute starting point" do
    subject(:result) { find(sandbox.join("dir").to_s, "-print") }

    before do
      sandbox.mkdir("dir")
      sandbox.touch("dir/file")
    end

    it "prints absolute names" do
      expect(result).to print_lines("#{sandbox.root}/dir", "#{sandbox.root}/dir/file")
    end
  end

  context "with a single file" do
    subject(:result) { find("file", "-print") }

    before { sandbox.touch("file") }

    it "prints the name followed by a newline" do
      expect(result).to print_exactly("file\n")
    end
  end

  context "with several starting points" do
    subject(:result) { find("b", "a", "-print") }

    before { sandbox.touch("a", "b") }

    it "prints them in command-line order" do
      expect(result).to print_lines("b", "a").in_order
    end
  end

  context "when followed by another -print" do
    subject(:result) { find("file", "-print", "-print") }

    before { sandbox.touch("file") }

    it "evaluates to true" do
      expect(result).to print_exactly("file\nfile\n")
    end
  end

  {
    "a newline" => "a\nb",
    "a tab" => "a\tb",
    "a carriage return" => "a\rb",
    "a control character" => "a#{1.chr}b",
    "a delete character" => "a#{127.chr}b",
    "a byte outside ASCII" => "a#{0xe9.chr}b",
  }.each do |description, name|
    context "with #{description} in the name" do
      subject(:result) { find(name, "-print") }

      before { sandbox.touch(name) }

      it "prints the name as-is to a pipe" do
        expect(result).to print_exactly("#{name}\n")
      end
    end
  end

  context "when standard output is full" do
    subject(:result) { find_with_stdout_to("/dev/full", "file", "-print") }

    before { sandbox.touch("file") }

    it { is_expected.to have_exit_status(1) }
    it { is_expected.to diagnose("find: 'standard output': No space left on device") }
  end

  context "without an explicit action" do
    subject(:result) { find("dir") }

    before do
      sandbox.mkdir("dir/sub")
      sandbox.touch("dir/sub/file")
    end

    it_behaves_like "a successful search", "dir", "dir/sub", "dir/sub/file"
  end

  context "with tests joined by -o and no action" do
    subject(:result) { find("a", "b", "-name", "a", "-o", "-name", "b") }

    before { sandbox.touch("a", "b") }

    it_behaves_like "a successful search", "a", "b"
  end

  context "with an explicit -print on one side of -o" do
    subject(:result) { find("a", "b", "-name", "a", "-o", "-name", "b", "-print") }

    before { sandbox.touch("a", "b") }

    it_behaves_like "a successful search", "b"
  end

  context "with -prune as the only action", upstream: "find.gnu/prune-default-print.exp" do
    subject(:result) { find("tmp", "-prune") }

    before { sandbox.mkdir("tmp") }

    it_behaves_like "a successful search", "tmp"
  end

  context "with -quit in a branch that never runs" do
    subject(:result) { find("file", "-true", "-o", "-quit") }

    before { sandbox.touch("file") }

    it_behaves_like "a successful search", "file"
  end

  context "with another action in a branch that never runs" do
    [
      ["-delete"],
      ["-exec", "true", ";"],
      ["-exec", "true", "{}", "+"],
      ["-execdir", "true", ";"],
      ["-execdir", "true", "{}", "+"],
      ["-fls", "out"],
      ["-fprint", "out"],
      ["-fprint0", "out"],
      ["-fprintf", "out", "%p"],
      ["-ls"],
      ["-ok", "true", ";"],
      ["-okdir", "true", ";"],
      ["-print0"],
      ["-printf", "%p"],
      ["-print"],
    ].each do |action|
      context "with #{action.join(" ")}" do
        subject(:result) { find("file", "-true", "-o", *action) }

        before { sandbox.touch("file") }

        it_behaves_like "a successful search"
      end
    end
  end
end
