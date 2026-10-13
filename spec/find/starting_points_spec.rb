RSpec.describe "find starting points" do
  context "without starting points", upstream: "tests/find/files0-from.sh" do
    subject(:result) { find("-maxdepth", "0") }

    it_behaves_like "a successful search", "."
  end

  context "without starting points or expression" do
    subject(:result) { find }

    before { sandbox.touch("file") }

    it_behaves_like "a successful search", ".", "./file"
  end

  context "with an expression opening with a parenthesis" do
    subject(:result) { find("(", "-name", "file", ")") }

    before { sandbox.touch("file") }

    it_behaves_like "a successful search", "./file"
  end

  context "with an expression opening with a negation" do
    subject(:result) { find("!", "-name", "file") }

    before { sandbox.touch("file") }

    it_behaves_like "a successful search", "."
  end

  context "with several files as starting points" do
    subject(:result) { find("c", "a", "b") }

    before { sandbox.touch("a", "b", "c") }

    it "processes them in command-line order" do
      expect(result).to print_lines("c", "a", "b").in_order
    end
  end

  context "with several directories as starting points" do
    subject(:result) { find("two", "one") }

    before do
      sandbox.mkdir("one", "two")
      sandbox.touch("one/file", "two/file")
    end

    it "searches each one completely before the next" do
      expect(result).to print_lines("two", "two/file", "one", "one/file").in_order
    end
  end

  context "with the same starting point twice" do
    subject(:result) { find("a", "a") }

    before { sandbox.touch("a") }

    it_behaves_like "a successful search", "a", "a"
  end

  context "with a starting point that does not exist" do
    subject(:result) { find("a", "missing", "b") }

    before { sandbox.touch("a", "b") }

    it_behaves_like "a failed search", "a", "b"

    it { is_expected.to diagnose("find: 'missing': No such file or directory") }

    it "carries on with the remaining starting points in order" do
      expect(result).to print_lines("a", "b").in_order
    end
  end

  context "with an empty starting point" do
    subject(:result) { find("", "a") }

    before { sandbox.touch("a") }

    it_behaves_like "a failed search", "a"

    it { is_expected.to diagnose("find: '': No such file or directory") }
  end

  context "with a trailing slash" do
    subject(:result) { find("dir/") }

    before do
      sandbox.mkdir("dir")
      sandbox.touch("dir/file")
    end

    it_behaves_like "a successful search", "dir/", "dir/file"
  end

  context "with an absolute starting point" do
    subject(:result) { find(sandbox.join("dir").to_s) }

    before do
      sandbox.mkdir("dir")
      sandbox.touch("dir/file")
    end

    it "prints absolute names" do
      expect(result).to print_lines(sandbox.join("dir").to_s, sandbox.join("dir/file").to_s)
    end
  end

  context "with a name starting with a dash" do
    subject(:result) { find("-file") }

    before { sandbox.touch("-file") }

    it_behaves_like "a rejected command line", "find: unknown predicate `-file'"
  end

  context "with a name starting with a dash written with a leading ./" do
    subject(:result) { find("./-file") }

    before { sandbox.touch("-file") }

    it_behaves_like "a successful search", "./-file"
  end

  context "with a lone dash" do
    subject(:result) { find("-") }

    before { sandbox.touch("-") }

    it_behaves_like "a successful search", "-"
  end

  context "with --" do
    context "when followed by a starting point" do
      subject(:result) { find("--", "dir") }

      before { sandbox.mkdir("dir") }

      it_behaves_like "a successful search", "dir"
    end

    context "when followed by a name starting with a dash" do
      subject(:result) { find("--", "-file") }

      before { sandbox.touch("-file") }

      it_behaves_like "a rejected command line", "find: unknown predicate `-file'"
    end

    context "when preceded by -L" do
      subject(:result) { find("-L", "--", "top") }

      include_context "with links to a directory at and below the starting points"

      it_behaves_like "a successful search", "top", "top/link", "top/link/file"
    end

    context "when followed by -L" do
      subject(:result) { find("--", "-L", "top") }

      it_behaves_like "a rejected command line", "find: unknown predicate `-L'"
    end

    context "when placed after a starting point" do
      subject(:result) { find("dir", "--") }

      before { sandbox.mkdir("dir") }

      it_behaves_like "a rejected command line", "find: unknown predicate `--'"
    end
  end

  context "with a name made of an opening parenthesis and more", upstream: "find.posix/files-not-expressions1.exp" do
    subject(:result) { find("(1") }

    before { sandbox.touch("(1") }

    it_behaves_like "a successful search", "(1"
  end

  context "with a name made of an exclamation mark and more", upstream: "find.posix/files-not-expressions2.exp" do
    subject(:result) { find("!2") }

    before { sandbox.touch("!2") }

    it_behaves_like "a successful search", "!2"
  end

  context "with a lone closing parenthesis", upstream: "find.posix/files-not-expressions3.exp" do
    subject(:result) { find(")") }

    before { sandbox.touch(")") }

    it_behaves_like "a successful search", ")"
  end

  context "with directories named like operators", upstream: "find.posix/sv-bug-15235.exp" do
    subject(:result) { find("(1", "!2", ",", ")") }

    before do
      sandbox.mkdir("(1", "!2", ",", ")")
      sandbox.touch("(1/a", "!2/b", ",/c", ")/d")
    end

    it_behaves_like "a successful search", "(1", "(1/a", "!2", "!2/b", ",", ",/c", ")", ")/d"
  end

  context "with an entry named with two leading dots", upstream: "find.posix/dotdotfiles.exp" do
    subject(:result) { find("tmp") }

    before { sandbox.mkdir("tmp/..tmp") }

    it_behaves_like "a successful search", "tmp", "tmp/..tmp"
  end

  context "with a parent directory that cannot be read", :unprivileged, upstream: "find.posix/parent.exp" do
    subject(:result) { find("tmp/dir") }

    before do
      sandbox.mkdir("tmp/dir")
      sandbox.chmod(0o111, "tmp")
    end

    it_behaves_like "a successful search", "tmp/dir"
  end
end
