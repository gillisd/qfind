RSpec.describe "find --help" do
  shared_examples "a usage summary" do
    it { is_expected.to be_successful }
    it { is_expected.to diagnose_nothing }

    it "starts with the synopsis" do
      expect(result.lines.first).to eq("Usage: find [-H] [-L] [-P] [-Olevel] [-D debugopts] [path...] [expression]")
    end

    {
      "names the default starting point and expression" =>
        "Default path is the current directory; default expression is -print.",
      "lists the operators" => "      ( EXPR )   ! EXPR   -not EXPR   EXPR1 -a EXPR2   EXPR1 -and EXPR2",
      "lists the positional options" => "      -daystart -follow -nowarn -regextype -warn",
      "lists the debug options" => "      exec, opt, rates, search, stat, time, tree, all, help",
      "describes --help" => "      --help                   display this help and exit",
      "describes --version" => "      --version                output version information and exit",
      "points to the documentation" => "Please see also the documentation at https://www.gnu.org/software/findutils/.",
    }.each do |behaviour, line|
      it behaviour do
        expect(result.lines).to include(line)
      end
    end
  end

  context "with two leading dashes", upstream: "tests/misc/help-version.sh" do
    subject(:result) { find("--help") }

    it_behaves_like "a usage summary"
  end

  context "with two leading dashes and standard output on a full device", upstream: "tests/misc/help-version.sh" do
    subject(:result) { find_with_stdout_to("/dev/full", "--help") }

    it { is_expected.to have_exit_status(1) }
    it { is_expected.to diagnose("find: write error: No space left on device") }
  end

  context "with a single leading dash" do
    subject(:result) { find("-help") }

    it_behaves_like "a usage summary"
  end

  context "with a single leading dash and standard output on a full device" do
    subject(:result) { find_with_stdout_to("/dev/full", "-help") }

    it { is_expected.to have_exit_status(1) }
    it { is_expected.to diagnose("find: write error: No space left on device") }
  end

  context "with a starting point that does not exist" do
    subject(:result) { find("missing", "--help") }

    it_behaves_like "a usage summary"
  end

  context "when followed by an unknown predicate" do
    subject(:result) { find("--help", "-foo") }

    it_behaves_like "a usage summary"
  end

  context "when followed by --version" do
    subject(:result) { find("--help", "--version") }

    it_behaves_like "a usage summary"
  end

  context "when preceded by an unknown predicate" do
    subject(:result) { find("-foo", "--help") }

    it_behaves_like "a rejected command line", "find: unknown predicate `-foo'"
  end

  context "with an abbreviated name" do
    subject(:result) { find("--he") }

    it_behaves_like "a rejected command line", "find: unknown predicate `--he'"
  end

  context "when placed after a test on a terminal" do
    subject(:result) { find_on_terminal(".", "-name", "x", "--help") }

    it { is_expected.to be_successful }

    it "warns that global options are not positional" do
      expect(result).to diagnose(
        "find: warning: you have specified the global option --help after the argument -name, but global options " \
        "are not positional, i.e., --help affects tests specified before it as well as those specified after it.  " \
        "Please specify global options before other arguments.",
      )
    end
  end
end
