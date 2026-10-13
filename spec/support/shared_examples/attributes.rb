RSpec.shared_context "with a directory of every file type a user can create" do
  before do
    sandbox.mkdir("dir/dir")
    sandbox.touch("dir/reg")
    sandbox.mkfifo("dir/fifo")
    sandbox.socket("dir/sock")
    ["reg", "dir", "fifo", "sock"].each { sandbox.symlink(it, "dir/#{it}-link") }
    sandbox.symlink("enoent", "dir/dangling-link")
  end
end

RSpec.shared_context "with a directory of every file type including devices" do
  include_context "with a directory of every file type a user can create"

  before do
    { "blk" => "b", "chr" => "c" }.each do |name, kind|
      created = system("mknod", sandbox.join("dir/#{name}").to_s, kind, "0", "0", err: File::NULL)
      skip "device files cannot be created here" unless created
      sandbox.symlink(name, "dir/#{name}-link")
    end
  end
end

RSpec.shared_context "with files of every owner access mode" do
  before do
    sandbox.mkdir("tmp")
    { "r" => 0o400, "w" => 0o200, "x" => 0o100, "0" => 0o000, "rw" => 0o600, "rwx" => 0o700 }.each do |name, mode|
      sandbox.touch("tmp/#{name}")
      sandbox.chmod(mode, "tmp/#{name}")
    end
  end
end

RSpec.shared_context "with directories of every owner access mode" do
  before do
    { "0" => 0o000, "r" => 0o400, "w" => 0o200, "x" => 0o100, "rx" => 0o500, "wx" => 0o300, "rwx" => 0o700 }
      .each do |name, mode|
        sandbox.mkdir("dirs/#{name}")
        sandbox.chmod(mode, "dirs/#{name}")
      end
  end
end

RSpec.shared_context "with links to an accessible file, an inaccessible file and nowhere" do
  before do
    sandbox.mkdir("links")
    sandbox.touch("open", "secret")
    sandbox.chmod(0o700, "open")
    sandbox.chmod(0o000, "secret")
    sandbox.symlink("../open", "links/to-open")
    sandbox.symlink("../secret", "links/to-secret")
    sandbox.symlink("nowhere", "links/dangling")
  end
end

RSpec.shared_examples "a test of file type letters" do |primary|
  include_context "with a directory of every file type a user can create"

  context "without a type list" do
    subject(:result) { find("dir", primary) }

    it_behaves_like "a rejected command line", "find: missing argument to `#{primary}'"
  end

  context "with an empty type list", upstream: "tests/find/type_list.sh" do
    subject(:result) { find("dir", "-mindepth", "1", primary, "") }

    it_behaves_like "a rejected command line", "find: Arguments to #{primary} should contain at least one letter"
  end

  context "with type letters not separated by commas", upstream: "tests/find/type_list.sh" do
    subject(:result) { find("dir", "-mindepth", "1", primary, "fd") }

    it_behaves_like "a rejected command line", "find: Must separate multiple arguments to #{primary} using: ','"
  end

  context "with a trailing comma", upstream: "tests/find/type_list.sh" do
    subject(:result) { find("dir", "-mindepth", "1", primary, "f,") }

    it_behaves_like "a rejected command line",
                    "find: Last file type in list argument to #{primary} is missing, i.e., list is ending on: ','"
  end

  context "with a repeated type letter", upstream: "tests/find/type_list.sh" do
    subject(:result) { find("dir", "-mindepth", "1", primary, "f,f") }

    it_behaves_like "a rejected command line", "find: Duplicate file type 'f' in the argument list to #{primary}."
  end

  context "with a leading comma" do
    subject(:result) { find("dir", primary, ",f") }

    it_behaves_like "a rejected command line", "find: Unknown argument to #{primary}: ,"
  end

  context "with an unknown letter after a known one" do
    subject(:result) { find("dir", primary, "f,x") }

    it_behaves_like "a rejected command line", "find: Unknown argument to #{primary}: x"
  end

  context "with the door type letter on a platform without doors" do
    subject(:result) { find("dir", primary, "D") }

    it_behaves_like "a rejected command line",
                    "find: #{primary} D is not supported because Solaris doors are not supported on the platform " \
                    "find was compiled on."
  end
end
