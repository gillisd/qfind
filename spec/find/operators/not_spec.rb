RSpec.describe "find ! expr" do
  shared_examples "a negation" do |operator, **metadata|
    context "with a test naming one of two sibling directories", **metadata do
      subject(:result) { find("tmp", operator, "-name", "fred", "-print") }

      before { sandbox.mkdir("tmp/fred", "tmp/jim") }

      it_behaves_like "a successful search", "tmp", "tmp/jim"
    end

    context "with a true operand" do
      subject(:result) { find("file", operator, "-true") }

      it_behaves_like "a successful search"
    end

    context "with a false operand" do
      subject(:result) { find("file", operator, "-false") }

      it_behaves_like "a successful search", "file"
    end

    context "when applied twice" do
      subject(:result) { find("file", operator, operator, "-true") }

      it_behaves_like "a successful search", "file"
    end

    context "with an action as operand" do
      subject(:result) { find("file", operator, "-printf", "acted\n", "-o", "-printf", "negated to false\n") }

      at_every_optimisation_level do
        it "performs the action yet evaluates to false" do
          expect(result).to print_lines("acted", "negated to false").in_order
        end
      end
    end

    context "when it ends the expression" do
      subject(:result) { find(".", operator) }

      it_behaves_like "a rejected command line", "find: expected an expression after '#{operator}'"
    end

    context "when it ends an expression containing an action" do
      subject(:result) { find(".", "-print", operator) }

      it_behaves_like "a rejected command line", "find: invalid expression"
    end

    context "when it directly precedes a closing parenthesis" do
      subject(:result) { find(".", "(", operator, ")") }

      it_behaves_like "a rejected command line", "find: expected an expression between '#{operator}' and ')'"
    end

    context "when it directly precedes a binary operator" do
      subject(:result) { find(".", operator, "-o", "-print") }

      it_behaves_like "a rejected command line",
                      "find: invalid expression; you have used a binary operator '-o' with nothing before it."
    end
  end

  before { sandbox.touch("file") }

  context "with !" do
    it_behaves_like "a negation", "!", upstream: "find.posix/posixnot.exp"
  end

  context "with -not" do
    it_behaves_like "a negation", "-not", upstream: "find.gnu/gnunot.exp"
  end

  context "with ! applied to -not" do
    subject(:result) { find("file", "!", "-not", "-true") }

    it_behaves_like "a successful search", "file"
  end

  context "with an argument merely beginning with !" do
    subject(:result) { find(".", "-true", "!x") }

    it_behaves_like "a rejected command line", "find: paths must precede expression: `!x'"
  end
end
