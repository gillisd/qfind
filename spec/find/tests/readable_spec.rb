RSpec.describe "find -readable" do
  context "with files of every owner access mode", :unprivileged, upstream: "find.gnu/access.exp" do
    subject(:result) { find("tmp", "-readable") }

    include_context "with files of every owner access mode"

    it_behaves_like "a successful search", "tmp", "tmp/r", "tmp/rw", "tmp/rwx"
  end

  context "with directories of every owner access mode", :unprivileged do
    subject(:result) { find("dirs", "-mindepth", "1", "-prune", "-readable") }

    include_context "with directories of every owner access mode"

    it_behaves_like "a successful search", "dirs/r", "dirs/rx", "dirs/rwx"
  end

  context "with links to an accessible file, an inaccessible file and nowhere", :unprivileged do
    subject(:result) { find("links", "-mindepth", "1", "-readable") }

    include_context "with links to an accessible file, an inaccessible file and nowhere"

    it_behaves_like "a successful search", "links/to-open"
  end

  context "when run by the superuser", :root do
    subject(:result) { find("tmp", "-readable") }

    include_context "with files of every owner access mode"

    it_behaves_like "a successful search", "tmp", "tmp/0", "tmp/r", "tmp/w", "tmp/x", "tmp/rw", "tmp/rwx"
  end
end
