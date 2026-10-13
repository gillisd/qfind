RSpec.describe "find -empty" do
  context "with an empty and a non-empty regular file", upstream: "find.gnu/empty.exp" do
    subject(:result) { find("tmp", "-type", "f", "-empty") }

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/empty")
      sandbox.write("tmp/notempty", "\n")
    end

    it_behaves_like "a successful search", "tmp/empty"
  end

  context "with an empty and a non-empty directory" do
    subject(:result) { find("tmp", "-type", "d", "-empty") }

    before do
      sandbox.mkdir("tmp/empty", "tmp/notempty")
      sandbox.touch("tmp/notempty/file")
    end

    it_behaves_like "a successful search", "tmp/empty"
  end

  context "with a directory holding only a hidden file" do
    subject(:result) { find("dir", "-type", "d", "-empty") }

    before do
      sandbox.mkdir("dir")
      sandbox.touch("dir/.hidden")
    end

    it_behaves_like "a successful search"
  end

  context "with an empty directory as starting point" do
    subject(:result) { find("dir", "-empty") }

    before { sandbox.mkdir("dir") }

    it_behaves_like "a successful search", "dir"
  end

  context "with files that are neither regular files nor directories" do
    subject(:result) { find("tmp", "-mindepth", "1", "-empty") }

    before do
      sandbox.mkdir("tmp", "emptydir")
      sandbox.touch("emptyfile")
      sandbox.mkfifo("tmp/fifo")
      sandbox.socket("tmp/sock")
      sandbox.symlink("../emptyfile", "tmp/file-link")
      sandbox.symlink("../emptydir", "tmp/dir-link")
      sandbox.symlink("nowhere", "tmp/dangling-link")
    end

    it_behaves_like "a successful search"
  end

  context "with a character device" do
    subject(:result) { find(File::NULL, "-empty") }

    it_behaves_like "a successful search"
  end

  context "with symbolic links to an empty file and an empty directory followed by -L" do
    subject(:result) { find("-L", "tmp", "-mindepth", "1", "-empty") }

    before do
      sandbox.mkdir("tmp", "emptydir")
      sandbox.touch("emptyfile")
      sandbox.symlink("../emptyfile", "tmp/file-link")
      sandbox.symlink("../emptydir", "tmp/dir-link")
    end

    it_behaves_like "a successful search", "tmp/file-link", "tmp/dir-link"
  end

  context "with a directory that cannot be read", :unprivileged do
    subject(:result) { find("locked", "-maxdepth", "0", "-empty") }

    before do
      sandbox.mkdir("locked")
      sandbox.chmod(0o000, "locked")
    end

    it_behaves_like "a failed search"

    it { is_expected.to diagnose("find: 'locked': Permission denied") }
  end
end
