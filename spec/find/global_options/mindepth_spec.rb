RSpec.describe "find -mindepth" do
  it_behaves_like "a depth limit option", "-mindepth"

  context "with 0 as level" do
    subject(:result) { find("top", "-mindepth", "0") }

    include_context "with files at every depth down to three levels"

    it_behaves_like "a successful search",
                    "top", "top/file", "top/one", "top/one/file", "top/one/deep", "top/one/deep/file"
  end

  context "with 1 as level" do
    subject(:result) { find("top", "-mindepth", "1") }

    include_context "with files at every depth down to three levels"

    it_behaves_like "a successful search", "top/file", "top/one", "top/one/file", "top/one/deep", "top/one/deep/file"
  end

  context "with the depth of the deepest file as level" do
    subject(:result) { find("top", "-mindepth", "3") }

    include_context "with files at every depth down to three levels"

    it_behaves_like "a successful search", "top/one/deep/file"
  end

  context "with a level deeper than the tree" do
    subject(:result) { find("top", "-mindepth", "4") }

    include_context "with files at every depth down to three levels"

    it_behaves_like "a successful search"
  end

  context "with 1 as level and files as starting points" do
    subject(:result) { find("a", "b", "-mindepth", "1") }

    before { sandbox.touch("a", "b") }

    it_behaves_like "a successful search"
  end

  context "with nested starting points" do
    subject(:result) { find("top", "top/one", "-mindepth", "2") }

    include_context "with files at every depth down to three levels"

    it "counts the levels from each starting point" do
      expect(result).to print_lines("top/one/file", "top/one/deep", "top/one/deep/file", "top/one/deep/file")
    end
  end

  context "with leading zeros in the level" do
    subject(:result) { find("a", "-mindepth", "010") }

    before { sandbox.mkdir("a/b/c/d/e/f/g/h/i/j/k/l") }

    it_behaves_like "a successful search", "a/b/c/d/e/f/g/h/i/j/k", "a/b/c/d/e/f/g/h/i/j/k/l"
  end

  context "when given twice" do
    subject(:result) { find("top", "-mindepth", "3", "-mindepth", "2") }

    include_context "with files at every depth down to three levels"

    it_behaves_like "a successful search", "top/one/file", "top/one/deep", "top/one/deep/file"
  end

  context "with an action that would stop the descent" do
    subject(:result) { find("top", "-mindepth", "1", "-prune") }

    include_context "with files at every depth down to three levels"

    it "applies it only from the minimum depth on" do
      expect(result).to print_lines("top/file", "top/one")
    end
  end

  context "with -depth" do
    subject(:result) { find("tree", "-depth", "-mindepth", "1") }

    before do
      sandbox.mkdir("tree/branch")
      sandbox.touch("tree/branch/leaf")
    end

    it "still lists the contents of each directory first" do
      expect(result).to print_lines("tree/branch/leaf", "tree/branch").in_order
    end
  end

  context "with a directory that cannot be read above the minimum depth", :unprivileged do
    subject(:result) { find("tmp", "-mindepth", "3") }

    before do
      sandbox.mkdir("tmp/dir", "tmp/other/sub")
      sandbox.touch("tmp/other/sub/file")
      sandbox.chmod(0o000, "tmp/dir")
    end

    it_behaves_like "a failed search", "tmp/other/sub/file"

    it "still searches it" do
      expect(result).to diagnose("find: 'tmp/dir': Permission denied")
    end
  end
end
