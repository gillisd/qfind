RSpec.describe "find -fstype" do
  before { sandbox.mkdir("tmp/sub") }

  context "with the type of the file system holding the files" do
    subject(:result) { find("tmp", "-fstype", MountTable.file_system_type(sandbox.root)) }

    it_behaves_like "a successful search", "tmp", "tmp/sub"
  end

  context "with that type in upper case" do
    subject(:result) { find("tmp", "-fstype", MountTable.file_system_type(sandbox.root).upcase) }

    it_behaves_like "a successful search"
  end

  context "with an unknown type" do
    subject(:result) { find("tmp", "-fstype", "nosuchfs") }

    it_behaves_like "a successful search"
  end

  context "with an unknown type negated" do
    subject(:result) { find("tmp", "!", "-fstype", "nosuchfs") }

    it_behaves_like "a successful search", "tmp", "tmp/sub"
  end

  context "without a type" do
    subject(:result) { find("tmp", "-fstype") }

    it_behaves_like "a rejected command line", "find: missing argument to `-fstype'"
  end
end
