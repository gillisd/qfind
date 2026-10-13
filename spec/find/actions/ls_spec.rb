RSpec.describe "find -ls" do
  include_context "with ls -dils listings"

  let(:last_modified) { Time.utc(2019, 12, 31, 23, 30) }

  shared_context "with a file last modified at the end of 2019" do
    before do
      sandbox.write("file", "x" * 5000)
      sandbox.chmod(0o640, "file")
      sandbox.utime("file", atime: last_modified, mtime: last_modified)
    end
  end

  context "with a regular file" do
    subject(:result) { find("file", "-ls") }

    include_context "with a file last modified at the end of 2019"

    it "lists inode, blocks in KiB, permissions, links, owner, group, size, date and name" do
      expect(result).to print_exactly(listing("file", permissions: "-rw-r-----", date: "Dec 31  2019"))
    end
  end

  context "with POSIXLY_CORRECT set" do
    subject(:result) { find("file", "-ls", env: { "POSIXLY_CORRECT" => "1" }) }

    include_context "with a file last modified at the end of 2019"

    it "counts blocks in 512-byte units" do
      expect(result).to print_exactly(listing("file", permissions: "-rw-r-----", date: "Dec 31  2019", block_size: 512))
    end
  end

  context "with a time zone nine hours east of UTC" do
    subject(:result) { find("file", "-ls", env: { "TZ" => "JST-9" }) }

    include_context "with a file last modified at the end of 2019"

    it "shows the date in that time zone" do
      expect(result).to print_exactly(listing("file", permissions: "-rw-r-----", date: "Jan  1  2020"))
    end
  end

  context "with a file that has a second hard link" do
    subject(:result) { find("file", "-ls") }

    include_context "with a file last modified at the end of 2019"

    before { sandbox.link("file", "other") }

    it "shows the link count" do
      expect(result).to print_exactly(listing("file", permissions: "-rw-r-----", date: "Dec 31  2019"))
    end
  end

  context "with a directory" do
    subject(:result) { find("dir", "-ls") }

    before do
      sandbox.mkdir("dir")
      sandbox.chmod(0o750, "dir")
      sandbox.utime("dir", atime: last_modified, mtime: last_modified)
    end

    it "lists it with the type d" do
      expect(result).to print_exactly(listing("dir", permissions: "drwxr-x---", date: "Dec 31  2019"))
    end
  end

  context "with a symbolic link" do
    subject(:result) { find("link", "-ls") }

    before do
      sandbox.touch("file")
      sandbox.symlink("file", "link")
      sandbox.lutime("link", atime: last_modified, mtime: last_modified)
    end

    it "lists the link itself with the type l, followed by its target" do
      expect(result).to print_exactly(
        listing("link", permissions: "lrwxrwxrwx", date: "Dec 31  2019", name: "link -> file"),
      )
    end
  end

  context "with a named pipe" do
    subject(:result) { find("pipe", "-ls") }

    before do
      sandbox.mkfifo("pipe")
      sandbox.chmod(0o640, "pipe")
      sandbox.utime("pipe", atime: last_modified, mtime: last_modified)
    end

    it "lists it with the type p" do
      expect(result).to print_exactly(listing("pipe", permissions: "prw-r-----", date: "Dec 31  2019"))
    end
  end

  context "with a socket" do
    subject(:result) { find("socket", "-ls") }

    before do
      sandbox.socket("socket")
      sandbox.chmod(0o750, "socket")
      sandbox.utime("socket", atime: last_modified, mtime: last_modified)
    end

    it "lists it with the type s" do
      expect(result).to print_exactly(listing("socket", permissions: "srwxr-x---", date: "Dec 31  2019"))
    end
  end

  context "with a character device" do
    subject(:result) { find(File::NULL, "-ls") }

    let(:device) { File.lstat(File::NULL) }

    it "lists it with the type c" do
      expect(result.stdout).to include(" crw-rw-rw- ")
    end

    it "shows the major and minor device numbers in place of the size" do
      expect(result.stdout).to include(format(" %<major>3d, %<minor>3d ", major: device.rdev_major,
                                                                          minor: device.rdev_minor))
    end
  end

  {
    0o4755 => "-rwsr-xr-x",
    0o4644 => "-rwSr--r--",
    0o2755 => "-rwxr-sr-x",
    0o2644 => "-rw-r-Sr--",
  }.each do |mode, permissions|
    context "with a file of mode #{mode.to_s(8)}" do
      subject(:result) { find("file", "-ls") }

      include_context "with a file last modified at the end of 2019"

      before { sandbox.chmod(mode, "file") }

      it "shows the permissions as #{permissions}" do
        expect(result).to print_exactly(listing("file", permissions:, date: "Dec 31  2019"))
      end
    end
  end

  {
    0o1777 => "drwxrwxrwt",
    0o1776 => "drwxrwxrwT",
  }.each do |mode, permissions|
    context "with a directory of mode #{mode.to_s(8)}" do
      subject(:result) { find("dir", "-ls") }

      before do
        sandbox.mkdir("dir")
        sandbox.chmod(mode, "dir")
        sandbox.utime("dir", atime: last_modified, mtime: last_modified)
      end

      it "shows the permissions as #{permissions}" do
        expect(result).to print_exactly(listing("dir", permissions:, date: "Dec 31  2019"))
      end
    end
  end

  {
    "179 days ago" => -179 * 86_400,
    "half an hour from now" => 1_800,
  }.each do |description, offset|
    context "with a file modified #{description}" do
      subject(:result) { find("file", "-ls") }

      let(:modified) { Time.at(Time.now.to_i + offset).utc }

      before do
        sandbox.touch("file")
        sandbox.utime("file", atime: modified, mtime: modified)
      end

      it "shows the time of day instead of the year" do
        expect(result.stdout).to end_with(modified.strftime(" %b %e %H:%M file\n"))
      end
    end
  end

  {
    "181 days ago" => -181 * 86_400,
    "two hours from now" => 7_200,
  }.each do |description, offset|
    context "with a file modified #{description}" do
      subject(:result) { find("file", "-ls") }

      let(:modified) { Time.at(Time.now.to_i + offset).utc }

      before do
        sandbox.touch("file")
        sandbox.utime("file", atime: modified, mtime: modified)
      end

      it "shows the year instead of the time of day" do
        expect(result.stdout).to end_with(modified.strftime(" %b %e  %Y file\n"))
      end
    end
  end

  context "with -L and a symbolic link" do
    subject(:result) { find("-L", "link", "-ls") }

    include_context "with a file last modified at the end of 2019"

    before { sandbox.symlink("file", "link") }

    it "describes the file the link points to under the name of the link" do
      expect(result).to print_exactly(listing("file", permissions: "-rw-r-----", date: "Dec 31  2019", name: "link"))
    end
  end

  context "with a directory hierarchy" do
    subject(:result) { find("dir", "-ls") }

    before do
      sandbox.mkdir("dir")
      sandbox.touch("dir/file")
    end

    it "lists every file on a line of its own" do
      expect(result).to print_lines(a_string_ending_with(" dir"), a_string_ending_with(" dir/file"))
    end
  end

  context "when followed by -print" do
    subject(:result) { find("file", "-ls", "-print") }

    before { sandbox.touch("file") }

    it "evaluates to true" do
      expect(result).to print_lines(a_string_ending_with(" file"), "file").in_order
    end
  end

  {
    "a space" => ["a b", "a\\ b"],
    "a newline" => ["a\nb", "a\\nb"],
    "a tab" => ["a\tb", "a\\tb"],
    "a carriage return" => ["a\rb", "a\\rb"],
    "a form feed" => ["a#{12.chr}b", "a\\fb"],
    "a backspace" => ["a#{8.chr}b", "a\\bb"],
    "a vertical tab" => ["a#{11.chr}b", "a\\013b"],
    "a backslash" => ["a\\b", "a\\\\b"],
    "a double quote" => ["a\"b", "a\\\"b"],
    "a bell" => ["a#{7.chr}b", "a\\007b"],
    "an escape character" => ["a#{27.chr}b", "a\\033b"],
    "a delete character" => ["a#{127.chr}b", "a\\177b"],
    "a byte outside ASCII" => ["a#{0xe9.chr}b", "a\\351b"],
    "a single quote" => ["a'b", "a'b"],
    "other printable characters" => ["!$%&()*+,-.:;<=>?@[]^_`{|}~#", "!$%&()*+,-.:;<=>?@[]^_`{|}~#"],
  }.each do |description, (name, escaped)|
    context "with #{description} in the name" do
      subject(:result) { find(name, "-ls") }

      before { sandbox.touch(name) }

      it "prints the name as #{escaped}" do
        expect(result.stdout).to end_with(" #{escaped}\n")
      end
    end
  end

  context "with a UTF-8 name in a UTF-8 locale" do
    subject(:result) { find("café", "-ls", env: { "LC_ALL" => "C.UTF-8" }) }

    before { sandbox.touch("café") }

    it "escapes each byte outside ASCII in octal" do
      expect(result.stdout).to end_with(" caf\\303\\251\n")
    end
  end

  context "with unusual characters in the target of a symbolic link" do
    subject(:result) { find("link", "-ls") }

    before { sandbox.symlink("t a\\r\"get\n", "link") }

    it "escapes the target in the same way" do
      expect(result.stdout).to end_with(" link -> t\\ a\\\\r\\\"get\\n\n")
    end
  end

  context "when standard output is a terminal" do
    subject(:result) { find_with_terminal_output("a\nb", "-ls") }

    before { sandbox.touch("a\nb") }

    it "still escapes unusual characters" do
      expect(result.stdout).to end_with(" a\\nb\n")
    end
  end

  context "when standard output is full" do
    subject(:result) { find_with_stdout_to("/dev/full", "file", "-ls") }

    before { sandbox.touch("file") }

    it { is_expected.to have_exit_status(1) }
    it { is_expected.to diagnose("find: 'standard output': No space left on device") }
  end

  context "with FIND_BLOCK_SIZE set" do
    subject(:result) { find("file", "-ls", env: { "FIND_BLOCK_SIZE" => "512" }) }

    before { sandbox.touch("file") }

    it_behaves_like "a rejected command line",
                    "find: The environment variable FIND_BLOCK_SIZE is not supported, the only thing that affects " \
                    "the block size is the POSIXLY_CORRECT environment variable"
  end
end
