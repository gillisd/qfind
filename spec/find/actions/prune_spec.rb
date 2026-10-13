RSpec.describe "find -prune" do
  context "with a directory skipped among its siblings", upstream: "find.posix/prune.exp" do
    subject(:result) { find("tmp", "-name", "jim", "-prune", "-o", "-print") }

    before { sandbox.mkdir("tmp/fred", "tmp/jim/1", "tmp/shiela") }

    it_behaves_like "a successful search", "tmp", "tmp/fred", "tmp/shiela"
  end

  context "with -depth", upstream: "find.posix/prune-result.exp" do
    subject(:result) { find("tmp", "-depth", "-name", "a", "-prune", "-o", "-print") }

    before do
      sandbox.mkdir("tmp/a")
      sandbox.touch("tmp/a/f")
    end

    it_behaves_like "a successful search", "tmp", "tmp/a/f"
  end

  context "with a regular file matched", upstream: "find.posix/prune-stat.exp" do
    subject(:result) { find("tmp", "-name", "b", "-prune", "-o", "-print") }

    before do
      sandbox.mkdir("tmp/a", "tmp/c")
      sandbox.touch("tmp/b")
    end

    it_behaves_like "a successful search", "tmp", "tmp/a", "tmp/c"
  end

  context "with a pruned directory that cannot be searched", upstream: "find.posix/sv-bug-11175.exp" do
    subject(:result) { find("tmp", "-name", "noxdir", "-prune", "-type", "d", "-print") }

    before do
      sandbox.mkdir("tmp/noxdir")
      sandbox.chmod(0o700, "tmp")
      sandbox.chmod(0o600, "tmp/noxdir")
    end

    it_behaves_like "a successful search", "tmp/noxdir"
  end

  context "with a pruned directory that cannot be read", :unprivileged do
    subject(:result) { find("tmp", "-name", "locked", "-prune", "-o", "-print") }

    before do
      sandbox.mkdir("tmp/locked")
      sandbox.touch("tmp/locked/inside")
      sandbox.chmod(0o000, "tmp/locked")
    end

    it_behaves_like "a successful search", "tmp"
  end

  context "with a starting point" do
    subject(:result) { find("dir", "-prune", "-print") }

    before do
      sandbox.mkdir("dir/sub")
      sandbox.touch("dir/file")
    end

    it "evaluates to true without descending into it" do
      expect(result).to print_exactly("dir\n")
    end
  end

  context "with a path to skip" do
    subject(:result) { find(".", "-path", "./src/emacs", "-prune", "-o", "-print") }

    before do
      sandbox.mkdir("src/emacs/lisp", "src/vim")
      sandbox.touch("src/emacs/README", "src/emacs/lisp/simple.el", "src/vim/README", "src/Makefile")
    end

    it_behaves_like "a successful search", ".", "./src", "./src/Makefile", "./src/vim", "./src/vim/README"
  end

  context "with projects found by their version control directories" do
    subject(:result) do
      find("repo/", "(", "-exec", "test", "-d", "{}/.svn", ";", "-or", "-exec", "test", "-d", "{}/.git", ";",
           "-or", "-exec", "test", "-d", "{}/CVS", ";", ")", "-print", "-prune")
    end

    before do
      sandbox.mkdir("repo/project1/CVS", "repo/gnu/project2/.svn", "repo/gnu/project3/.svn",
                    "repo/gnu/project3/src/.svn", "repo/project4/.git")
    end

    it_behaves_like "a successful search", "repo/project1", "repo/gnu/project2", "repo/gnu/project3", "repo/project4"
  end

  context "with -print0 on the other side of -o" do
    subject(:result) { find(".", "-name", ".snapshot", "-prune", "-o", "(", "!", "-name", "*~", "-print0", ")") }

    before do
      sandbox.mkdir(".snapshot/old", "backup~")
      sandbox.touch(".snapshot/old/file", "backup~/kept", "file", "file~")
    end

    it "omits pruned directories entirely but still visits the contents of other excluded directories" do
      expect(result.records).to contain_exactly(".", "./file", "./backup~/kept")
    end
  end
end
