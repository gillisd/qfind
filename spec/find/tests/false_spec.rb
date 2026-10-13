RSpec.describe "find -false" do
  before { sandbox.mkdir("tmp/fred/jim") }

  context "with a print on each side", upstream: "find.gnu/false.exp" do
    subject(:result) { find("tmp", "-depth", "-print", "-false", "-print") }

    it_behaves_like "a successful search", "tmp/fred/jim", "tmp/fred", "tmp"
  end

  context "with nothing else in the expression" do
    subject(:result) { find("tmp", "-false") }

    it_behaves_like "a successful search"
  end

  context "with an alternative after -o" do
    subject(:result) { find("tmp", "-false", "-o", "-print") }

    it_behaves_like "a successful search", "tmp", "tmp/fred", "tmp/fred/jim"
  end

  context "when negated" do
    subject(:result) { find("tmp", "!", "-false", "-print") }

    it_behaves_like "a successful search", "tmp", "tmp/fred", "tmp/fred/jim"
  end
end
