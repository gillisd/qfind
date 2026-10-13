RSpec.describe "find -samefile" do
  context "with copies of the reference file", upstream: "find.gnu/samefile-copy.exp" do
    subject(:result) { find("tmp/one", "tmp/two", "-samefile", "tmp/file1", "-print") }

    before do
      sandbox.mkdir("tmp/one", "tmp/two")
      sandbox.touch("tmp/file1", "tmp/file2")
      sandbox.write("tmp/one/link", sandbox.read("tmp/file1"))
      sandbox.write("tmp/two/link", sandbox.read("tmp/file2"))
    end

    it_behaves_like "a successful search"
  end

  context "with a hard link to the reference file", upstream: "find.gnu/samefile-link.exp" do
    subject(:result) { find("tmp/one", "tmp/two", "-samefile", "tmp/file1", "-print") }

    before do
      sandbox.mkdir("tmp/one", "tmp/two")
      sandbox.touch("tmp/file1", "tmp/file2")
      sandbox.link("tmp/file1", "tmp/one/link")
      sandbox.symlink("tmp/file1", "tmp/one/symlink")
      sandbox.link("tmp/file2", "tmp/two/link")
    end

    it_behaves_like "a successful search", "tmp/one/link"
  end

  context "with the reference file reached twice", upstream: "find.gnu/samefile-same.exp" do
    subject(:result) { find("tmp", "tmp/file", "-samefile", "tmp/file", "-print") }

    before do
      sandbox.mkdir("tmp/one", "tmp/two")
      sandbox.touch("tmp/file")
    end

    it_behaves_like "a successful search", "tmp/file", "tmp/file"
  end

  context "with symbolic links followed by -L", upstream: "find.gnu/samefile-symlink.exp" do
    subject(:result) { find("-L", "tmp/one", "tmp/two", "-samefile", "tmp/file1", "-print") }

    before do
      sandbox.mkdir("tmp/one", "tmp/two")
      sandbox.touch("tmp/file1", "tmp/file2")
      sandbox.link("tmp/file1", "tmp/one/link")
      sandbox.symlink("../file1", "tmp/one/symlink")
      sandbox.link("tmp/file2", "tmp/two/link")
      sandbox.symlink("../file2", "tmp/two/symlink")
    end

    it_behaves_like "a successful search", "tmp/one/link", "tmp/one/symlink"
  end

  context "with a broken symbolic link as reference under -P", upstream: "find.gnu/samefile-p-brokenlink.exp" do
    subject(:result) { find("-P", "tmp", "-samefile", "tmp/broken", "-print") }

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/file1")
      sandbox.link("tmp/file1", "tmp/link")
      sandbox.symlink("file1", "tmp/symlink")
      sandbox.symlink("tmp/file2", "tmp/broken")
      sandbox.link("tmp/broken", "tmp/blink")
    end

    it_behaves_like "a successful search", "tmp/blink", "tmp/broken"
  end

  context "with a broken symbolic link as reference under -L" do
    subject(:result) { find("-L", ".", "-samefile", "broken") }

    before do
      sandbox.touch("file")
      sandbox.symlink("nowhere", "broken")
    end

    it_behaves_like "a successful search", "./broken"
  end

  context "with a symbolic link to a file of two names as reference" do
    before do
      sandbox.touch("file")
      sandbox.link("file", "twin")
      sandbox.symlink("file", "link")
    end

    context "with -P" do
      subject(:result) { find("-P", ".", "-samefile", "link") }

      it_behaves_like "a successful search", "./link"
    end

    context "with -H" do
      subject(:result) { find("-H", ".", "-samefile", "link") }

      it_behaves_like "a successful search", "./file", "./twin"
    end

    context "with -L" do
      subject(:result) { find("-L", ".", "-samefile", "link") }

      it_behaves_like "a successful search", "./file", "./twin", "./link"
    end

    context "with -follow before the test" do
      subject(:result) { find(".", "-follow", "-samefile", "link") }

      it_behaves_like "a successful search", "./file", "./twin", "./link"
    end

    context "with -follow after the test" do
      subject(:result) { find(".", "-samefile", "link", "-follow") }

      it_behaves_like "a successful search"
    end
  end

  context "with a reference file that does not exist" do
    subject(:result) { find(".", "-samefile", "nosuch") }

    it_behaves_like "a rejected command line", "find: 'nosuch': No such file or directory"
  end

  context "with a reference file that cannot be examined", :unprivileged do
    subject(:result) { find(".", "-samefile", "locked/file") }

    before do
      sandbox.mkdir("locked")
      sandbox.touch("locked/file")
      sandbox.chmod(0o000, "locked")
    end

    it_behaves_like "a rejected command line", "find: 'locked/file': Permission denied"
  end

  context "without a reference file", upstream: "find.gnu/samefile-missing.exp" do
    subject(:result) { find(".", "-samefile") }

    it_behaves_like "a rejected command line", "find: missing argument to `-samefile'"
  end
end
