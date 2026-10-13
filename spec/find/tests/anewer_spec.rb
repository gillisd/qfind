RSpec.describe "find -anewer" do
  let(:instant) { Time.at(1_600_000_000, 123_456_789, :nsec) }

  context "with three files created in succession", upstream: "tests/find/newer.sh" do
    subject(:result) { find(".", "-anewer", "file2", "-name", "file*") }

    include_context "with three files created in succession"

    it_behaves_like "a successful search", "./file3"
  end

  context "with files accessed before and after the reference was modified" do
    subject(:result) { find("accessed-later", "accessed-between", "modified-later", "-anewer", "reference") }

    before do
      sandbox.touch("reference", "accessed-later", "accessed-between", "modified-later")
      sandbox.utime("reference", atime: Time.now - (20 * 86_400), mtime: Time.now - (10 * 86_400))
      sandbox.utime("accessed-later", atime: Time.now - (5 * 86_400), mtime: Time.now - (30 * 86_400))
      sandbox.utime("accessed-between", atime: Time.now - (15 * 86_400), mtime: Time.now - (30 * 86_400))
      sandbox.utime("modified-later", atime: Time.now - (30 * 86_400), mtime: Time.now - (5 * 86_400))
    end

    it_behaves_like "a successful search", "accessed-later"
  end

  context "with a file accessed at the same instant as the reference was modified" do
    subject(:result) { find("twin", "-anewer", "reference") }

    before do
      sandbox.touch("reference", "twin")
      sandbox.utime("reference", atime: instant - 1, mtime: instant)
      sandbox.utime("twin", atime: instant, mtime: instant - 1)
    end

    it_behaves_like "a successful search"
  end

  context "with a file accessed one nanosecond after the reference was modified" do
    subject(:result) { find("later", "-anewer", "reference") }

    before do
      sandbox.touch("reference", "later")
      sandbox.utime("reference", atime: instant - 1, mtime: instant)
      sandbox.utime("later", atime: instant + Rational(1, 1_000_000_000), mtime: instant - 1)
    end

    it_behaves_like "a successful search", "later"
  end

  context "without a reference file" do
    subject(:result) { find(".", "-anewer") }

    it_behaves_like "a rejected command line", "find: missing argument to `-anewer'"
  end

  it_behaves_like "a test comparing against a reference file", "-anewer"
end
