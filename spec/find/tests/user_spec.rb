RSpec.describe "find -user" do
  it_behaves_like "a test matching an owner by name or id", "-user", "user name or UID" do
    let(:owner_name) { Etc.getpwuid(Process.uid).name }
    let(:owner_id) { Process.uid }
  end

  context "with a file owned by a user id unknown to the system", :root do
    subject(:result) { find("orphan", "-user", "4000000000") }

    include_context "with a file owned by a user id unknown to the system"

    it_behaves_like "a successful search", "orphan"
  end

  [":", "14JJ", "10.3", "-4"].each do |user|
    context "with #{user} as the user", upstream: "find.gnu/user-invalid.exp" do
      subject(:result) { find(".", "-user", user) }

      it_behaves_like "a rejected command line", "find: invalid user name or UID argument to -user: '#{user}'"
    end
  end

  context "with nothing after the test", upstream: "find.gnu/user-invalid.exp" do
    subject(:result) { find(".", "-user") }

    it_behaves_like "a rejected command line", "find: missing argument to `-user'"
  end

  context "with an empty user name", upstream: "find.posix/user-empty.exp" do
    subject(:result) { find(".", "-user", "") }

    it_behaves_like "a rejected command line", "find: invalid user name or UID argument to -user: ''"
  end

  context "without a user", upstream: "find.posix/user-missing.exp" do
    subject(:result) { find("tmp", "-user") }

    it_behaves_like "a rejected command line", "find: missing argument to `-user'"
  end
end
