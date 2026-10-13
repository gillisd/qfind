RSpec.describe "find dash-prefixed operators" do
  def warning_about(operator)
    "find: warning: operator '#{operator}' (with leading dash '-') will no longer be accepted in future " \
      "findutils releases!"
  end

  shared_examples "a deprecated operator spelling" do |*operators|
    it { is_expected.to be_successful }

    operators.each do |operator|
      it "warns that #{operator} will no longer be accepted" do
        expect(result).to diagnose(warning_about(operator))
      end
    end
  end

  before { sandbox.touch("file") }

  context "with -(, -!, -, and -) in one expression", upstream: "tests/find/operators-wrong-with-dash.sh" do
    subject(:result) { find("-(", "-!", "-not", "-type", "c", "-,", "-type", "b", "-)") }

    it { is_expected.to be_successful }

    it "warns about each of them in command-line order" do
      expect(result).to diagnose_exactly(*["-(", "-!", "-,", "-)"].map { warning_about(it) }).in_order
    end
  end

  context "with -!" do
    context "when it precedes a false test" do
      subject(:result) { find("file", "-!", "-false") }

      it_behaves_like "a deprecated operator spelling", "-!"

      it "negates like !" do
        expect(result).to print_lines("file")
      end
    end

    context "when it ends the expression" do
      subject(:result) { find(".", "-!") }

      it_behaves_like "a rejected command line", "find: expected an expression after '-!'"
    end
  end

  context "with -( and -)" do
    context "when they enclose a false -a expression after !" do
      subject(:result) { find("file", "!", "-(", "-false", "-a", "-false", "-)") }

      it_behaves_like "a deprecated operator spelling", "-(", "-)"

      it "groups like ( and )" do
        expect(result).to print_lines("file")
      end
    end

    context "when -( is never closed" do
      subject(:result) { find(".", "-(", "-true") }

      it_behaves_like "a rejected command line",
                      "find: invalid expression; I was expecting to find a ')' somewhere but did not see one."
    end

    context "when -) begins the expression" do
      subject(:result) { find(".", "-)") }

      it_behaves_like "a rejected command line",
                      "find: invalid expression: expected expression before closing parentheses '-)'."
    end
  end

  context "with -," do
    context "when it joins a false and a true test" do
      subject(:result) { find("file", "-false", "-,", "-true") }

      it_behaves_like "a deprecated operator spelling", "-,"

      it "evaluates to its second operand like ," do
        expect(result).to print_lines("file")
      end
    end

    it_behaves_like "a binary operator recognised at the start of the expression", "-,"
  end

  context "with -nowarn" do
    subject(:result) { find("file", "-nowarn", "-!", "-false") }

    it "still warns" do
      expect(result).to diagnose(warning_about("-!"))
    end
  end

  context "with POSIXLY_CORRECT set" do
    subject(:result) { find("file", "-!", "-false", env: { "POSIXLY_CORRECT" => "1" }) }

    it "still warns" do
      expect(result).to diagnose(warning_about("-!"))
    end
  end

  context "with the same operator twice" do
    subject(:result) { find("file", "-!", "-!", "-true") }

    it "warns once for each occurrence" do
      expect(result).to diagnose_exactly(warning_about("-!"), warning_about("-!"))
    end
  end

  ["--(", "--!", "--,", "--)"].each do |operator|
    context "with two leading dashes, as in #{operator}" do
      subject(:result) { find(".", operator) }

      it_behaves_like "a rejected command line", "find: unknown predicate `#{operator}'"
    end
  end
end
