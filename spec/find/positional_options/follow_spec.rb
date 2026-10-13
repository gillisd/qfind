RSpec.describe "find -follow" do
  context "with links as starting points", upstream: "find.gnu/follow-basic.exp" do
    subject(:result) { find("tmp/LINK", "tmp/BROKEN", "tmp", "-follow", "-printf", "%y %d %p\n") }

    include_context "with a link to a file and a broken link"

    it_behaves_like "a successful search",
                    "d 0 tmp", "f 0 tmp/LINK", "f 1 tmp/LINK", "l 0 tmp/BROKEN", "l 1 tmp/BROKEN"
  end

  it_behaves_like "a search dereferencing symbolic links", [], ["-follow"]

  ["-newer", "-anewer", "-cnewer", "-newermm"].each do |test|
    context "with a link as the reference file of #{test} placed before it" do
      subject(:result) { find("mid", test, "ref", "-follow") }

      include_context "with a recent link to an old file as timestamp reference"

      it_behaves_like "a successful search"
    end

    context "with -H and a link as the reference file of #{test} placed before it" do
      subject(:result) { find("-H", "mid", test, "ref", "-follow") }

      include_context "with a recent link to an old file as timestamp reference"

      it_behaves_like "a successful search", "mid"
    end
  end

  [["-type", "l"], ["-lname", "*"]].each do |test|
    context "with #{test.join(" ")} placed before it" do
      subject(:result) { find("tmp", *test, "-follow") }

      include_context "with a link to a file and a broken link"

      it_behaves_like "a successful search", "tmp/BROKEN"
    end
  end

  context "with -P" do
    subject(:result) { find("-P", "top", "-follow") }

    include_context "with links to a directory at and below the starting points"

    it_behaves_like "a successful search", "top", "top/link", "top/link/file"
  end

  context "when placed after a test on a terminal" do
    subject(:result) { find_on_terminal("top", "-name", "nothing", "-follow") }

    before { sandbox.mkdir("top") }

    it "is not reported as a misplaced global option" do
      expect(result).to diagnose_nothing
    end
  end
end
