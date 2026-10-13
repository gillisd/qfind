RSpec.describe "find expr1 -o expr2" do
  shared_examples "a disjunction" do |operator, **metadata|
    context "with -print -false on the left and -print on the right", **metadata do
      subject(:result) { find("tmp", "-depth", "-print", "-false", operator, "-print") }

      before { sandbox.mkdir("tmp/fred/jim") }

      it_behaves_like "a successful search", "tmp/fred/jim", "tmp/fred/jim", "tmp/fred", "tmp/fred", "tmp", "tmp"
    end

    {
      ["-true", "-true"] => ["file"],
      ["-true", "-false"] => ["file"],
      ["-false", "-true"] => ["file"],
      ["-false", "-false"] => [],
    }.each do |(left, right), lines|
      context "with #{left} and #{right} as operands" do
        subject(:result) { find("file", left, operator, right) }

        it_behaves_like "a successful search", *lines
      end
    end

    context "when the first expression is true" do
      subject(:result) { find("file", "-true", operator, "-printf", "evaluated\n") }

      at_every_optimisation_level do
        it "does not evaluate the second expression" do
          expect(result).to print_nothing
        end
      end
    end

    context "when the first expression is false" do
      subject(:result) { find("file", "-false", operator, "-printf", "evaluated\n") }

      at_every_optimisation_level do
        it "evaluates the second expression" do
          expect(result).to print_lines("evaluated")
        end
      end
    end

    context "with an action on each side" do
      subject(:result) { find("file", "-printf", "first\n", operator, "-printf", "second\n") }

      at_every_optimisation_level do
        it "performs only the first, which is true" do
          expect(result).to print_lines("first")
        end
      end
    end
  end

  before { sandbox.touch("file") }

  context "with -o" do
    it_behaves_like "a disjunction", "-o"
    it_behaves_like "a binary operator recognised at the start of the expression", "-o"
  end

  context "with -or" do
    it_behaves_like "a disjunction", "-or", upstream: "find.gnu/gnu-or.exp"
    it_behaves_like "a binary operator recognised at the start of the expression", "-or"
  end
end
