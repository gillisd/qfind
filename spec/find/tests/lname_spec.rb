RSpec.describe "find -lname" do
  context "with the target of a link", upstream: "find.gnu/lname.exp" do
    subject(:result) { find("tmp", "-lname", "one", "-print") }

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/one")
      sandbox.symlink("one", "tmp/two")
    end

    it_behaves_like "a successful search", "tmp/two"
  end

  it_behaves_like "a link-target pattern test", "-lname"

  context "with a pattern differing only in case from the target" do
    subject(:result) { find("tmp", "-lname", "ONE") }

    before do
      sandbox.mkdir("tmp")
      sandbox.symlink("one", "tmp/two")
    end

    it_behaves_like "a successful search"
  end
end
