RSpec.describe "find -xtype" do
  it_behaves_like "a test of file type letters", "-xtype"

  context "with an unknown type letter" do
    subject(:result) { find(".", "-xtype", "Z") }

    it_behaves_like "a rejected command line", "find: Unknown argument to -xtype: Z"
  end

  context "with links as starting points and below them under -H", upstream: "find.gnu/xtype.exp" do
    subject(:result) do
      find("-H", "tmp/LINK", "tmp/BROKEN", "tmp",
           "-xtype", "l", "-printf", "xtype is l: %p\n", ",",
           "!", "-xtype", "l", "-printf", "xtype is not l: %p\n")
    end

    include_context "with a link to a file and a broken link"

    it_behaves_like "a successful search",
                    "xtype is l: tmp/LINK",
                    "xtype is l: tmp/BROKEN",
                    "xtype is not l: tmp",
                    "xtype is not l: tmp/LINK",
                    "xtype is l: tmp/BROKEN"
  end

  context "with a starting point linking to a regular file", upstream: "find.gnu/xtype-symlink.exp" do
    subject(:result) { find("tmp/LINK", "-xtype", "f") }

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/file")
      sandbox.symlink("file", "tmp/LINK")
    end

    it_behaves_like "a successful search", "tmp/LINK"
  end

  context "with a directory of every file type a user can create", upstream: "tests/find/type_list.sh" do
    include_context "with a directory of every file type a user can create"

    {
      "f" => ["dir/reg", "dir/reg-link"],
      "d" => ["dir/dir", "dir/dir-link"],
      "l" => ["dir/dangling-link"],
      "p" => ["dir/fifo", "dir/fifo-link"],
      "s" => ["dir/sock", "dir/sock-link"],
      "b" => [],
      "c" => [],
      "f,d" => ["dir/reg", "dir/reg-link", "dir/dir", "dir/dir-link"],
      "b,f,d" => ["dir/reg", "dir/reg-link", "dir/dir", "dir/dir-link"],
      "f,c,d" => ["dir/reg", "dir/reg-link", "dir/dir", "dir/dir-link"],
      "f,d,p" => ["dir/reg", "dir/reg-link", "dir/dir", "dir/dir-link", "dir/fifo", "dir/fifo-link"],
      "f,d,s" => ["dir/reg", "dir/reg-link", "dir/dir", "dir/dir-link", "dir/sock", "dir/sock-link"],
      "f,l" => ["dir/reg", "dir/reg-link", "dir/dangling-link"],
      "f,d,b,c,p,s" => [
        "dir/reg", "dir/dir", "dir/fifo", "dir/sock",
        "dir/reg-link", "dir/dir-link", "dir/fifo-link", "dir/sock-link"
      ],
      "f,d,b,c,p,s,l" => [
        "dir/reg", "dir/dir", "dir/fifo", "dir/sock",
        "dir/reg-link", "dir/dir-link", "dir/fifo-link", "dir/sock-link", "dir/dangling-link"
      ],
    }.each do |types, matches|
      context "with the argument #{types}" do
        subject(:result) { find("dir", "-mindepth", "1", "-xtype", types) }

        it_behaves_like "a successful search", *matches
      end
    end

    context "when negated for symbolic links" do
      subject(:result) { find("dir", "-mindepth", "1", "-not", "-xtype", "l") }

      it_behaves_like "a successful search",
                      "dir/reg", "dir/dir", "dir/fifo", "dir/sock",
                      "dir/reg-link", "dir/dir-link", "dir/fifo-link", "dir/sock-link"
    end
  end

  context "with a directory of every file type including devices", :root, upstream: "tests/find/type_list.sh" do
    include_context "with a directory of every file type including devices"

    {
      "b" => ["dir/blk", "dir/blk-link"],
      "c" => ["dir/chr", "dir/chr-link"],
      "b,f,d" => ["dir/blk", "dir/blk-link", "dir/reg", "dir/reg-link", "dir/dir", "dir/dir-link"],
      "f,c,d" => ["dir/chr", "dir/chr-link", "dir/reg", "dir/reg-link", "dir/dir", "dir/dir-link"],
      "f,d,b,c,p,s" => [
        "dir/reg", "dir/dir", "dir/blk", "dir/chr", "dir/fifo", "dir/sock",
        "dir/reg-link", "dir/dir-link", "dir/blk-link", "dir/chr-link", "dir/fifo-link", "dir/sock-link"
      ],
      "f,d,b,c,p,s,l" => [
        "dir/reg", "dir/dir", "dir/blk", "dir/chr", "dir/fifo", "dir/sock",
        "dir/reg-link", "dir/dir-link", "dir/blk-link", "dir/chr-link", "dir/fifo-link", "dir/sock-link",
        "dir/dangling-link"
      ],
    }.each do |types, matches|
      context "with the argument #{types}" do
        subject(:result) { find("dir", "-mindepth", "1", "-xtype", types) }

        it_behaves_like "a successful search", *matches
      end
    end
  end

  context "with a link to a character device" do
    subject(:result) { find("null-link", "-xtype", "c") }

    before { sandbox.symlink(File::NULL, "null-link") }

    it_behaves_like "a successful search", "null-link"
  end

  context "with a type list and POSIXLY_CORRECT" do
    subject(:result) { find("dir", "-mindepth", "1", "-xtype", "f,d", env: { "POSIXLY_CORRECT" => "1" }) }

    include_context "with a directory of every file type a user can create"

    it_behaves_like "a successful search", "dir/reg", "dir/reg-link", "dir/dir", "dir/dir-link"
  end

  context "with symbolic links followed by -L" do
    include_context "with a directory of every file type a user can create"

    {
      "l" => ["dir/reg-link", "dir/dir-link", "dir/fifo-link", "dir/sock-link", "dir/dangling-link"],
      "f" => ["dir/reg"],
      "d" => ["dir/dir"],
    }.each do |types, matches|
      context "with the argument #{types}" do
        subject(:result) { find("-L", "dir", "-mindepth", "1", "-xtype", types) }

        it_behaves_like "a successful search", *matches
      end
    end
  end

  context "with symbolic links followed by -follow" do
    subject(:result) { find("dir", "-mindepth", "1", "-follow", "-xtype", "l") }

    include_context "with a directory of every file type a user can create"

    it_behaves_like "a successful search",
                    "dir/reg-link", "dir/dir-link", "dir/fifo-link", "dir/sock-link", "dir/dangling-link"
  end

  context "with a starting point linking to a regular file and -P" do
    subject(:result) { find("-P", "dir/reg-link", "-xtype", "f") }

    include_context "with a directory of every file type a user can create"

    it_behaves_like "a successful search", "dir/reg-link"
  end

  context "with a self-referencing link" do
    before do
      sandbox.mkdir("dir")
      sandbox.symlink("self", "dir/self")
    end

    context "without -L" do
      subject(:result) { find("dir", "-mindepth", "1", "-xtype", "l") }

      it_behaves_like "a successful search", "dir/self"
    end

    context "with -L" do
      subject(:result) { find("-L", "dir", "-mindepth", "1", "-xtype", "l") }

      it_behaves_like "a failed search"

      it { is_expected.to diagnose("find: 'dir/self': Too many levels of symbolic links") }
    end
  end

  context "with a link whose target cannot be examined", :unprivileged do
    subject(:result) { find("dir", "-mindepth", "1", "-xtype", "l") }

    before do
      sandbox.mkdir("dir", "locked")
      sandbox.touch("locked/file")
      sandbox.symlink("../locked/file", "dir/blocked")
      sandbox.chmod(0o000, "locked")
    end

    it_behaves_like "a failed search"

    it { is_expected.to diagnose("find: 'dir/blocked': Permission denied") }
  end
end
