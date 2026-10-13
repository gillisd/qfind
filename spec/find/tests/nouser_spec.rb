RSpec.describe "find -nouser" do
  context "with a file owned by the current user" do
    before { sandbox.touch("mine") }

    context "without negation" do
      subject(:result) { find("mine", "-nouser") }

      it_behaves_like "a successful search"
    end

    context "with negation" do
      subject(:result) { find("mine", "!", "-nouser") }

      it_behaves_like "a successful search", "mine"
    end
  end

  context "with a file owned by a user id unknown to the system", :root do
    subject(:result) { find("orphan", "mine", "-nouser") }

    include_context "with a file owned by a user id unknown to the system"

    before { sandbox.touch("mine") }

    it_behaves_like "a successful search", "orphan"
  end
end
