RSpec.describe "find directory traversal" do
  {
    "neither read nor searched" => 0o000,
    "searched but not read" => 0o311,
  }.each do |access, mode|
    context "with a directory that can be #{access}", :unprivileged do
      subject(:result) { find("tmp") }

      before do
        sandbox.mkdir("tmp/dir", "tmp/other")
        sandbox.touch("tmp/dir/file", "tmp/other/file")
        sandbox.chmod(mode, "tmp/dir")
      end

      it_behaves_like "a failed search", "tmp", "tmp/dir", "tmp/other", "tmp/other/file"

      it { is_expected.to diagnose("find: 'tmp/dir': Permission denied") }
    end
  end

  context "with a starting point that cannot be read", :unprivileged do
    subject(:result) { find("dir", "other") }

    before do
      sandbox.mkdir("dir", "other")
      sandbox.touch("dir/file", "other/file")
      sandbox.chmod(0o000, "dir")
    end

    it_behaves_like "a failed search", "dir", "other", "other/file"

    it { is_expected.to diagnose("find: 'dir': Permission denied") }
  end

  context "with several directories that cannot be read", :unprivileged do
    subject(:result) { find("tmp") }

    before do
      sandbox.mkdir("tmp/one", "tmp/two")
      sandbox.chmod(0o000, "tmp/one", "tmp/two")
    end

    it "diagnoses each of them" do
      expect(result).to diagnose_exactly("find: 'tmp/one': Permission denied", "find: 'tmp/two': Permission denied")
    end
  end

  context "with a directory bind-mounted within the same file system", :root,
          upstream: "tests/find/mount-vs-xdev-bind.sh" do
    subject(:result) { find("-path", "./mnt/f") }

    include_context "with a directory bind-mounted within the sandbox"

    it_behaves_like "a successful search", "./mnt/f"
  end

  context "with a file bind-mounted within the same file system", :root,
          upstream: "tests/find/mount-vs-xdev-bind.sh" do
    subject(:result) { find("-path", "./mntf") }

    include_context "with a file bind-mounted within the sandbox"

    it_behaves_like "a successful search", "./mntf"
  end

  context "with another file system mounted below the starting point", :root,
          upstream: "tests/find/mount-vs-xdev-other-fs.sh" do
    subject(:result) { find("-name", "file") }

    include_context "with an ext2 file system mounted within the sandbox"

    it_behaves_like "a successful search", "./mnt/file"
  end

  context "with a file of another file system bind-mounted below the starting point", :root,
          upstream: "tests/find/mount-vs-xdev-other-fs.sh" do
    subject(:result) { find("-name", "mntf") }

    include_context "with a file of a mounted ext2 file system bind-mounted within the sandbox"

    it_behaves_like "a successful search", "./mntf"
  end

  context "with a directory of 200000 entries", :expensive, upstream: "tests/find/many-dir-entries-vs-OOM.sh" do
    subject(:result) { find_with_stdout_to(File::NULL, "dir", rlimit_as: (address_space_for_dir2 + 35_000) * 1024) }

    let(:address_space_for_dir2) do
      coarse = (5_000..50_000).step(5_000).find { dir2_fits_in?(it) }
      skip "find does not run within 50000 KiB of address space" unless coarse
      (1_000...coarse).step(1_000).reverse_each.take_while { dir2_fits_in?(it) }.last || coarse
    end

    before do
      sandbox.mkdir("dir", "dir2")
      sandbox.touch(*(1..200_000).map { "dir/#{it}" }, "dir2/a", "dir2/b", "dir2/c")
    end

    def dir2_fits_in?(kibibytes)
      find("dir2", "-fprint", "dummy", rlimit_as: kibibytes * 1024, rlimit_core: 0).successful?
    end

    it "needs at most 35000 KiB more address space than for a directory of three entries" do
      expect(result).to be_successful
    end
  end
end
