RSpec.describe "find --version" do
  shared_examples "a version report" do
    it { is_expected.to be_successful }
    it { is_expected.to diagnose_nothing }

    it "starts with the program, package and version" do
      expect(result.lines.first).to eq("find (GNU findutils) 4.11.0")
    end

    it "states the copyright" do
      expect(result.lines).to include("Copyright (C) 2026 Free Software Foundation, Inc.")
    end

    it "states the licence" do
      expect(result.lines).to include("License GPLv3+: GNU GPL version 3 or later <https://gnu.org/licenses/gpl.html>.")
    end

    it "ends with the enabled features" do
      expect(result.lines.last).to start_with("Features enabled: ")
    end
  end

  context "with two leading dashes", upstream: "tests/misc/help-version.sh" do
    subject(:result) { find("--version") }

    it_behaves_like "a version report"
  end

  context "with two leading dashes and standard output on a full device", upstream: "tests/misc/help-version.sh" do
    subject(:result) { find_with_stdout_to("/dev/full", "--version") }

    it { is_expected.to have_exit_status(1) }
    it { is_expected.to diagnose("find: write error: No space left on device") }
  end

  context "with a single leading dash" do
    subject(:result) { find("-version") }

    it_behaves_like "a version report"
  end

  context "with a single leading dash and standard output on a full device" do
    subject(:result) { find_with_stdout_to("/dev/full", "-version") }

    it { is_expected.to have_exit_status(1) }
    it { is_expected.to diagnose("find: write error: No space left on device") }
  end

  context "with a starting point that does not exist" do
    subject(:result) { find("missing", "--version") }

    it_behaves_like "a version report"
  end

  context "when followed by --help" do
    subject(:result) { find("--version", "--help") }

    it_behaves_like "a version report"
  end

  context "when preceded by an unknown predicate" do
    subject(:result) { find("-foo", "--version") }

    it_behaves_like "a rejected command line", "find: unknown predicate `-foo'"
  end

  context "with an abbreviated name" do
    subject(:result) { find("--vers") }

    it_behaves_like "a rejected command line", "find: unknown predicate `--vers'"
  end
end
