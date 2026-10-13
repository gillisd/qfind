RSpec.describe "find -printf escapes" do
  before { sandbox.touch("file") }

  {
    "character escapes" => {
      "\\a" => [7.chr, "an alarm bell"],
      "\\b" => [8.chr, "a backspace"],
      "\\f" => [12.chr, "a form feed"],
      "\\n" => ["\n", "a newline"],
      "\\r" => ["\r", "a carriage return"],
      "\\t" => ["\t", "a horizontal tab"],
      "\\v" => [11.chr, "a vertical tab"],
      "\\\\" => ["\\", "a single backslash"],
    },
    "octal escapes" => {
      "\\0" => [0.chr, "a null character"],
      "\\1" => [1.chr, "the character whose code is a single octal digit"],
      "\\02" => [2.chr, "the character whose code is two octal digits"],
      "\\003" => [3.chr, "the character whose code is three octal digits"],
      "\\101" => ["A", "the letter whose code is octal 101"],
      "\\0044" => ["#{4.chr}4", "the character for the first three digits followed by the fourth digit"],
      "\\0028" => ["#{2.chr}8", "the character for the octal digits followed by the digit 8"],
      "\\501" => ["A", "the character whose code is the octal value modulo 256"],
      "\\045p" => ["%p", "a percent sign that starts no directive"],
    },
  }.each do |kind, escapes|
    context "with #{kind}", upstream: "tests/find/printf_escapechars.sh" do
      escapes.each do |escape, (character, meaning)|
        context "with #{escape}" do
          subject(:result) { find("file", "-printf", "<#{escape}>") }

          it "prints #{meaning}" do
            expect(result).to print_exactly("<#{character}>")
          end

          it { is_expected.to diagnose_nothing }
        end
      end
    end
  end

  context "with escapes between directives" do
    subject(:result) { find("file", "-printf", "%p\\t%f\\n") }

    it_behaves_like "a successful search printing exactly", "file\tfile\n"
  end

  context "with unknown escapes", upstream: "tests/find/printf_escapechars.sh" do
    {
      "\\z" => ["\\z", "an unknown letter"],
      "\\8" => ["\\8", "a digit that is not octal"],
      "\\x41" => ["\\x", "a hexadecimal escape as in C"],
    }.each do |escape, (warned, description)|
      context "with #{escape}, #{description}" do
        subject(:result) { find("file", "-printf", "<#{escape}>\n") }

        it_behaves_like "a successful search that warns",
                        "find: warning: unrecognized escape `#{warned}'", "<#{escape}>"
      end
    end
  end

  context "with a backslash ahead of a directive" do
    subject(:result) { find("file", "-printf", "<\\%p>\n") }

    it_behaves_like "a successful search that warns", "find: warning: unrecognized escape `\\%'", "<\\%p>"
  end

  context "with a backslash at the end of the format" do
    subject(:result) { find("file", "-printf", "end\\") }

    it_behaves_like "a successful search that warns",
                    "find: warning: escape `\\' followed by nothing at all", "end\\"
  end

  context "with \\c ahead of a command", upstream: "tests/find/printf_escape_c.sh" do
    subject(:result) do
      find(".", "-maxdepth", "0", "-printf", "hello^\\cthere",
           "-exec", "printf", "%s", "{}", ";", "-printf", "^world\\n")
    end

    it { is_expected.to be_successful }

    it "prints the format up to \\c only, ahead of the output of the command" do
      expect(result).to print_exactly("hello^.^world\n")
    end
  end

  context "with \\c and several files" do
    subject(:result) { find("dir", "-printf", "%p\\c and more\n") }

    before do
      sandbox.mkdir("dir")
      sandbox.touch("dir/entry")
    end

    it "stops the format at \\c for every file" do
      expect(result).to print_exactly("dirdir/entry")
    end
  end

  context "with \\c at the start of the format" do
    subject(:result) { find("file", "-printf", "\\c%p\n") }

    it { is_expected.to print_nothing }
  end

  context "with \\c ahead of invalid directives" do
    subject(:result) { find("file", "-printf", "%p\\c%(%") }

    it_behaves_like "a successful search printing exactly", "file"
  end

  context "with \\c ahead of an unknown escape" do
    subject(:result) { find("file", "-printf", "%p\\c\\z") }

    it "ignores the rest of the format" do
      expect(result).to diagnose_nothing
    end
  end
end
