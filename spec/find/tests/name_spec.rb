RSpec.describe "find -name" do
  let(:slash_warning) do
    "find: warning: '-name' matches against basenames only, but the given pattern contains a " \
      "directory separator ('/'), thus the expression will evaluate to false all the time.  " \
      "Did you mean '-wholename'?"
  end

  context "with a pattern naming one of two sibling directories", upstream: "find.posix/name.exp" do
    subject(:result) { find("tmp", "-name", "fred", "-print") }

    before { sandbox.mkdir("tmp/fred", "tmp/jim") }

    it_behaves_like "a successful search", "tmp/fred"
  end

  context "with a wildcard that could match a leading dot", upstream: "find.gnu/name-period.exp" do
    subject(:result) { find("tmp/top", "-name", "*ne", "-print") }

    before { sandbox.mkdir("tmp/top/.one", "tmp/top/bane") }

    it_behaves_like "a successful search", "tmp/top/bane", "tmp/top/.one"
  end

  context "with starting points carrying trailing slashes", upstream: "find.posix/nameslash.exp" do
    subject(:result) { find("tmp/foo///", "tmp/bar///", "-name", "foo", "-o", "-name", "bar?*") }

    before { sandbox.mkdir("tmp/foo", "tmp/bar") }

    it_behaves_like "a successful search", "tmp/foo///"
  end

  context "with -prune ahead of a pattern matching only below the starting point", upstream: "find.gnu/name-opt.exp" do
    subject(:result) { find("tmp/top", "-prune", "-name", "foo") }

    before do
      sandbox.mkdir("tmp/top/one")
      sandbox.touch("tmp/top/one/foo")
    end

    it_behaves_like "a successful search"
  end

  context "with the root directory as pattern", upstream: "find.gnu/name-slash.exp" do
    subject(:result) { find("/", "///", "-maxdepth", "0", "-name", "/") }

    it_behaves_like "a successful search", "/", "///"
  end

  context "with a lone opening bracket as pattern", upstream: "tests/find/name-lbracket-literal.sh" do
    subject(:result) { find("-name", "[", "-print") }

    before { sandbox.touch("[") }

    it_behaves_like "a successful search", "./["
  end

  context "with a bracket expression" do
    subject(:result) { find(".", "-name", "[ab]*") }

    before { sandbox.touch("apple", "banana", "cherry") }

    it_behaves_like "a successful search", "./apple", "./banana"
  end

  context "with a question mark wildcard" do
    subject(:result) { find(".", "-name", "?at") }

    before { sandbox.touch("bat", "cat", "chat") }

    it_behaves_like "a successful search", "./bat", "./cat"
  end

  context "with an escaped wildcard" do
    subject(:result) { find(".", "-name", "\\*") }

    before { sandbox.touch("*", "star") }

    it_behaves_like "a successful search", "./*"
  end

  context "with a pattern differing only in case" do
    subject(:result) { find(".", "-name", "FRED") }

    before { sandbox.touch("fred") }

    it_behaves_like "a successful search"
  end

  context "without a pattern", upstream: "find.posix/name-missing.exp" do
    subject(:result) { find("tmp", "-name") }

    before { sandbox.touch("tmp") }

    it_behaves_like "a rejected command line", "find: missing argument to `-name'"
  end

  context "with a slash in the pattern", upstream: "tests/find/name-slash.sh" do
    context "when standard input is a terminal" do
      subject(:result) { find_on_terminal("-name", "dir/file") }

      it { is_expected.to be_successful }
      it { is_expected.to print_nothing }
      it { is_expected.to diagnose(slash_warning) }
    end

    context "with -warn" do
      subject(:result) { find("-warn", "-name", "dir/file") }

      it { is_expected.to diagnose(slash_warning) }
    end

    context "with -nowarn" do
      subject(:result) { find_on_terminal("-nowarn", "-name", "dir/file") }

      it { is_expected.to diagnose_nothing }
    end

    context "with POSIXLY_CORRECT set" do
      subject(:result) { find_on_terminal("-name", "dir/file", env: { "POSIXLY_CORRECT" => "1" }) }

      it { is_expected.to diagnose_nothing }
    end

    context "when standard input is not a terminal" do
      subject(:result) { find("-name", "dir/file") }

      it_behaves_like "a successful search"
    end
  end

  context "with a lone slash as pattern", upstream: "tests/find/name-slash.sh" do
    context "with -warn" do
      subject(:result) { find("/", "-warn", "-maxdepth", "0", "-name", "/") }

      it_behaves_like "a successful search", "/"
    end

    context "with POSIXLY_CORRECT set" do
      subject(:result) { find("/", "-maxdepth", "0", "-name", "/", env: { "POSIXLY_CORRECT" => "1" }) }

      it_behaves_like "a successful search", "/"
    end
  end
end
