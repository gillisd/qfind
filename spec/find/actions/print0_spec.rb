RSpec.describe "find -print0" do
  context "with a regular file", upstream: "find.gnu/print0.exp" do
    subject(:result) { find("tmp", "-print0") }

    before { sandbox.touch("tmp") }

    it_behaves_like "a successful search printing exactly", "tmp#{0.chr}"
  end

  context "with a directory hierarchy" do
    subject(:result) { find("dir", "-print0") }

    before do
      sandbox.mkdir("dir/sub")
      sandbox.touch("dir/sub/file")
    end

    it "separates the names with null characters" do
      expect(result.records).to contain_exactly("dir", "dir/sub", "dir/sub/file")
    end
  end

  context "with several starting points" do
    subject(:result) { find("b", "a", "-print0") }

    before { sandbox.touch("a", "b") }

    it "prints them in command-line order" do
      expect(result).to print_exactly("b#{0.chr}a#{0.chr}")
    end
  end

  context "when followed by -print" do
    subject(:result) { find("file", "-print0", "-print") }

    before { sandbox.touch("file") }

    it "evaluates to true" do
      expect(result).to print_exactly("file#{0.chr}file\n")
    end
  end

  {
    "a newline" => "a\nb",
    "spaces" => "a b c",
    "quotes" => "a'b\"c",
    "a backslash" => "a\\b",
    "a control character" => "a#{1.chr}b",
    "a byte outside ASCII" => "a#{0xe9.chr}b",
  }.each do |description, name|
    context "with #{description} in the name" do
      subject(:result) { find(name, "-print0") }

      before { sandbox.touch(name) }

      it "prints the exact name" do
        expect(result).to print_exactly("#{name}#{0.chr}")
      end
    end
  end

  context "when standard output is full" do
    subject(:result) { find_with_stdout_to("/dev/full", "file", "-print0") }

    before { sandbox.touch("file") }

    it { is_expected.to have_exit_status(1) }
    it { is_expected.to diagnose("find: 'standard output': No space left on device") }
  end
end
