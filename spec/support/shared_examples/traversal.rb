RSpec.shared_context "with files at every depth down to three levels" do
  before do
    sandbox.mkdir("top/one/deep")
    sandbox.touch("top/file", "top/one/file", "top/one/deep/file")
  end
end

RSpec.shared_examples "a depth limit option" do |option|
  context "with 0 as level and -quit", upstream: "find.gnu/mindepth-arg.exp" do
    subject(:result) { find("tmp/top", option, "0", "-print", "-quit") }

    before { sandbox.mkdir("tmp/top/one") }

    it_behaves_like "a successful search", "tmp/top"
  end

  ["x", "0x01", "Fnord", "", "+1", "1.2", "-3", "3e1"].each do |level|
    context "with #{level.inspect} as level", upstream: "find.gnu/mindepth-badarg.exp" do
      subject(:result) { find("tmp/top", option, level, "-quit") }

      before { sandbox.mkdir("tmp/top/one") }

      it_behaves_like "a rejected command line",
                      "find: Expected a positive decimal integer argument to #{option}, but got '#{level}'"
    end
  end

  [" 1", "1 ", "-0"].each do |level|
    context "with #{level.inspect} as level" do
      subject(:result) { find(".", option, level) }

      it_behaves_like "a rejected command line",
                      "find: Expected a positive decimal integer argument to #{option}, but got '#{level}'"
    end
  end

  ["2147483648", "99999999999999999999"].each do |level|
    context "with #{level} as level" do
      subject(:result) { find(".", option, level) }

      it_behaves_like "a rejected command line", "find: #{level}: Numerical result out of range"
    end
  end

  context "with the largest int as level" do
    subject(:result) { find(".", option, "2147483647") }

    it { is_expected.to be_successful }
  end

  context "without a level" do
    subject(:result) { find(".", option) }

    it_behaves_like "a rejected command line", "find: missing argument to `#{option}'"
  end
end

RSpec.shared_context "with a mount point of another device directly below the root directory" do
  let(:mount_point) do
    ["/dev", "/home", "/proc", "/run", "/tmp"].find do |directory|
      File.directory?(directory) && File.lstat(directory).dev != File.stat("/").dev
    end
  end

  before { skip "cannot determine other device mounts in '/'" unless mount_point }
end

RSpec.shared_context "with links to a directory and a file on other devices" do
  let(:foreign_directory) do
    ["/proc", "/sys", "/dev", "/"].find { File.directory?(it) && File.stat(it).dev != sandbox.root.stat.dev }
  end

  before do
    skip "no directory on another device than #{sandbox.root}" unless foreign_directory
    skip "#{File::NULL} is on the same device as #{sandbox.root}" if File.stat(File::NULL).dev == sandbox.root.stat.dev
    sandbox.mkdir("here")
    sandbox.touch("file", "here/inside")
    sandbox.symlink(foreign_directory, "there")
    sandbox.symlink(File::NULL, "null")
  end
end

RSpec.shared_context "with a directory bind-mounted within the sandbox" do
  before do
    sandbox.mkdir("d", "mnt")
    sandbox.touch("d/f")
    mounted = system("mount", "--bind", sandbox.join("d").to_s, sandbox.join("mnt").to_s)
    skip "This test requires mount with a working --bind option." unless mounted
  end

  after { system("umount", sandbox.join("mnt").to_s, err: File::NULL) }
end

RSpec.shared_context "with a file bind-mounted within the sandbox" do
  before do
    sandbox.write("file", "test\n")
    sandbox.touch("mntf")
    mounted = system("mount", "--bind", sandbox.join("file").to_s, sandbox.join("mntf").to_s)
    skip "This test requires mount with a working --bind option." unless mounted
  end

  after { system("umount", sandbox.join("mntf").to_s, err: File::NULL) }
end

RSpec.shared_context "with an ext2 file system mounted within the sandbox" do
  before do
    sandbox.mkdir("d", "mnt")
    sandbox.touch("fs")
    sandbox.truncate("fs", 8192 * 200)
    created = system("mkfs", "-t", "ext2", "-F", sandbox.join("fs").to_s, out: File::NULL, err: File::NULL)
    skip "failed to create ext2 file system" unless created
    mounted = system("mount", "-oloop", sandbox.join("fs").to_s, sandbox.join("mnt").to_s)
    skip "insufficient mount/ext2 support" unless mounted
    sandbox.write("mnt/file", "test\n")
  end

  after { system("umount", sandbox.join("mnt").to_s, err: File::NULL) }
end

RSpec.shared_context "with a file of a mounted ext2 file system bind-mounted within the sandbox" do
  include_context "with an ext2 file system mounted within the sandbox"

  before do
    sandbox.touch("mntf")
    mounted = system("mount", "--bind", sandbox.join("mnt/file").to_s, sandbox.join("mntf").to_s)
    skip "This test requires mount with a working --bind option." unless mounted
  end

  after { system("umount", sandbox.join("mntf").to_s, err: File::NULL) }
end
