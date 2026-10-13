RSpec.describe "find -regex" do
  context "with a repeated group matching nested directories", upstream: "find.gnu/regex1.exp" do
    subject(:result) { find("tmp", "-regex", "tmp\\(/d\\)*", "-print") }

    before { sandbox.mkdir("tmp/d/d/d/e") }

    it_behaves_like "a successful search", "tmp", "tmp/d", "tmp/d/d", "tmp/d/d/d"
  end

  context "with the expression as the last argument", upstream: "find.gnu/sv-bug-17490.exp" do
    subject(:result) { find(".", "-maxdepth", "0", "-regex", "x") }

    it_behaves_like "a successful search"
  end

  it_behaves_like "a whole-path regular expression test", "-regex"

  context "with an expression differing only in case" do
    subject(:result) { find(".", "-regex", ".*FUBAR3") }

    before { sandbox.touch("fubar3", "FUBAR3") }

    it_behaves_like "a successful search", "./FUBAR3"
  end
end
