RSpec.describe "find -fls" do
  include_context "with ls -dils listings"

  let(:last_modified) { Time.utc(2019, 12, 31, 23, 30) }

  before do
    sandbox.write("file", "x" * 5000)
    sandbox.chmod(0o640, "file")
    sandbox.utime("file", atime: last_modified, mtime: last_modified)
  end

  it_behaves_like "an action writing to an output file", "-fls"

  context "with a regular file" do
    subject(:written) { find("file", "-fls", "out").then { sandbox.read("out") } }

    it "writes the listing that -ls would print" do
      expect(written).to eq(listing("file", permissions: "-rw-r-----", date: "Dec 31  2019"))
    end
  end

  context "with POSIXLY_CORRECT set" do
    subject(:written) { find("file", "-fls", "out", env: { "POSIXLY_CORRECT" => "1" }).then { sandbox.read("out") } }

    it "counts blocks in 512-byte units" do
      expect(written).to eq(listing("file", permissions: "-rw-r-----", date: "Dec 31  2019", block_size: 512))
    end
  end

  context "with a symbolic link" do
    subject(:written) { find("link", "-fls", "out").then { sandbox.read("out") } }

    before { sandbox.symlink("file", "link") }

    it "writes the link followed by its target" do
      expect(written).to end_with(" link -> file\n")
    end
  end

  context "with unusual characters in a name" do
    subject(:written) { find("a b\nc#{7.chr}d", "-fls", "out").then { sandbox.read("out") } }

    before { sandbox.touch("a b\nc#{7.chr}d") }

    it "escapes them as -ls does" do
      expect(written).to end_with(" a\\ b\\nc\\007d\n")
    end
  end

  context "with /dev/stdout as the file and -print" do
    subject(:result) { find("file", "file", "-print", "-fls", "/dev/stdout") }

    it "shares standard output with -print" do
      expect(result).to print_exactly(
        "file\n#{listing("file", permissions: "-rw-r-----", date: "Dec 31  2019")}" * 2,
      )
    end
  end

  context "with /dev/stderr as the file" do
    subject(:result) { find("file", "-fls", "/dev/stderr") }

    it { is_expected.to print_nothing }

    it "writes the listing to standard error" do
      expect(result).to have_attributes(stderr: listing("file", permissions: "-rw-r-----", date: "Dec 31  2019"))
    end
  end

  context "with -fprint writing to the same file" do
    subject(:written) { find("file", "-fprint", "out", "-fls", "out").then { sandbox.read("out") } }

    it "writes both outputs through one stream" do
      expect(written).to eq("file\n#{listing("file", permissions: "-rw-r-----", date: "Dec 31  2019")}")
    end
  end
end
