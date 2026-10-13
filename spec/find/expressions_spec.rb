RSpec.describe "find expressions" do
  context "with an unknown predicate" do
    subject(:result) { find("-foo") }

    it_behaves_like "a rejected command line", "find: unknown predicate `-foo'"
  end

  context "with an unknown predicate after a starting point" do
    subject(:result) { find(".", "-foo") }

    it_behaves_like "a rejected command line", "find: unknown predicate `-foo'"
  end

  context "with an unknown predicate after a test" do
    subject(:result) { find(".", "-name", "a", "-foo") }

    it_behaves_like "a rejected command line", "find: unknown predicate `-foo'"
  end

  ["-noop", "---noop"].each do |predicate|
    context "with the internal #{predicate} pseudo-option", upstream: "tests/find/refuse-noop.sh" do
      subject(:result) { find(predicate) }

      it_behaves_like "a rejected command line", "find: unknown predicate `#{predicate}'"
    end
  end

  context "with an unquoted pattern expanded into several existing file names" do
    subject(:result) { find(".", "-name", "frcode.c", "locate.c", "word_io.c", "-print") }

    before { sandbox.touch("frcode.c", "locate.c", "word_io.c") }

    it_behaves_like "a rejected command line", "find: paths must precede expression: `locate.c'"

    it { is_expected.to diagnose("find: possible unquoted pattern after predicate `-name'?") }
  end

  context "with a name that does not exist after the expression" do
    subject(:result) { find(".", "-name", "x.c", "missing.c") }

    it_behaves_like "a rejected command line", "find: paths must precede expression: `missing.c'"

    it "does not suspect an unquoted pattern" do
      expect(result).not_to diagnose(a_string_starting_with("find: possible unquoted pattern"))
    end
  end

  context "with an existing file after an action" do
    subject(:result) { find(".", "-print", "file") }

    before { sandbox.touch("file") }

    it_behaves_like "a rejected command line", "find: paths must precede expression: `file'"

    it { is_expected.to diagnose("find: possible unquoted pattern after predicate `-print'?") }
  end

  context "with surplus closing parentheses after the expression" do
    subject(:result) { find(".", "-print", ")", ")") }

    it_behaves_like "a rejected command line", "find: you have too many ')'"
  end

  context "without an expression" do
    subject(:result) { find("dir") }

    before do
      sandbox.mkdir("dir")
      sandbox.touch("dir/file")
    end

    it_behaves_like "a successful search", "dir", "dir/file"
  end

  context "with tests but no action" do
    subject(:result) { find("dir", "-name", "a", "-o", "-name", "b") }

    before do
      sandbox.mkdir("dir")
      sandbox.touch("dir/a", "dir/b", "dir/c")
    end

    it_behaves_like "a successful search", "dir/a", "dir/b"
  end

  context "with -prune as the only action" do
    subject(:result) { find("dir", "-name", "sub", "-prune") }

    before do
      sandbox.mkdir("dir/sub")
      sandbox.touch("dir/sub/file")
    end

    it_behaves_like "a successful search", "dir/sub"
  end

  context "with an action other than -prune" do
    subject(:result) { find("dir", "-name", "a", "-printf", "found %f\n") }

    before do
      sandbox.mkdir("dir")
      sandbox.touch("dir/a", "dir/b")
    end

    it_behaves_like "a successful search", "found a"
  end
end
