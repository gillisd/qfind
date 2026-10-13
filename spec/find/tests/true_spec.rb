RSpec.describe "find -true" do
  before { sandbox.mkdir("tmp/fred/jim") }

  context "with a print on each side", upstream: "find.gnu/true.exp" do
    subject(:result) { find("tmp", "-depth", "-print", "-true", "-print") }

    it_behaves_like "a successful search", "tmp/fred/jim", "tmp/fred/jim", "tmp/fred", "tmp/fred", "tmp", "tmp"

    it "prints each file twice before moving on to the next" do
      expect(result).to print_lines("tmp/fred/jim", "tmp/fred/jim", "tmp/fred", "tmp/fred", "tmp", "tmp").in_order
    end
  end

  context "with nothing else in the expression" do
    subject(:result) { find("tmp", "-true") }

    it_behaves_like "a successful search", "tmp", "tmp/fred", "tmp/fred/jim"
  end

  context "with an alternative after -o" do
    subject(:result) { find("tmp", "-true", "-o", "-print") }

    it_behaves_like "a successful search"
  end

  context "when negated" do
    subject(:result) { find("tmp", "!", "-true", "-print") }

    it_behaves_like "a successful search"
  end
end
