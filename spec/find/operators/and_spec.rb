RSpec.describe "find expr1 -a expr2" do
  shared_examples "a conjunction" do |*operator, **metadata|
    context "with a test naming one of two sibling directories before -print", **metadata do
      subject(:result) { find("tmp", "-name", "fred", *operator, "-print") }

      before { sandbox.mkdir("tmp/fred", "tmp/jim") }

      it_behaves_like "a successful search", "tmp/fred"
    end

    {
      ["-true", "-true"] => ["file"],
      ["-true", "-false"] => [],
      ["-false", "-true"] => [],
      ["-false", "-false"] => [],
    }.each do |(left, right), lines|
      context "with #{left} and #{right} as operands" do
        subject(:result) { find("file", left, *operator, right) }

        it_behaves_like "a successful search", *lines
      end
    end

    context "when the first expression is false" do
      subject(:result) { find("file", "-false", *operator, "-printf", "evaluated\n") }

      at_every_optimisation_level do
        it "does not evaluate the second expression" do
          expect(result).to print_nothing
        end
      end
    end

    context "when the first expression is true" do
      subject(:result) { find("file", "-true", *operator, "-printf", "evaluated\n") }

      at_every_optimisation_level do
        it "evaluates the second expression" do
          expect(result).to print_lines("evaluated")
        end
      end
    end

    context "with an action on each side" do
      subject(:result) { find("file", "-printf", "first\n", *operator, "-printf", "second\n") }

      at_every_optimisation_level do
        it "evaluates both from left to right" do
          expect(result).to print_lines("first", "second").in_order
        end
      end
    end
  end

  before { sandbox.touch("file") }

  context "without an operator between two expressions" do
    it_behaves_like "a conjunction"

    context "with a true test followed by a false parenthesised group" do
      subject(:result) { find("file", "-true", "(", "-false", ")") }

      it_behaves_like "a successful search"
    end

    context "with a true parenthesised group followed by a false test" do
      subject(:result) { find("file", "(", "-true", ")", "-false") }

      it_behaves_like "a successful search"
    end

    context "with a true test followed by a negated true test" do
      subject(:result) { find("file", "-true", "!", "-true") }

      it_behaves_like "a successful search"
    end
  end

  context "with -a" do
    it_behaves_like "a conjunction", "-a", upstream: "find.posix/and.exp"
    it_behaves_like "a binary operator recognised at the start of the expression", "-a"
  end

  context "with -and" do
    it_behaves_like "a conjunction", "-and", upstream: "find.gnu/gnuand.exp"
    it_behaves_like "a binary operator recognised at the start of the expression", "-and"
  end
end
