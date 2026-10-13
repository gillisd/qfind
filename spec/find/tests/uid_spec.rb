RSpec.describe "find -uid" do
  it_behaves_like "a test comparing numeric owner ids", "-uid" do
    let(:owner_id) { Process.uid }
  end

  context "with a file owned by a user id unknown to the system", :root do
    include_context "with a file owned by a user id unknown to the system"

    context "with that id" do
      subject(:result) { find("orphan", "-uid", "4000000000") }

      it_behaves_like "a successful search", "orphan"
    end

    context "with a plus sign before a lower id" do
      subject(:result) { find("orphan", "-uid", "+3999999999") }

      it_behaves_like "a successful search", "orphan"
    end
  end
end
