RSpec.describe "find expr1 , expr2" do
  before { sandbox.touch("file") }

  context "with a false test before a test matching only the starting point", upstream: "find.gnu/comma.exp" do
    subject(:result) { find(".", "-maxdepth", "2", "-false", ",", "-name", ".") }

    before { sandbox.mkdir("sub/deeper/deepest") }

    it_behaves_like "a successful search", "."
  end

  {
    ["-true", "-true"] => ["file"],
    ["-true", "-false"] => [],
    ["-false", "-true"] => ["file"],
    ["-false", "-false"] => [],
  }.each do |(left, right), lines|
    context "with #{left} and #{right} as operands" do
      subject(:result) { find("file", left, ",", right) }

      it_behaves_like "a successful search", *lines
    end
  end

  ["true", "false"].each do |value|
    context "when the first expression is #{value}" do
      subject(:result) { find("file", "-#{value}", ",", "-printf", "evaluated\n") }

      at_every_optimisation_level do
        it "evaluates the second expression" do
          expect(result).to print_lines("evaluated")
        end
      end
    end
  end

  context "with an action on each side" do
    subject(:result) { find("file", "-printf", "first\n", ",", "-printf", "second\n") }

    at_every_optimisation_level do
      it "performs both from left to right" do
        expect(result).to print_lines("first", "second").in_order
      end
    end
  end

  context "with -fprintf to a different file on each side" do
    let(:search) do
      find("d",
           "(", "-type", "d", "-fprintf", "dirs", "%p\n", ")",
           ",",
           "(", "-type", "f", "-fprintf", "files", "%p\n", ")")
    end

    before do
      sandbox.mkdir("d/sub")
      sandbox.touch("d/f")
    end

    context "when reading the file written by the first expression" do
      subject(:written) { search.then { sandbox.read("dirs") } }

      it "lists the directories" do
        expect(written).to eq("d\nd/sub\n")
      end
    end

    context "when reading the file written by the second expression" do
      subject(:written) { search.then { sandbox.read("files") } }

      it "lists the regular files" do
        expect(written).to eq("d/f\n")
      end
    end
  end

  context "with a comma where a starting point is expected" do
    subject(:result) { find(",", "-print") }

    before { sandbox.touch(",") }

    it_behaves_like "a successful search", ","
  end

  context "with an argument merely beginning with a comma" do
    subject(:result) { find(".", "-true", ",x") }

    it_behaves_like "a rejected command line", "find: paths must precede expression: `,x'"
  end

  it_behaves_like "a binary operator requiring two operands", ","
end
