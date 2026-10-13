RSpec.shared_examples "a binary operator requiring two operands" do |operator|
  [
    ["directly follows an opening parenthesis", ["(", operator, "-true", ")"]],
    ["directly follows a negation", ["!", operator, "-true"]],
    ["is written twice in a row", ["-true", operator, operator, "-true"]],
  ].each do |position, expression|
    context "when it #{position}" do
      subject(:result) { find(".", *expression) }

      it_behaves_like "a rejected command line",
                      "find: invalid expression; you have used a binary operator '#{operator}' with nothing before it."
    end
  end

  context "when it directly precedes a closing parenthesis" do
    subject(:result) { find(".", "(", "-true", operator, ")") }

    it_behaves_like "a rejected command line", "find: expected an expression between '#{operator}' and ')'"
  end

  context "when it ends the expression" do
    subject(:result) { find(".", "-true", operator) }

    it_behaves_like "a rejected command line", "find: expected an expression after '#{operator}'"
  end

  context "when it ends an expression containing an action" do
    subject(:result) { find(".", "-print", operator) }

    it_behaves_like "a rejected command line", "find: invalid expression"
  end
end

RSpec.shared_examples "a binary operator recognised at the start of the expression" do |operator|
  it_behaves_like "a binary operator requiring two operands", operator

  context "when it opens the expression" do
    subject(:result) { find(".", operator, "-print") }

    it_behaves_like "a rejected command line",
                    "find: invalid expression; you have used a binary operator '#{operator}' with nothing before it."
  end
end
