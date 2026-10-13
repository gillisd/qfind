RSpec.describe "find -nogroup" do
  context "with a file owned by the current group" do
    before { sandbox.touch("mine") }

    context "without negation" do
      subject(:result) { find("mine", "-nogroup") }

      it_behaves_like "a successful search"
    end

    context "with negation" do
      subject(:result) { find("mine", "!", "-nogroup") }

      it_behaves_like "a successful search", "mine"
    end
  end

  context "with a file owned by a group id unknown to the system", :root do
    subject(:result) { find("orphan", "mine", "-nogroup") }

    include_context "with a file owned by a group id unknown to the system"

    before { sandbox.touch("mine") }

    it_behaves_like "a successful search", "orphan"
  end
end
