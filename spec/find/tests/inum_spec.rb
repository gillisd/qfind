RSpec.describe "find -inum" do
  def inode_of(path) = sandbox.join(path).stat.ino

  def inode_number_zero?(path) = File.exist?(path) && File.stat(path).ino.zero?

  context "with the inode number of one of two files", upstream: "find.gnu/inum.exp" do
    subject(:result) { find("tmp", "tmp2", "-inum", inode_of("tmp").to_s, "-print") }

    before { sandbox.touch("tmp", "tmp2") }

    it_behaves_like "a successful search", "tmp"
  end

  context "with the inode number of a file with two names" do
    subject(:result) { find(".", "-inum", inode_of("file").to_s) }

    before do
      sandbox.touch("file", "other")
      sandbox.link("file", "twin")
    end

    it_behaves_like "a successful search", "./file", "./twin"
  end

  context "with a number compared to the inode number of a file" do
    before { sandbox.touch("file") }

    {
      "a plus sign and one less than the inode number" => ["+", -1, ["file"]],
      "a plus sign and the inode number itself" => ["+", 0, []],
      "a minus sign and one more than the inode number" => ["-", 1, ["file"]],
      "a minus sign and the inode number itself" => ["-", 0, []],
    }.each do |description, (sign, offset, matches)|
      context "with #{description}" do
        subject(:result) { find("file", "-inum", "#{sign}#{inode_of("file") + offset}") }

        it_behaves_like "a successful search", *matches
      end
    end
  end

  context "with a file whose inode number is zero", upstream: "tests/find/inode-zero.sh" do
    before { skip "no file with inode number 0 here" unless inode_number_zero?("/dev/console") }

    context "with zero" do
      subject(:result) { find("/dev/console", "-inum", "0") }

      it_behaves_like "a successful search", "/dev/console"
    end

    context "with less than one" do
      subject(:result) { find("/dev/console", "-inum", "-1") }

      it_behaves_like "a successful search", "/dev/console"
    end

    context "with an unrelated inode number" do
      subject(:result) { find("/dev/console", "-inum", "12345") }

      it_behaves_like "a successful search"
    end

    context "when printing the inode number" do
      subject(:result) { find("/dev/console", "-printf", "%i\n") }

      it_behaves_like "a successful search", "0"
    end
  end

  context "with a non-numeric inode number" do
    subject(:result) { find(".", "-inum", "x") }

    it_behaves_like "a rejected command line", "find: non-numeric argument to -inum: 'x'"
  end

  context "with a sign before a negative inode number" do
    subject(:result) { find(".", "-inum", "+-1") }

    it_behaves_like "a rejected command line", "find: non-numeric argument to -inum: '+-1'"
  end

  context "without an inode number" do
    subject(:result) { find(".", "-inum") }

    it_behaves_like "a rejected command line", "find: missing argument to `-inum'"
  end
end
