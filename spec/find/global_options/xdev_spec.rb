RSpec.describe "find -xdev" do
  context "with a mount point of another device directly below the root directory",
          upstream: "tests/find/mount-vs-xdev.sh" do
    subject(:result) { find("/", "-maxdepth", "1", "-xdev", "-path", mount_point, "-print") }

    include_context "with a mount point of another device directly below the root directory"

    at_every_optimisation_level do
      it { is_expected.to be_successful }

      it "visits the mount point" do
        expect(result).to print_lines(mount_point)
      end

      it { is_expected.to diagnose_nothing }
    end
  end

  context "with -L and links to a directory and a file on other devices" do
    subject(:result) { find("-L", ".", "-xdev") }

    include_context "with links to a directory and a file on other devices"

    it_behaves_like "a successful search", ".", "./file", "./here", "./here/inside", "./there", "./null"
  end

  context "with -L, -depth and links to a directory and a file on other devices" do
    subject(:result) { find("-L", ".", "-depth", "-xdev") }

    include_context "with links to a directory and a file on other devices"

    it "still visits the directory on another device after declining to descend into it" do
      expect(result).to print_lines(".", "./file", "./here", "./here/inside", "./there", "./null")
    end
  end

  context "with a directory bind-mounted within the same file system", :root,
          upstream: "tests/find/mount-vs-xdev-bind.sh" do
    subject(:result) { find("-xdev", "-path", "./mnt/f") }

    include_context "with a directory bind-mounted within the sandbox"

    it_behaves_like "a successful search", "./mnt/f"
  end

  context "with a file bind-mounted within the same file system", :root,
          upstream: "tests/find/mount-vs-xdev-bind.sh" do
    subject(:result) { find("-xdev", "-path", "./mntf") }

    include_context "with a file bind-mounted within the sandbox"

    it_behaves_like "a successful search", "./mntf"
  end

  context "with another file system mounted below the starting point", :root,
          upstream: "tests/find/mount-vs-xdev-other-fs.sh" do
    subject(:result) { find("-xdev", "-name", "file") }

    include_context "with an ext2 file system mounted within the sandbox"

    it_behaves_like "a successful search"

    context "when looking for the mount point itself" do
      subject(:result) { find("-xdev", "-name", "mnt") }

      it_behaves_like "a successful search", "./mnt"
    end
  end

  context "with a file of another file system bind-mounted below the starting point", :root,
          upstream: "tests/find/mount-vs-xdev-other-fs.sh" do
    subject(:result) { find("-xdev", "-name", "mntf") }

    include_context "with a file of a mounted ext2 file system bind-mounted within the sandbox"

    it_behaves_like "a successful search", "./mntf"
  end

  context "with a stray argument after it" do
    subject(:result) { find(".", "-xdev", "1") }

    it_behaves_like "a rejected command line", "find: paths must precede expression: `1'"
  end
end
