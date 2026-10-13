RSpec.describe "find -iregex" do
  context "with a repeated group matching nested directories of either case", upstream: "find.gnu/iregex1.exp" do
    subject(:result) { find("tmp", "-iregex", "tmp\\(/d\\)*", "-print") }

    before { sandbox.mkdir("tmp/d/D/d/e") }

    it_behaves_like "a successful search", "tmp", "tmp/d", "tmp/d/D", "tmp/d/D/d"
  end

  it_behaves_like "a whole-path regular expression test", "-iregex"

  context "with an expression differing only in case" do
    subject(:result) { find(".", "-iregex", ".*FUBAR3") }

    before { sandbox.touch("fubar3", "FUBAR3", "fubar4") }

    it_behaves_like "a successful search", "./fubar3", "./FUBAR3"
  end

  context "with a bracket expression" do
    subject(:result) { find(".", "-iregex", "\\./[a-c]+") }

    before { sandbox.touch("abc", "ABC", "xyz") }

    it_behaves_like "a successful search", "./abc", "./ABC"
  end
end
