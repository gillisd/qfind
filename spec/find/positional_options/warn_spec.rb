RSpec.describe "find -warn and -nowarn" do
  let(:deprecation_warning) do
    "find: warning: the -d option is deprecated; please use -depth instead, because the latter is a " \
      "POSIX-compliant feature."
  end

  let(:placement_warning) do
    "find: warning: you have specified the global option -maxdepth after the argument -name, but global options " \
      "are not positional, i.e., -maxdepth affects tests specified before it as well as those specified after " \
      "it.  Please specify global options before other arguments."
  end

  before { sandbox.mkdir("dir") }

  context "with -warn before a deprecated option" do
    subject(:result) { find("dir", "-warn", "-d") }

    it { is_expected.to diagnose(deprecation_warning) }

    it "leaves the exit status unaffected" do
      expect(result).to be_successful
    end
  end

  context "with -warn after a deprecated option" do
    subject(:result) { find("dir", "-d", "-warn") }

    it { is_expected.to diagnose_nothing }
  end

  context "with -nowarn before a deprecated option on a terminal" do
    subject(:result) { find_on_terminal("dir", "-nowarn", "-d") }

    it { is_expected.to diagnose_nothing }
  end

  context "with -nowarn after a deprecated option on a terminal" do
    subject(:result) { find_on_terminal("dir", "-d", "-nowarn") }

    it { is_expected.to diagnose(deprecation_warning) }
  end

  context "with -warn followed by -nowarn" do
    subject(:result) { find_on_terminal("dir", "-warn", "-nowarn", "-d") }

    it { is_expected.to diagnose_nothing }
  end

  context "with -nowarn followed by -warn" do
    subject(:result) { find("dir", "-nowarn", "-warn", "-d") }

    it { is_expected.to diagnose(deprecation_warning) }
  end

  context "with -warn between a test and a global option" do
    subject(:result) { find("dir", "-name", "x", "-warn", "-maxdepth", "1") }

    it { is_expected.to diagnose(placement_warning) }
  end

  context "with -nowarn between a test and a global option on a terminal" do
    subject(:result) { find_on_terminal("dir", "-name", "x", "-nowarn", "-maxdepth", "1") }

    it { is_expected.to diagnose_nothing }
  end

  context "without -warn or -nowarn" do
    context "when standard input is a terminal" do
      subject(:result) { find_on_terminal("dir", "-d") }

      it { is_expected.to diagnose(deprecation_warning) }
    end

    context "when standard input is a pipe" do
      subject(:result) { find("dir", "-d") }

      it { is_expected.to diagnose_nothing }
    end

    context "when standard input is a regular file" do
      subject(:result) { find_with_stdin_from("input", "dir", "-d") }

      before { sandbox.touch("input") }

      it { is_expected.to diagnose_nothing }
    end
  end

  context "with POSIXLY_CORRECT set" do
    context "when standard input is a terminal" do
      subject(:result) { find_on_terminal("dir", "-d", env: { "POSIXLY_CORRECT" => "1" }) }

      it { is_expected.to diagnose_nothing }
    end

    context "with -warn" do
      subject(:result) { find("dir", "-warn", "-d", env: { "POSIXLY_CORRECT" => "1" }) }

      it { is_expected.to diagnose_nothing }
    end
  end
end
