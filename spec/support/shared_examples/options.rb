RSpec.shared_context "with links to a directory at and below the starting points" do
  before do
    sandbox.mkdir("dir", "top")
    sandbox.touch("dir/file")
    sandbox.symlink("dir", "dirlink")
    sandbox.symlink("../dir", "top/link")
  end
end

RSpec.shared_context "with a recent link to an old file as timestamp reference" do
  before do
    sandbox.touch("old", "mid")
    sandbox.utime("old", atime: Time.utc(2000), mtime: Time.utc(2000))
    sandbox.utime("mid", atime: Time.utc(2010), mtime: Time.utc(2010))
    sandbox.symlink("old", "ref")
  end
end

RSpec.shared_examples "a search dereferencing symbolic links" do |leading, trailing|
  context "with a link to a directory below the starting point" do
    subject(:result) { find(*leading, "top", *trailing) }

    include_context "with links to a directory at and below the starting points"

    it_behaves_like "a successful search", "top", "top/link", "top/link/file"
  end

  context "with a link to an ancestor directory" do
    subject(:result) { find(*leading, "top", *trailing) }

    before do
      sandbox.mkdir("top/sub")
      sandbox.symlink("..", "top/sub/up")
    end

    it_behaves_like "a failed search", "top", "top/sub"

    it "diagnoses the file system loop" do
      expect(result).to diagnose("find: File system loop detected; the following directory is part of the cycle: " \
                                 "'top/sub/up'")
    end
  end

  context "with two links pointing at each other" do
    subject(:result) { find(*leading, "tmp", *trailing) }

    before do
      sandbox.mkdir("tmp")
      sandbox.symlink("a", "tmp/b")
      sandbox.symlink("b", "tmp/a")
    end

    it_behaves_like "a failed search", "tmp"

    it { is_expected.to diagnose("find: 'tmp/a': Too many levels of symbolic links") }
  end

  context "with -type l" do
    subject(:result) { find(*leading, "tmp", *trailing, "-type", "l") }

    include_context "with a link to a file and a broken link"

    it_behaves_like "a successful search", "tmp/BROKEN"
  end

  ["-lname", "-ilname"].each do |test|
    context "with #{test}" do
      subject(:result) { find(*leading, "tmp", *trailing, test, "*") }

      include_context "with a link to a file and a broken link"

      it_behaves_like "a successful search", "tmp/BROKEN"
    end
  end

  ["-newer", "-anewer", "-cnewer", "-newermm"].each do |test|
    context "with a link as the reference file of #{test}" do
      subject(:result) { find(*leading, "mid", *trailing, test, "ref") }

      include_context "with a recent link to an old file as timestamp reference"

      it_behaves_like "a successful search", "mid"
    end
  end
end
