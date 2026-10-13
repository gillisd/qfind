RSpec.describe "find -printf" do
  before { sandbox.touch("file") }

  context "without a format" do
    subject(:result) { find("file", "-printf") }

    it_behaves_like "a rejected command line", "find: missing argument to `-printf'"
  end

  context "with a format without a newline" do
    subject(:result) { find("dir", "-printf", "%p") }

    before do
      sandbox.mkdir("dir")
      sandbox.touch("dir/entry")
    end

    it "runs the output for successive files together" do
      expect(result).to print_exactly("dirdir/entry")
    end
  end

  context "with an empty format" do
    subject(:result) { find("file", "-printf", "") }

    it "suppresses the default -print" do
      expect(result).to print_nothing
    end
  end

  context "when followed by another action" do
    subject(:result) { find("file", "-printf", "printed ", "-printf", "%p\n") }

    it "evaluates to true" do
      expect(result).to print_exactly("printed file\n")
    end
  end

  context "when combined with -print" do
    subject(:result) { find("file", "-printf", "<%p>\n", "-print", "-printf", "</%p>\n") }

    it "shares standard output in evaluation order" do
      expect(result).to print_exactly("<file>\nfile\n</file>\n")
    end
  end

  context "with a format that looks like an action" do
    subject(:result) { find("file", "-printf", "-print") }

    it "prints the format as it is" do
      expect(result).to print_exactly("-print")
    end
  end

  context "with %%" do
    subject(:result) { find("file", "-printf", "100%%\n") }

    it_behaves_like "a successful search printing exactly", "100%\n"
  end

  context "with %% ahead of a directive letter" do
    subject(:result) { find("file", "-printf", "%%p\n") }

    it_behaves_like "a successful search printing exactly", "%p\n"
  end

  context "with a field width on %%" do
    subject(:result) { find("file", "-printf", "[%5%]") }

    it "prints the percent sign and the width instead of padding" do
      expect(result).to print_exactly("[%5]")
    end
  end

  {
    "z" => ["z", "an unknown letter"],
    "-5z" => ["z", "flags and a width ahead of an unknown letter"],
    "*d" => ["*", "a width taken from an argument as in printf(3)"],
    "1$p" => ["$", "a positional argument as in printf(3)"],
    "5-p" => ["-", "a flag after the width"],
  }.each do |directive, (character, description)|
    context "with % followed by #{description}" do
      subject(:result) { find("file", "-printf", "[%#{directive}]\n") }

      it_behaves_like "a successful search that warns",
                      "find: warning: unrecognized format directive `%#{character}'", "[%#{directive}]"
    end
  end

  context "with an unknown directive and several files" do
    subject(:result) { find("dir", "-printf", "%z\n") }

    before do
      sandbox.mkdir("dir")
      sandbox.touch("dir/a", "dir/b")
    end

    it "warns once, while parsing the format" do
      expect(result).to diagnose_exactly("find: warning: unrecognized format directive `%z'")
    end
  end

  context "with an unknown directive in an action no file reaches" do
    subject(:result) { find("file", "-false", "-printf", "%z") }

    it_behaves_like "a successful search that warns", "find: warning: unrecognized format directive `%z'"
  end

  context "with an unknown directive and -nowarn" do
    subject(:result) { find("file", "-nowarn", "-printf", "%z\n") }

    it_behaves_like "a successful search that warns", "find: warning: unrecognized format directive `%z'", "%z"
  end

  context "with a length modifier as in printf(3)" do
    subject(:result) { find("file", "-printf", "[%ld]") }

    it "reads the modifier as a directive of its own" do
      expect(result).to print_exactly("[d]")
    end
  end

  ["{", "[", "("].each do |character|
    context "with the reserved directive %#{character}", upstream: "find.gnu/printf-reserved.exp" do
      subject(:result) { find(".", "-maxdepth", "0", "-printf", "%#{character}") }

      it_behaves_like "a rejected command line",
                      "find: error: the format directive `%#{character}' is reserved for future use"
    end
  end

  context "with flags and a width ahead of a reserved directive" do
    subject(:result) { find("file", "-printf", "%-5(") }

    it_behaves_like "a rejected command line", "find: error: the format directive `%(' is reserved for future use"
  end

  context "with a reserved directive after other directives" do
    subject(:result) { find("file", "-printf", "%p %{") }

    it_behaves_like "a rejected command line", "find: error: the format directive `%{' is reserved for future use"
  end

  context "with a % at the end of the format", upstream: "find.gnu/sv-bug-17477.exp" do
    subject(:result) { find("-maxdepth", "0", "-printf", "test: %") }

    it_behaves_like "a rejected command line", "find: error: % at end of format string"
  end

  context "with flags and a width but no directive at the end of the format" do
    subject(:result) { find("file", "-printf", "test: %-5") }

    at_every_optimisation_level do
      it { is_expected.to have_exit_status(1) }
      it { is_expected.to print_nothing }
    end

    it "reports an error in the format" do
      expect(result).to diagnose(a_string_starting_with("find: error: "))
    end
  end

  context "when standard output is full" do
    subject(:result) { find_with_stdout_to("/dev/full", "file", "-printf", "%p\n") }

    it { is_expected.to have_exit_status(1) }
    it { is_expected.to diagnose("find: 'standard output': No space left on device") }
  end
end
