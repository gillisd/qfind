RSpec.describe "find -wholename" do
  context "with a pattern naming a directory below the starting point", upstream: "find.gnu/wholename.exp" do
    subject(:result) { find("tmp/top", "-wholename", "tmp/top/one", "-print") }

    before { sandbox.mkdir("tmp/top/one/two") }

    it_behaves_like "a successful search", "tmp/top/one"
  end

  it_behaves_like "a case-sensitive whole-path pattern test", "-wholename"
end
