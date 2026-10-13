RSpec.describe "find -type" do
  it_behaves_like "a test of file type letters", "-type"

  context "with an unknown type letter", upstream: "find.posix/typearg.exp" do
    subject(:result) { find(".", "-type", "Z") }

    it_behaves_like "a rejected command line", "find: Unknown argument to -type: Z"
  end

  context "with a word of unknown type letters", upstream: "find.posix/typearg.exp" do
    subject(:result) { find(".", "-type", "ZZ") }

    it_behaves_like "a rejected command line", "find: Unknown argument to -type: Z"
  end

  context "with a directory of every file type a user can create", upstream: "tests/find/type_list.sh" do
    include_context "with a directory of every file type a user can create"

    {
      "f" => ["dir/reg"],
      "d" => ["dir/dir"],
      "l" => ["dir/reg-link", "dir/dir-link", "dir/fifo-link", "dir/sock-link", "dir/dangling-link"],
      "p" => ["dir/fifo"],
      "s" => ["dir/sock"],
      "b" => [],
      "c" => [],
      "f,d" => ["dir/reg", "dir/dir"],
      "b,f,d" => ["dir/reg", "dir/dir"],
      "f,c,d" => ["dir/reg", "dir/dir"],
      "f,d,p" => ["dir/reg", "dir/dir", "dir/fifo"],
      "f,d,s" => ["dir/reg", "dir/dir", "dir/sock"],
      "f,l" => ["dir/reg", "dir/reg-link", "dir/dir-link", "dir/fifo-link", "dir/sock-link", "dir/dangling-link"],
      "f,d,b,c,p,s,l" => [
        "dir/reg", "dir/dir", "dir/fifo", "dir/sock",
        "dir/reg-link", "dir/dir-link", "dir/fifo-link", "dir/sock-link", "dir/dangling-link"
      ],
    }.each do |types, matches|
      context "with the argument #{types}" do
        subject(:result) { find("dir", "-mindepth", "1", "-type", types) }

        it_behaves_like "a successful search", *matches
      end
    end
  end

  context "with a directory of every file type including devices", :root, upstream: "tests/find/type_list.sh" do
    include_context "with a directory of every file type including devices"

    {
      "b" => ["dir/blk"],
      "c" => ["dir/chr"],
      "b,f,d" => ["dir/blk", "dir/reg", "dir/dir"],
      "f,c,d" => ["dir/chr", "dir/reg", "dir/dir"],
      "f,d,b,c,p,s,l" => [
        "dir/reg", "dir/dir", "dir/blk", "dir/chr", "dir/fifo", "dir/sock",
        "dir/reg-link", "dir/dir-link", "dir/blk-link", "dir/chr-link", "dir/fifo-link", "dir/sock-link",
        "dir/dangling-link"
      ],
    }.each do |types, matches|
      context "with the argument #{types}" do
        subject(:result) { find("dir", "-mindepth", "1", "-type", types) }

        it_behaves_like "a successful search", *matches
      end
    end
  end

  context "with a character device as starting point" do
    subject(:result) { find(File::NULL, "-type", "c") }

    it_behaves_like "a successful search", File::NULL
  end

  context "with a type list and POSIXLY_CORRECT" do
    subject(:result) { find("dir", "-mindepth", "1", "-type", "f,d", env: { "POSIXLY_CORRECT" => "1" }) }

    include_context "with a directory of every file type a user can create"

    it_behaves_like "a successful search", "dir/reg", "dir/dir"
  end

  context "with symbolic links followed by -L" do
    include_context "with a directory of every file type a user can create"

    {
      "l" => ["dir/dangling-link"],
      "f" => ["dir/reg", "dir/reg-link"],
      "d" => ["dir/dir", "dir/dir-link"],
      "p" => ["dir/fifo", "dir/fifo-link"],
      "s" => ["dir/sock", "dir/sock-link"],
    }.each do |types, matches|
      context "with the argument #{types}" do
        subject(:result) { find("-L", "dir", "-mindepth", "1", "-type", types) }

        it_behaves_like "a successful search", *matches
      end
    end
  end

  context "with symbolic links followed by -follow" do
    subject(:result) { find("dir", "-mindepth", "1", "-follow", "-type", "l") }

    include_context "with a directory of every file type a user can create"

    it_behaves_like "a successful search", "dir/dangling-link"
  end

  context "with symbolic links below a starting point and -H" do
    subject(:result) { find("-H", "dir", "-mindepth", "1", "-type", "l") }

    include_context "with a directory of every file type a user can create"

    it_behaves_like "a successful search",
                    "dir/reg-link", "dir/dir-link", "dir/fifo-link", "dir/sock-link", "dir/dangling-link"
  end

  context "with a symbolic link as starting point" do
    include_context "with a directory of every file type a user can create"

    context "with -P" do
      subject(:result) { find("-P", "dir/reg-link", "-type", "l") }

      it_behaves_like "a successful search", "dir/reg-link"
    end

    context "with -H" do
      subject(:result) { find("-H", "dir/reg-link", "-type", "f") }

      it_behaves_like "a successful search", "dir/reg-link"
    end
  end
end
