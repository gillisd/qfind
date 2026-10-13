RSpec.describe "find -context" do
  def selinux_enabled? = File.foreach("/proc/self/mountinfo").any? { it.include?(" - selinuxfs ") }

  before { sandbox.touch("file") }

  context "when SELinux is not enabled" do
    before { skip "SELinux is enabled here" if selinux_enabled? }

    context "with a pattern matching every context" do
      subject(:result) { find(".", "-context", "*") }

      it_behaves_like "a rejected command line", "find: invalid predicate -context: SELinux is not enabled."
    end

    context "with an empty pattern" do
      subject(:result) { find(".", "-context", "") }

      it_behaves_like "a rejected command line", "find: invalid predicate -context: SELinux is not enabled."
    end
  end

  context "without a pattern" do
    subject(:result) { find(".", "-context") }

    it_behaves_like "a rejected command line", "find: missing argument to `-context'"
  end
end
