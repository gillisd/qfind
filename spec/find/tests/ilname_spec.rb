RSpec.describe "find -ilname" do
  context "with the target of a link in a different case", upstream: "find.gnu/ilname.exp" do
    subject(:result) { find("tmp", "-ilname", "One", "-print") }

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/one")
      sandbox.symlink("one", "tmp/two")
    end

    it_behaves_like "a successful search", "tmp/two"
  end

  it_behaves_like "a link-target pattern test", "-ilname"

  context "with a wildcard pattern in a different case from the target" do
    subject(:result) { find("tmp", "-ilname", "*PASSWD") }

    include_context "with a link to a file and a broken link"

    it_behaves_like "a successful search", "tmp/LINK"
  end

  context "with -L and a pattern in a different case from a broken target" do
    subject(:result) { find("-L", "tmp", "-ilname", "/nosuch*") }

    include_context "with a link to a file and a broken link"

    it_behaves_like "a successful search", "tmp/BROKEN"
  end
end
