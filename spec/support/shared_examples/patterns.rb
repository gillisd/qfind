RSpec.shared_examples "a whole-path pattern test" do |primary|
  context "with a wildcard spanning directory separators" do
    subject(:result) { find(".", primary, "./sr*sc") }

    before { sandbox.mkdir("src/misc", "misc") }

    it_behaves_like "a successful search", "./src/misc"
  end

  context "with a wildcard matching a leading dot" do
    subject(:result) { find(".", primary, "*/*config") }

    before do
      sandbox.mkdir("quux/bar/baz")
      sandbox.touch("quux/bar/baz/.config", "quux/bar/baz/f")
    end

    it_behaves_like "a successful search", "./quux/bar/baz/.config"
  end

  context "with a question mark matching a directory separator" do
    subject(:result) { find(".", primary, "./a?b") }

    before do
      sandbox.mkdir("a")
      sandbox.touch("a/b", "axb", "ab")
    end

    it_behaves_like "a successful search", "./a/b", "./axb"
  end

  context "with a bracket expression matching a directory separator" do
    subject(:result) { find(".", primary, "./a[/]b") }

    before do
      sandbox.mkdir("a")
      sandbox.touch("a/b", "axb")
    end

    it_behaves_like "a successful search", "./a/b"
  end

  context "with an escaped wildcard" do
    subject(:result) { find(".", primary, "./a/\\*") }

    before do
      sandbox.mkdir("a")
      sandbox.touch("a/*", "a/b")
    end

    it_behaves_like "a successful search", "./a/*"
  end

  context "with a pattern naming only the base name" do
    subject(:result) { find("tmp", primary, "one") }

    before { sandbox.mkdir("tmp/one") }

    it_behaves_like "a successful search"
  end

  context "with a pattern omitting the starting point" do
    subject(:result) { find(".", primary, "a/b") }

    before do
      sandbox.mkdir("a")
      sandbox.touch("a/b")
    end

    it_behaves_like "a successful search"
  end

  context "with a pattern naming the starting point" do
    subject(:result) { find("tmp", primary, "tmp") }

    before { sandbox.mkdir("tmp/one") }

    it_behaves_like "a successful search", "tmp"
  end

  context "with an absolute pattern and a relative starting point" do
    subject(:result) { find("bar", primary, sandbox.join("bar/myfile").to_s) }

    before do
      sandbox.mkdir("bar")
      sandbox.touch("bar/myfile")
    end

    it_behaves_like "a successful search"
  end

  context "with an absolute pattern and an absolute starting point" do
    subject(:result) { find(starting_point, primary, "#{starting_point}/myfile") }

    let(:starting_point) { sandbox.join("bar").to_s }

    before do
      sandbox.mkdir("bar")
      sandbox.touch("bar/myfile")
    end

    it "matches the absolute name" do
      expect(result).to print_lines("#{starting_point}/myfile")
    end
  end

  context "with -prune to skip a directory tree" do
    subject(:result) { find(".", primary, "./src/emacs", "-prune", "-o", "-print") }

    before do
      sandbox.mkdir("src/emacs/lisp", "src/misc")
      sandbox.touch("src/emacs/lisp/site.el")
    end

    it_behaves_like "a successful search", ".", "./src", "./src/misc"
  end

  context "with a pattern ending in a slash" do
    subject(:result) { find(".", primary, "./foo/") }

    let(:warning) { "find: warning: #{primary} ./foo/ will not match anything because it ends with /." }

    before { sandbox.mkdir("foo") }

    it { is_expected.to be_successful }
    it { is_expected.to print_nothing }
    it { is_expected.to diagnose(warning) }

    context "with -nowarn" do
      subject(:result) { find(".", "-nowarn", primary, "./foo/") }

      it { is_expected.to diagnose(warning) }
    end

    context "with POSIXLY_CORRECT set" do
      subject(:result) { find(".", primary, "./foo/", env: { "POSIXLY_CORRECT" => "1" }) }

      it { is_expected.to diagnose_nothing }
    end

    context "when a starting point carries the same trailing slash" do
      subject(:result) { find("foo/", primary, "foo/") }

      it_behaves_like "a successful search", "foo/"
    end

    context "when it matches a starting point carrying a trailing slash" do
      subject(:result) { find("foo/", primary, "f*/") }

      it_behaves_like "a successful search", "foo/"
    end
  end

  context "without a pattern" do
    subject(:result) { find(".", primary) }

    it_behaves_like "a rejected command line", "find: missing argument to `#{primary}'"
  end
end

RSpec.shared_examples "a case-sensitive whole-path pattern test" do |primary|
  it_behaves_like "a whole-path pattern test", primary

  context "with a pattern differing only in case" do
    subject(:result) { find(".", primary, "./A/B") }

    before do
      sandbox.mkdir("a")
      sandbox.touch("a/b")
    end

    it_behaves_like "a successful search"
  end

  context "with a pattern ending in a slash differing in case from a starting point" do
    subject(:result) { find("FOO/", primary, "foo/") }

    before { sandbox.mkdir("FOO") }

    it { is_expected.to diagnose("find: warning: #{primary} foo/ will not match anything because it ends with /.") }
  end
end

RSpec.shared_examples "a case-insensitive whole-path pattern test" do |primary|
  it_behaves_like "a whole-path pattern test", primary

  context "with a wildcard pattern in a different case" do
    subject(:result) { find(".", primary, "./SR*SC") }

    before { sandbox.mkdir("src/misc", "SRC/MISC") }

    it_behaves_like "a successful search", "./src/misc", "./SRC/MISC"
  end

  context "with a starting point in a different case" do
    subject(:result) { find("Foo", primary, "foo/bar") }

    before { sandbox.mkdir("Foo/Bar") }

    it_behaves_like "a successful search", "Foo/Bar"
  end

  context "with a pattern ending in a slash differing in case from a starting point" do
    subject(:result) { find("FOO/", primary, "foo/") }

    before { sandbox.mkdir("FOO") }

    it_behaves_like "a successful search", "FOO/"
  end
end

RSpec.shared_examples "a link-target pattern test" do |primary|
  context "with a wildcard spanning directory separators" do
    subject(:result) { find("tmp", primary, "a*c") }

    before do
      sandbox.mkdir("tmp")
      sandbox.symlink("a/b/c", "tmp/deep")
    end

    it_behaves_like "a successful search", "tmp/deep"
  end

  context "with a wildcard matching a leading dot" do
    subject(:result) { find("tmp", primary, "*hidden") }

    before do
      sandbox.mkdir("tmp")
      sandbox.symlink(".hidden", "tmp/dot")
    end

    it_behaves_like "a successful search", "tmp/dot"
  end

  context "with a question mark matching a directory separator" do
    subject(:result) { find("tmp", primary, "a?b") }

    before do
      sandbox.mkdir("tmp")
      sandbox.symlink("a/b", "tmp/slash")
      sandbox.symlink("ab", "tmp/short")
    end

    it_behaves_like "a successful search", "tmp/slash"
  end

  context "with a pattern matching only the end of the target" do
    subject(:result) { find("tmp", primary, "one") }

    before do
      sandbox.mkdir("tmp", "dir")
      sandbox.touch("dir/one")
      sandbox.symlink("../dir/one", "tmp/up")
    end

    it_behaves_like "a successful search"
  end

  context "with a pattern matching any target" do
    subject(:result) { find("tmp", primary, "*") }

    before do
      sandbox.mkdir("tmp/dir")
      sandbox.touch("tmp/file")
      sandbox.symlink("file", "tmp/link")
    end

    it_behaves_like "a successful search", "tmp/link"
  end

  context "with a link to a file and a broken link" do
    include_context "with a link to a file and a broken link"

    context "without options" do
      subject(:result) { find("tmp", primary, "*") }

      it_behaves_like "a successful search", "tmp/BROKEN", "tmp/LINK"
    end

    context "with -L" do
      subject(:result) { find("-L", "tmp", primary, "*") }

      it_behaves_like "a successful search", "tmp/BROKEN"
    end

    context "with -follow" do
      subject(:result) { find("tmp", "-follow", primary, "*") }

      it_behaves_like "a successful search", "tmp/BROKEN"
    end

    context "with -follow after the test" do
      subject(:result) { find("tmp", primary, "*", "-follow") }

      it_behaves_like "a successful search", "tmp/BROKEN"
    end

    context "with -H" do
      subject(:result) { find("-H", "tmp", primary, "*") }

      it_behaves_like "a successful search", "tmp/BROKEN", "tmp/LINK"
    end

    context "with the links as starting points" do
      subject(:result) { find("tmp/LINK", "tmp/BROKEN", primary, "*") }

      it_behaves_like "a successful search", "tmp/BROKEN", "tmp/LINK"
    end

    context "with -H and the links as starting points" do
      subject(:result) { find("-H", "tmp/LINK", "tmp/BROKEN", primary, "*") }

      it_behaves_like "a successful search", "tmp/BROKEN"
    end
  end

  context "without a pattern" do
    subject(:result) { find(".", primary) }

    it_behaves_like "a rejected command line", "find: missing argument to `#{primary}'"
  end
end

RSpec.shared_examples "a whole-path regular expression test" do |primary|
  context "with a file named fubar3" do
    before { sandbox.touch("fubar3") }

    {
      ".*bar." => ["./fubar3"],
      ".*b.*3" => ["./fubar3"],
      "f.*r3" => [],
      ".*fubar" => [],
    }.each do |expression, lines|
      context "with the expression #{expression}" do
        subject(:result) { find(".", primary, expression) }

        it_behaves_like "a successful search", *lines
      end
    end
  end

  context "with the default syntax" do
    before { sandbox.touch("a", "b", "aa", "(a)", "a+", "a{2}") }

    {
      "\\./\\(a\\|b\\)" => ["./a", "./b"],
      "\\./(a)" => ["./(a)"],
      "\\./a+" => ["./a", "./aa"],
      "\\./a\\+" => ["./a+"],
      "\\./a\\{2\\}" => ["./aa"],
      "\\./a{2}" => ["./a{2}"],
    }.each do |expression, lines|
      context "with the expression #{expression}" do
        subject(:result) { find(".", primary, expression) }

        it_behaves_like "a successful search", *lines
      end
    end
  end

  context "with a dot and a name containing a newline" do
    subject(:result) { find(".", primary, "\\./a.b") }

    before { sandbox.touch("a\nb") }

    it_behaves_like "a successful search"
  end

  context "with POSIXLY_CORRECT set" do
    subject(:result) { find(".", primary, "\\./\\(a\\|b\\)", env: { "POSIXLY_CORRECT" => "1" }) }

    before { sandbox.touch("a", "b", "(a|b)") }

    it_behaves_like "a successful search", "./a", "./b"
  end

  context "with an expression ending in a slash" do
    subject(:result) { find(".", primary, ".*/") }

    before { sandbox.mkdir("foo") }

    it_behaves_like "a successful search"

    context "when the starting point carries a trailing slash" do
      subject(:result) { find("foo/", primary, "foo/") }

      it_behaves_like "a successful search", "foo/"
    end
  end

  {
    "[a" => "Unmatched [, [^, [:, [., or [=",
    "\\(a" => "Unmatched ( or \\(",
    "a\\)" => "Unmatched ) or \\)",
    "a\\{1" => "Unmatched \\{",
    "a\\{2,1\\}" => "Invalid content of \\{\\}",
    "[[:nope:]]" => "Invalid character class name",
    "\\1" => "Invalid back reference",
    "a\\" => "Trailing backslash",
  }.each do |expression, reason|
    context "with the invalid expression #{expression}" do
      subject(:result) { find(".", primary, expression) }

      it_behaves_like "a rejected command line", "find: failed to compile regular expression '#{expression}': #{reason}"
    end
  end

  context "without an expression" do
    subject(:result) { find(".", primary) }

    it_behaves_like "a rejected command line", "find: missing argument to `#{primary}'"
  end
end
