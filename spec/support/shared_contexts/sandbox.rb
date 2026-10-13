RSpec.shared_context "with a sandbox" do
  let(:sandbox) { Sandbox.new }
  let(:optimisation_level) { nil }

  after { sandbox.remove }
end
