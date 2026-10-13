RSpec.describe "find -newer" do
  let(:instant) { Time.at(1_600_000_000, 123_456_789, :nsec) }

  context "with three files created in succession", upstream: "tests/find/newer.sh" do
    subject(:result) { find(".", "-newer", "file2", "-name", "file*") }

    include_context "with three files created in succession"

    it_behaves_like "a successful search", "./file3"
  end

  context "with files modified before and after the reference" do
    subject(:result) { find("modified-earlier", "modified-later", "accessed-later", "-newer", "reference") }

    before do
      sandbox.touch("reference", "modified-earlier", "modified-later", "accessed-later")
      sandbox.utime("reference", atime: Time.now - (10 * 86_400), mtime: Time.now - (10 * 86_400))
      sandbox.utime("modified-earlier", atime: Time.now - (20 * 86_400), mtime: Time.now - (20 * 86_400))
      sandbox.utime("modified-later", atime: Time.now - (20 * 86_400), mtime: Time.now - (5 * 86_400))
      sandbox.utime("accessed-later", atime: Time.now - (5 * 86_400), mtime: Time.now - (20 * 86_400))
    end

    it_behaves_like "a successful search", "modified-later"
  end

  context "with a file modified at the same instant as the reference" do
    subject(:result) { find("twin", "-newer", "reference") }

    before do
      sandbox.touch("reference", "twin")
      sandbox.utime("reference", atime: instant, mtime: instant)
      sandbox.utime("twin", atime: instant, mtime: instant)
    end

    it_behaves_like "a successful search"
  end

  context "with a file modified one nanosecond after the reference" do
    subject(:result) { find("later", "-newer", "reference") }

    before do
      sandbox.touch("reference", "later")
      sandbox.utime("reference", atime: instant, mtime: instant)
      sandbox.utime("later", atime: instant, mtime: instant + Rational(1, 1_000_000_000))
    end

    it_behaves_like "a successful search", "later"
  end

  context "without a reference file" do
    subject(:result) { find(".", "-newer") }

    it_behaves_like "a rejected command line", "find: missing argument to `-newer'"
  end

  it_behaves_like "a test comparing against a reference file", "-newer"
end
