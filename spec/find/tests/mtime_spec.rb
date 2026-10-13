RSpec.describe "find -mtime" do
  it_behaves_like "a day-based timestamp test", "-mtime", :mtime

  context "with a file modified just before the search", upstream: "find.posix/mtime0.exp" do
    subject(:result) { find("tmp", "-type", "f", "-mtime", "0") }

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/x")
    end

    it_behaves_like "a successful search", "tmp/x"
  end
end
