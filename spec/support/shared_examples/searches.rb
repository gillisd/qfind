RSpec.shared_examples "a successful search" do |*lines|
  at_every_optimisation_level do
    it { is_expected.to be_successful }
    it { is_expected.to print_lines(*lines) }
    it { is_expected.to diagnose_nothing }
  end
end

RSpec.shared_examples "a successful search that warns" do |warning, *lines|
  at_every_optimisation_level do
    it { is_expected.to be_successful }
    it { is_expected.to print_lines(*lines) }
    it { is_expected.to diagnose(warning) }
  end
end

RSpec.shared_examples "a failed search" do |*lines|
  at_every_optimisation_level do
    it { is_expected.to have_exit_status(1) }
    it { is_expected.to print_lines(*lines) }
  end
end

RSpec.shared_examples "a rejected command line" do |diagnostic|
  at_every_optimisation_level do
    it { is_expected.to have_exit_status(1) }
    it { is_expected.to print_nothing }
    it { is_expected.to diagnose(diagnostic) }
  end
end
