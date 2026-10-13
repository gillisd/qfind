RSpec.describe "find -iname" do
  let(:slash_warning) do
    "find: warning: '-iname' matches against basenames only, but the given pattern contains a " \
      "directory separator ('/'), thus the expression will evaluate to false all the time.  " \
      "Did you mean '-iwholename'?"
  end

  context "with a pattern differing in case from a directory name", upstream: "find.gnu/iname.exp" do
    subject(:result) { find("tmp", "-iname", "frED", "-print") }

    before { sandbox.mkdir("tmp/fred", "tmp/jim") }

    it_behaves_like "a successful search", "tmp/fred"
  end

  ["fo*", "F??"].each do |pattern|
    context "with the pattern #{pattern}" do
      subject(:result) { find(".", "-iname", pattern) }

      before { sandbox.touch("Foo", "FOO", "foo", "fOo", "bar") }

      it_behaves_like "a successful search", "./Foo", "./FOO", "./foo", "./fOo"
    end
  end

  context "with a wildcard that could match a leading dot" do
    subject(:result) { find(".", "-iname", "*foo*") }

    before { sandbox.touch(".foobar", "bar") }

    it_behaves_like "a successful search", "./.foobar"
  end

  context "with a bracket expression" do
    subject(:result) { find(".", "-iname", "[A-C]*") }

    before { sandbox.touch("apple", "Banana", "cherry", "Date") }

    it_behaves_like "a successful search", "./apple", "./Banana", "./cherry"
  end

  context "with braces in the pattern" do
    subject(:result) { find(".", "-iname", "FOO{1,2}") }

    before { sandbox.touch("foo{1,2}", "foo1", "foo2") }

    it_behaves_like "a successful search", "./foo{1,2}"
  end

  context "with a pattern matching a directory that has entries below it" do
    subject(:result) { find("tmp", "-iname", "TOP") }

    before { sandbox.mkdir("tmp/top/one") }

    it_behaves_like "a successful search", "tmp/top"
  end

  context "with starting points carrying trailing slashes" do
    subject(:result) { find("tmp/FOO///", "tmp/bar///", "-iname", "foo") }

    before { sandbox.mkdir("tmp/FOO", "tmp/bar") }

    it_behaves_like "a successful search", "tmp/FOO///"
  end

  context "with a lone slash as pattern" do
    subject(:result) { find("/", "-maxdepth", "0", "-iname", "/") }

    it_behaves_like "a successful search", "/"

    context "with -warn" do
      subject(:result) { find("/", "-warn", "-maxdepth", "0", "-iname", "/") }

      it { is_expected.to diagnose_nothing }
    end
  end

  context "with a slash in the pattern" do
    context "when standard input is a terminal" do
      subject(:result) { find_on_terminal("-iname", "dir/file") }

      it { is_expected.to be_successful }
      it { is_expected.to print_nothing }
      it { is_expected.to diagnose(slash_warning) }
    end

    context "with -warn" do
      subject(:result) { find("-warn", "-iname", "dir/file") }

      it { is_expected.to diagnose(slash_warning) }
    end

    context "with -nowarn" do
      subject(:result) { find_on_terminal("-nowarn", "-iname", "dir/file") }

      it { is_expected.to diagnose_nothing }
    end

    context "with POSIXLY_CORRECT set" do
      subject(:result) { find_on_terminal("-iname", "dir/file", env: { "POSIXLY_CORRECT" => "1" }) }

      it { is_expected.to diagnose_nothing }
    end

    context "when standard input is not a terminal" do
      subject(:result) { find("-iname", "dir/file") }

      before do
        sandbox.mkdir("dir")
        sandbox.touch("dir/file")
      end

      it_behaves_like "a successful search"
    end
  end

  context "without a pattern" do
    subject(:result) { find("tmp", "-iname") }

    before { sandbox.touch("tmp") }

    it_behaves_like "a rejected command line", "find: missing argument to `-iname'"
  end
end
