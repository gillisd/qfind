RSpec.describe "find -ipath" do
  context "with a pattern differing in case from a directory below the starting point",
          upstream: "find.gnu/ipath.exp" do
    subject(:result) { find("tmp/top", "-ipath", "Tmp/TOP/one", "-print") }

    before { sandbox.mkdir("tmp/top/ONE/two") }

    it_behaves_like "a successful search", "tmp/top/ONE"
  end

  it_behaves_like "a case-insensitive whole-path pattern test", "-ipath"
end
