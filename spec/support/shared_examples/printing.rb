require "etc"

RSpec.shared_examples "a successful search printing exactly" do |output|
  at_every_optimisation_level do
    it { is_expected.to be_successful }
    it { is_expected.to print_exactly(output) }
    it { is_expected.to diagnose_nothing }
  end
end

RSpec.shared_examples "an action writing to an output file" do |action, *arguments|
  before { sandbox.touch("file") }

  context "without a file name" do
    subject(:result) { find("file", action) }

    it_behaves_like "a rejected command line", "find: missing argument to `#{action}'"
  end

  context "with a file that does not exist yet" do
    subject(:result) { find("file", "-false", action, "out", *arguments) }

    it "creates the file even though nothing matches" do
      expect { result }.to change { sandbox.exist?("out") }.from(false).to(true)
    end
  end

  context "with a file that already exists" do
    subject(:result) { find("file", "-false", action, "out", *arguments) }

    before { sandbox.write("out", "stale") }

    it "truncates the file even though nothing matches" do
      expect { result }.to change { sandbox.read("out") }.from("stale").to("")
    end
  end

  context "with a file in a directory that does not exist" do
    subject(:result) { find("file", action, "missing/out", *arguments) }

    it_behaves_like "a rejected command line", "find: 'missing/out': No such file or directory"
  end

  context "with a directory in place of the file" do
    subject(:result) { find("file", action, "dir", *arguments) }

    before { sandbox.mkdir("dir") }

    it_behaves_like "a rejected command line", "find: 'dir': Is a directory"
  end

  context "with an empty file name" do
    subject(:result) { find("file", action, "", *arguments) }

    it_behaves_like "a rejected command line", "find: '': No such file or directory"
  end

  context "with a file that cannot be written", :unprivileged do
    subject(:result) { find("file", action, "out", *arguments) }

    before do
      sandbox.touch("out")
      sandbox.chmod(0o444, "out")
    end

    it_behaves_like "a rejected command line", "find: 'out': Permission denied"
  end

  context "with a file on a full device" do
    subject(:result) { find("file", action, "/dev/full", *arguments) }

    it_behaves_like "a failed search"

    it { is_expected.to diagnose("find: '/dev/full': No space left on device") }
  end
end

RSpec.shared_context "with ls -dils listings" do
  def listing(path, permissions:, date:, block_size: 1024, name: path)
    status = File.lstat(sandbox.join(path))
    format(
      "%<inode>9d %<blocks>6d %<permissions>s %<links>3d %<owner>-8s %<group>-8s %<size>8d %<date>s %<name>s\n",
      inode: status.ino, blocks: (status.blocks * 512).ceildiv(block_size), permissions:, links: status.nlink,
      owner: Etc.getpwuid(status.uid).name, group: Etc.getgrgid(status.gid).name, size: status.size, date:, name:
    )
  end
end
