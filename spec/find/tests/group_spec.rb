RSpec.describe "find -group" do
  it_behaves_like "a test matching an owner by name or id", "-group", "group name or GID" do
    let(:owner_name) { Etc.getgrgid(Process.gid).name }
    let(:owner_id) { Process.gid }
  end

  context "with a file owned by a group id unknown to the system", :root do
    subject(:result) { find("orphan", "-group", "4000000000") }

    include_context "with a file owned by a group id unknown to the system"

    it_behaves_like "a successful search", "orphan"
  end

  [":", "14JJ", "10.3", "-4"].each do |group|
    context "with #{group} as the group" do
      subject(:result) { find(".", "-group", group) }

      it_behaves_like "a rejected command line", "find: invalid group name or GID argument to -group: '#{group}'"
    end
  end

  context "with an empty group name", upstream: "find.posix/group-empty.exp" do
    subject(:result) { find(".", "-group", "") }

    it_behaves_like "a rejected command line", "find: invalid group name or GID argument to -group: ''"
  end

  context "without a group", upstream: "find.posix/group-missing.exp" do
    subject(:result) { find("tmp", "-group") }

    before { sandbox.touch("tmp") }

    it_behaves_like "a rejected command line", "find: missing argument to `-group'"
  end
end
