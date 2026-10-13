RSpec.describe "find -maxdepth" do
  it_behaves_like "a depth limit option", "-maxdepth"

  context "with 0 as level" do
    subject(:result) { find("top", "-maxdepth", "0") }

    include_context "with files at every depth down to three levels"

    it_behaves_like "a successful search", "top"
  end

  context "with 1 as level" do
    subject(:result) { find("top", "-maxdepth", "1") }

    include_context "with files at every depth down to three levels"

    it_behaves_like "a successful search", "top", "top/file", "top/one"
  end

  context "with 2 as level" do
    subject(:result) { find("top", "-maxdepth", "2") }

    include_context "with files at every depth down to three levels"

    it_behaves_like "a successful search", "top", "top/file", "top/one", "top/one/file", "top/one/deep"
  end

  context "with a level deeper than the tree" do
    subject(:result) { find("top", "-maxdepth", "9") }

    include_context "with files at every depth down to three levels"

    it_behaves_like "a successful search",
                    "top", "top/file", "top/one", "top/one/file", "top/one/deep", "top/one/deep/file"
  end

  context "with 0 as level and files as starting points" do
    subject(:result) { find("a", "b", "-maxdepth", "0") }

    before { sandbox.touch("a", "b") }

    it_behaves_like "a successful search", "a", "b"
  end

  context "with nested starting points" do
    subject(:result) { find("top", "top/one", "-maxdepth", "1") }

    include_context "with files at every depth down to three levels"

    it "counts the levels from each starting point" do
      expect(result).to print_lines("top", "top/file", "top/one", "top/one", "top/one/file", "top/one/deep")
    end
  end

  context "with leading zeros in the level" do
    subject(:result) { find("a", "-maxdepth", "010", "-name", "[kl]") }

    before { sandbox.mkdir("a/b/c/d/e/f/g/h/i/j/k/l") }

    it_behaves_like "a successful search", "a/b/c/d/e/f/g/h/i/j/k"
  end

  context "when given twice" do
    subject(:result) { find("top", "-maxdepth", "0", "-maxdepth", "1") }

    include_context "with files at every depth down to three levels"

    it_behaves_like "a successful search", "top", "top/file", "top/one"
  end

  context "with -mindepth at the same level" do
    subject(:result) { find("top", "-mindepth", "2", "-maxdepth", "2") }

    include_context "with files at every depth down to three levels"

    it_behaves_like "a successful search", "top/one/file", "top/one/deep"
  end

  context "with -mindepth beyond it" do
    subject(:result) { find("top", "-mindepth", "2", "-maxdepth", "1") }

    include_context "with files at every depth down to three levels"

    it_behaves_like "a successful search"
  end

  context "with a directory that cannot be read at the maximum depth", :unprivileged do
    subject(:result) { find("tmp", "-maxdepth", "1") }

    before do
      sandbox.mkdir("tmp/dir")
      sandbox.touch("tmp/dir/file")
      sandbox.chmod(0o000, "tmp/dir")
    end

    it_behaves_like "a successful search", "tmp", "tmp/dir"
  end

  context "with a directory that cannot be read above the maximum depth", :unprivileged do
    subject(:result) { find("tmp", "-maxdepth", "2") }

    before do
      sandbox.mkdir("tmp/dir")
      sandbox.touch("tmp/dir/file")
      sandbox.chmod(0o000, "tmp/dir")
    end

    it_behaves_like "a failed search", "tmp", "tmp/dir"

    it { is_expected.to diagnose("find: 'tmp/dir': Permission denied") }
  end
end
