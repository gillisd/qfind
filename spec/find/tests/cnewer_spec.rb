RSpec.describe "find -cnewer" do
  context "with three files created in succession", upstream: "tests/find/newer.sh" do
    subject(:result) { find(".", "-cnewer", "file2", "-name", "file*") }

    include_context "with three files created in succession"

    it_behaves_like "a successful search", "./file3"
  end

  context "with a file changed after the reference was modified" do
    subject(:result) { find("changed", "-cnewer", "reference") }

    before do
      sandbox.touch("changed", "reference")
      sandbox.utime("changed", atime: Time.now - (30 * 86_400), mtime: Time.now - (30 * 86_400))
      sandbox.utime("reference", atime: Time.now - (20 * 86_400), mtime: Time.now - (10 * 86_400))
    end

    it_behaves_like "a successful search", "changed"
  end

  context "with a file changed before the reference was modified" do
    subject(:result) { find("changed", "-cnewer", "reference") }

    before do
      sandbox.touch("changed", "reference")
      sandbox.utime("reference", atime: Time.now, mtime: Time.now + 86_400)
    end

    it_behaves_like "a successful search"
  end

  context "with a file changed at the same instant as the reference was modified" do
    subject(:result) { find("changed", "-cnewer", "reference") }

    before do
      sandbox.touch("changed", "reference")
      sandbox.join("changed").ctime.then { sandbox.utime("reference", atime: it, mtime: it) }
    end

    it_behaves_like "a successful search"
  end

  context "without a reference file" do
    subject(:result) { find(".", "-cnewer") }

    it_behaves_like "a rejected command line", "find: missing argument to `-cnewer'"
  end

  it_behaves_like "a test comparing against a reference file", "-cnewer"
end
