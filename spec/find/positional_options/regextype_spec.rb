RSpec.describe "find -regextype" do
  shared_examples "a regular expression syntax" do |type, matches|
    before do
      sandbox.touch("a", "b", "aa", "ab1", "abab", "w", "ww", "a+", "a{1", "a{2}", "(a)", "a|b", "]", "\\]")
    end

    matches.each do |expression, lines|
      context "with the expression #{expression}" do
        subject(:result) { find(".", "-regextype", type, "-regex", expression) }

        it_behaves_like "a successful search", *lines
      end
    end
  end

  shared_examples "a syntax whose dot matches a newline" do |type|
    context "with a dot and a name containing a newline" do
      subject(:result) { find(".", "-regextype", type, "-regex", "\\./a.b") }

      before { sandbox.touch("a\nb") }

      it { is_expected.to print_exactly("./a\nb\n") }
    end
  end

  shared_examples "an Emacs syntax" do |type|
    it_behaves_like "a regular expression syntax", type, {
      "\\./\\(a\\|b\\)" => ["./a", "./b"],
      "\\./(a)" => ["./(a)"],
      "\\./a+" => ["./a", "./aa"],
      "\\./a\\{2\\}" => ["./aa"],
      "\\./a{2}" => ["./a{2}"],
    }
  end

  shared_examples "a POSIX basic syntax" do |type|
    it_behaves_like "a regular expression syntax", type, {
      "\\./\\(a\\|b\\)" => ["./a", "./b"],
      "\\./a\\+" => ["./a", "./aa"],
      "\\./a+" => ["./a+"],
      "\\./a\\{2\\}" => ["./aa"],
    }
    it_behaves_like "a syntax whose dot matches a newline", type
  end

  shared_examples "a POSIX extended syntax" do |type|
    it_behaves_like "a regular expression syntax", type, {
      "\\./(a|b)" => ["./a", "./b"],
      "\\./a+" => ["./a", "./aa"],
      "\\./a{2}" => ["./aa"],
      "\\./a\\{2\\}" => ["./a{2}"],
    }
    it_behaves_like "a syntax whose dot matches a newline", type
  end

  context "with posix-extended and a grouping expression", upstream: "find.gnu/regex2.exp" do
    subject(:result) { find("tmp", "-regextype", "posix-extended", "-regex", "tmp(/d)*", "-print") }

    before { sandbox.mkdir("tmp/d/d/d/e") }

    it_behaves_like "a successful search", "tmp", "tmp/d", "tmp/d/d", "tmp/d/d/d"
  end

  context "when placed after the expression" do
    subject(:result) { find(".", "-regex", "\\./(a)", "-regextype", "posix-extended") }

    before { sandbox.touch("a", "(a)") }

    it_behaves_like "a successful search", "./(a)"
  end

  context "when placed between two expressions" do
    subject(:result) { find(".", "-regex", "\\./(a)", "-regextype", "posix-extended", "-o", "-regex", "\\./(a)") }

    before { sandbox.touch("a", "(a)") }

    it_behaves_like "a successful search", "./(a)", "./a"
  end

  context "when given twice" do
    subject(:result) { find(".", "-regextype", "posix-extended", "-regextype", "emacs", "-regex", "\\./(a)") }

    before { sandbox.touch("a", "(a)") }

    it_behaves_like "a successful search", "./(a)"
  end

  context "when followed by -iregex" do
    subject(:result) { find(".", "-regextype", "posix-extended", "-iregex", "\\./(A)") }

    before { sandbox.touch("a", "(a)") }

    it_behaves_like "a successful search", "./a"
  end

  context "without any test or action" do
    subject(:result) { find(".", "-regextype", "posix-extended") }

    before { sandbox.touch("a") }

    it_behaves_like "a successful search", ".", "./a"
  end

  context "with emacs" do
    it_behaves_like "an Emacs syntax", "emacs"

    context "with a dot and a name containing a newline" do
      subject(:result) { find(".", "-regextype", "emacs", "-regex", "\\./a.b") }

      before { sandbox.touch("a\nb") }

      it_behaves_like "a successful search"
    end
  end

  context "with findutils-default" do
    it_behaves_like "an Emacs syntax", "findutils-default"
    it_behaves_like "a syntax whose dot matches a newline", "findutils-default"
  end

  ["posix-basic", "ed", "sed"].each do |type|
    context "with #{type}" do
      it_behaves_like "a POSIX basic syntax", type

      context "with alternatives separated by a newline" do
        subject(:result) { find(".", "-regextype", type, "-regex", "\\./a\n\\./b") }

        before { sandbox.touch("a", "b") }

        it_behaves_like "a successful search"
      end

      context "with a doubled star" do
        subject(:result) { find(".", "-regextype", type, "-regex", "\\./a**") }

        it_behaves_like "a rejected command line",
                        "find: failed to compile regular expression '\\./a**': Invalid preceding regular expression"
      end
    end
  end

  context "with grep" do
    it_behaves_like "a POSIX basic syntax", "grep"

    context "with alternatives separated by a newline" do
      subject(:result) { find(".", "-regextype", "grep", "-regex", "\\./a\n\\./b") }

      before { sandbox.touch("a", "b", "c") }

      it_behaves_like "a successful search", "./a", "./b"
    end

    context "with a doubled star" do
      subject(:result) { find(".", "-regextype", "grep", "-regex", "\\./a**") }

      before { sandbox.touch("a", "aa", "b") }

      it_behaves_like "a successful search", "./a", "./aa"
    end
  end

  context "with posix-minimal-basic" do
    it_behaves_like "a regular expression syntax", "posix-minimal-basic", {
      "\\./\\(a\\)" => ["./a"],
      "\\./a\\|b" => ["./a|b"],
      "\\./a\\+" => ["./a+"],
      "\\./a\\{2\\}" => ["./aa"],
    }
    it_behaves_like "a syntax whose dot matches a newline", "posix-minimal-basic"
  end

  context "with posix-extended" do
    it_behaves_like "a POSIX extended syntax", "posix-extended"

    context "with alternatives separated by a newline" do
      subject(:result) { find(".", "-regextype", "posix-extended", "-regex", "\\./a\n\\./b") }

      before { sandbox.touch("a", "b") }

      it_behaves_like "a successful search"
    end

    context "with an unterminated interval" do
      subject(:result) { find(".", "-regextype", "posix-extended", "-regex", "\\./a{1") }

      it_behaves_like "a rejected command line", "find: failed to compile regular expression '\\./a{1': Unmatched \\{"
    end
  end

  ["egrep", "posix-egrep"].each do |type|
    context "with #{type}" do
      it_behaves_like "a POSIX extended syntax", type

      context "with alternatives separated by a newline" do
        subject(:result) { find(".", "-regextype", type, "-regex", "\\./a\n\\./b") }

        before { sandbox.touch("a", "b", "c") }

        it_behaves_like "a successful search", "./a", "./b"
      end

      context "with an unterminated interval" do
        subject(:result) { find(".", "-regextype", type, "-regex", "\\./a{1") }

        before { sandbox.touch("a", "a{1") }

        it_behaves_like "a successful search", "./a{1"
      end
    end
  end

  context "with awk" do
    it_behaves_like "a regular expression syntax", "awk", {
      "\\./(a|b)" => ["./a", "./b"],
      "\\./a+" => ["./a", "./aa"],
      "\\./a{2}" => ["./a{2}"],
      "\\./\\w\\w" => ["./ww"],
      "\\./(ab)\\1" => ["./ab1"],
      "\\./[\\]]" => ["./]"],
    }
    it_behaves_like "a syntax whose dot matches a newline", "awk"
  end

  context "with gnu-awk" do
    it_behaves_like "a POSIX extended syntax", "gnu-awk"
    it_behaves_like "a regular expression syntax", "gnu-awk", {
      "\\./\\w\\w" => ["./aa", "./ww"],
      "\\./(ab)\\1" => ["./abab"],
      "\\./[\\]]" => ["./]"],
    }
  end

  context "with posix-awk" do
    it_behaves_like "a POSIX extended syntax", "posix-awk"
    it_behaves_like "a regular expression syntax", "posix-awk", {
      "\\./\\w\\w" => ["./ww"],
      "\\./(ab)\\1" => ["./abab"],
      "\\./[\\]]" => ["./]"],
    }
  end

  {
    "help" => "with help as the type",
    "bogus" => "with an unknown type",
    "EMACS" => "with a known type in upper case",
    "" => "with an empty type",
  }.each do |type, description|
    context description do
      subject(:result) { find(".", "-regextype", type) }

      it_behaves_like "a rejected command line",
                      "find: Unknown regular expression type '#{type}'; valid types are 'findutils-default', " \
                      "'awk', 'ed', 'egrep', 'emacs', 'gnu-awk', 'grep', 'posix-awk', 'posix-basic', " \
                      "'posix-egrep', 'posix-extended', 'posix-minimal-basic', 'sed'."
    end
  end

  context "without a type" do
    subject(:result) { find(".", "-regextype") }

    it_behaves_like "a rejected command line", "find: missing argument to `-regextype'"
  end
end
