RSpec.describe "find -iwholename" do
  context "with a pattern differing in case from a directory below the starting point",
          upstream: "find.gnu/iwholename.exp" do
    subject(:result) { find("tmp/top", "-iwholename", "tmP/TOP/One", "-print") }

    before { sandbox.mkdir("tmp/top/one/two") }

    it_behaves_like "a successful search", "tmp/top/one"
  end

  it_behaves_like "a case-insensitive whole-path pattern test", "-iwholename"
end
