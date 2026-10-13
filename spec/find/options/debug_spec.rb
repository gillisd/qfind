RSpec.describe "find -D" do
  subject(:result) { find(*debug_arguments, "dir", "-name", "file", "-empty", "-exec", "echo", "{}", ";") }

  before do
    sandbox.mkdir("dir")
    sandbox.touch("dir/file")
  end

  shared_examples "a search unaffected by debugging" do
    it { is_expected.to be_successful }
    it { is_expected.to print_lines("dir/file") }
  end

  shared_examples "a trace of launched commands" do
    it { is_expected.to diagnose("DebugExec: launching process (argc=2): 'echo' 'dir/file'") }
    it { is_expected.to diagnose(a_string_ending_with("terminated with exit status: 0")) }
  end

  shared_examples "an explanation of the optimisations" do
    it { is_expected.to diagnose("-O1: promoting cheap predicate -name file into name_list") }
  end

  shared_examples "a summary of predicate success rates" do
    it { is_expected.to diagnose("Predicate success rates after completion:") }
  end

  shared_examples "a narration of the directory traversal" do
    it { is_expected.to diagnose(a_string_starting_with("consider_visiting (early): 'dir/file'")) }
  end

  shared_examples "a trace of stat calls" do
    it { is_expected.to diagnose("debug_stat (file)") }
  end

  shared_examples "a report of the start of the day" do
    it { is_expected.to diagnose(a_string_starting_with("cur_day_start = ")) }
  end

  shared_examples "a display of the expression tree" do
    it { is_expected.to diagnose("Predicate List:") }
    it { is_expected.to diagnose("Optimized Eval Tree:") }
  end

  {
    "exec" => ["a trace of launched commands"],
    "opt" => ["an explanation of the optimisations", "a display of the expression tree"],
    "rates" => ["a summary of predicate success rates"],
    "search" => ["a narration of the directory traversal"],
    "stat" => ["a trace of stat calls"],
    "time" => ["a report of the start of the day"],
    "tree" => ["a display of the expression tree"],
  }.each do |option, behaviours|
    context "with #{option}" do
      let(:debug_arguments) { ["-D", option] }

      it_behaves_like "a search unaffected by debugging"

      behaviours.each { |behaviour| it_behaves_like behaviour }
    end
  end

  context "with all" do
    let(:debug_arguments) { ["-D", "all"] }

    it_behaves_like "a search unaffected by debugging"
    it_behaves_like "a trace of launched commands"
    it_behaves_like "an explanation of the optimisations"
    it_behaves_like "a summary of predicate success rates"
    it_behaves_like "a narration of the directory traversal"
    it_behaves_like "a trace of stat calls"
    it_behaves_like "a report of the start of the day"
    it_behaves_like "a display of the expression tree"
  end

  context "with a comma-separated list" do
    let(:debug_arguments) { ["-D", "tree,search"] }

    it_behaves_like "a search unaffected by debugging"
    it_behaves_like "a display of the expression tree"
    it_behaves_like "a narration of the directory traversal"
  end

  context "when given twice" do
    let(:debug_arguments) { ["-D", "tree", "-D", "search"] }

    it_behaves_like "a display of the expression tree"
    it_behaves_like "a narration of the directory traversal"
  end

  context "with an unknown debug option" do
    let(:debug_arguments) { ["-D", "foo"] }

    it_behaves_like "a search unaffected by debugging"

    it { is_expected.to diagnose("find: Ignoring unrecognised debug flag 'foo'") }
  end

  context "with an unknown debug option after a known one" do
    let(:debug_arguments) { ["-D", "tree,foo"] }

    it_behaves_like "a display of the expression tree"

    it { is_expected.to diagnose(a_string_starting_with("find: Ignoring unrecognised debug flag ")) }
  end

  context "with help" do
    subject(:result) { find("-D", "help", "dir") }

    it { is_expected.to be_successful }

    it "starts with a heading" do
      expect(result.lines.first).to eq("Valid arguments for -D:")
    end

    {
      "exec" => "Show diagnostic information relating to -exec, -execdir, -ok and -okdir",
      "opt" => "Show diagnostic information relating to optimisation",
      "rates" => "Indicate how often each predicate succeeded",
      "search" => "Navigate the directory tree verbosely",
      "stat" => "Trace calls to stat(2) and lstat(2)",
      "time" => "Show diagnostic information relating to time-of-day and timestamp comparisons",
      "tree" => "Display the expression tree",
      "all" => "Set all of the debug flags (but help)",
      "help" => "Explain the various -D options",
    }.each do |option, explanation|
      it "explains #{option}" do
        expect(result.lines).to include("#{option.ljust(10)} #{explanation}")
      end
    end

    it "does not search" do
      expect(result.lines).not_to include("dir")
    end
  end

  context "with help among other debug options" do
    subject(:result) { find("-D", "tree,help", "dir") }

    it { is_expected.to be_successful }
    it { is_expected.to diagnose_nothing }

    it "explains the debug options instead of searching" do
      expect(result.lines.first).to eq("Valid arguments for -D:")
    end
  end

  context "without a list", upstream: "tests/find/debug-missing-arg.sh" do
    subject(:result) { find("-D") }

    it_behaves_like "a rejected command line", "find: Missing argument after the -D option."

    it { is_expected.to diagnose("Try 'find --help' for more information.") }
  end

  ["", ","].each do |list|
    context "with the list #{list.inspect}" do
      subject(:result) { find("-D", list) }

      it_behaves_like "a rejected command line", "find: Empty argument to the -D option."

      it { is_expected.to diagnose("Try 'find --help' for more information.") }
    end
  end

  context "with the list attached to the option" do
    subject(:result) { find("-Dtree") }

    it_behaves_like "a rejected command line", "find: unknown predicate `-Dtree'"
  end

  context "with the option after a starting point" do
    subject(:result) { find(".", "-D", "tree") }

    it_behaves_like "a rejected command line", "find: unknown predicate `-D'"
  end
end
