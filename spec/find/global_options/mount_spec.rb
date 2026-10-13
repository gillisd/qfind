RSpec.describe "find -mount" do
  context "with a mount point of another device directly below the root directory",
          upstream: "tests/find/mount-vs-xdev.sh" do
    subject(:result) { find("/", "-maxdepth", "1", "-mount", "-path", mount_point, "-print") }

    include_context "with a mount point of another device directly below the root directory"

    it_behaves_like "a successful search"

    context "with -xdev as well" do
      subject(:result) { find("/", "-maxdepth", "1", "-mount", "-xdev", "-path", mount_point, "-print") }

      it_behaves_like "a successful search"
    end
  end

  context "with -L and links to a directory and a file on other devices" do
    subject(:result) { find("-L", ".", "-mount") }

    include_context "with links to a directory and a file on other devices"

    it_behaves_like "a successful search", ".", "./file", "./here", "./here/inside"

    context "with -xdev as well" do
      subject(:result) { find("-L", ".", "-mount", "-xdev") }

      it_behaves_like "a successful search", ".", "./file", "./here", "./here/inside"
    end
  end

  context "with links to a directory and a file on other devices that are not followed" do
    subject(:result) { find(".", "-mount") }

    include_context "with links to a directory and a file on other devices"

    it_behaves_like "a successful search", ".", "./file", "./here", "./here/inside", "./there", "./null"
  end

  context "with -L and a starting point on another device than the working directory" do
    subject(:result) { find("-L", "there", "-mount", "-maxdepth", "0") }

    include_context "with links to a directory and a file on other devices"

    it_behaves_like "a successful search", "there"
  end

  context "with a directory bind-mounted within the same file system", :root,
          upstream: "tests/find/mount-vs-xdev-bind.sh" do
    subject(:result) { find("-mount", "-path", "./mnt/f") }

    include_context "with a directory bind-mounted within the sandbox"

    it_behaves_like "a successful search", "./mnt/f"

    context "with -xdev as well" do
      subject(:result) { find("-mount", "-xdev", "-path", "./mnt/f") }

      it_behaves_like "a successful search", "./mnt/f"
    end
  end

  context "with a file bind-mounted within the same file system", :root,
          upstream: "tests/find/mount-vs-xdev-bind.sh" do
    subject(:result) { find("-mount", "-path", "./mntf") }

    include_context "with a file bind-mounted within the sandbox"

    it_behaves_like "a successful search", "./mntf"

    context "with -xdev as well" do
      subject(:result) { find("-mount", "-xdev", "-path", "./mntf") }

      it_behaves_like "a successful search", "./mntf"
    end
  end

  context "with another file system mounted below the starting point", :root,
          upstream: "tests/find/mount-vs-xdev-other-fs.sh" do
    subject(:result) { find("-mount", "-name", "file") }

    include_context "with an ext2 file system mounted within the sandbox"

    it_behaves_like "a successful search"

    context "with -xdev as well" do
      subject(:result) { find("-mount", "-xdev", "-name", "file") }

      it_behaves_like "a successful search"
    end

    context "when looking for the mount point itself" do
      subject(:result) { find("-mount", "-name", "mnt") }

      it_behaves_like "a successful search"
    end
  end

  context "with a file of another file system bind-mounted below the starting point", :root,
          upstream: "tests/find/mount-vs-xdev-other-fs.sh" do
    subject(:result) { find("-mount", "-name", "mntf") }

    include_context "with a file of a mounted ext2 file system bind-mounted within the sandbox"

    it_behaves_like "a successful search"

    context "with -xdev as well" do
      subject(:result) { find("-mount", "-xdev", "-name", "mntf") }

      it_behaves_like "a successful search"
    end
  end

  context "with a stray argument after it" do
    subject(:result) { find(".", "-mount", "1") }

    it_behaves_like "a rejected command line", "find: paths must precede expression: `1'"
  end
end
