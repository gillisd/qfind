RSpec.describe "find -writable" do
  context "with files of every owner access mode", :unprivileged, upstream: "find.gnu/access.exp" do
    subject(:result) { find("tmp", "-writable") }

    include_context "with files of every owner access mode"

    it_behaves_like "a successful search", "tmp", "tmp/w", "tmp/rw", "tmp/rwx"
  end

  context "with directories of every owner access mode", :unprivileged do
    subject(:result) { find("dirs", "-mindepth", "1", "-prune", "-writable") }

    include_context "with directories of every owner access mode"

    it_behaves_like "a successful search", "dirs/w", "dirs/wx", "dirs/rwx"
  end

  context "with links to an accessible file, an inaccessible file and nowhere", :unprivileged do
    subject(:result) { find("links", "-mindepth", "1", "-writable") }

    include_context "with links to an accessible file, an inaccessible file and nowhere"

    it_behaves_like "a successful search", "links/to-open"
  end

  context "when run by the superuser", :root do
    subject(:result) { find("tmp", "-writable") }

    include_context "with files of every owner access mode"

    it_behaves_like "a successful search", "tmp", "tmp/0", "tmp/r", "tmp/w", "tmp/x", "tmp/rw", "tmp/rwx"
  end
end
