RSpec.describe "find ( expr )" do
  before { sandbox.touch("file") }

  context "with ! before a group of two alternatives", upstream: "find.posix/grouping.exp" do
    subject(:result) { find("tmp", "!", "(", "-name", "fred", "-o", "-name", "tmp", ")", "-print") }

    before { sandbox.mkdir("tmp/fred", "tmp/jim") }

    it_behaves_like "a successful search", "tmp/jim"
  end

  context "with a global option alone in a group", upstream: "find.posix/bracket-depth.exp" do
    subject(:result) { find("tmp", "(", "-depth", ")", "-false", "-print") }

    before { sandbox.mkdir("tmp") }

    it_behaves_like "a successful search"
  end

  context "with two alternatives grouped before an action" do
    subject(:result) { find("afile", "bfile", "(", "-name", "afile", "-o", "-name", "bfile", ")", "-print") }

    before { sandbox.touch("afile", "bfile") }

    it_behaves_like "a successful search", "afile", "bfile"
  end

  context "with groups nested in groups" do
    subject(:result) { find("file", "(", "(", "(", "-true", ")", ")", ")") }

    it_behaves_like "a successful search", "file"
  end

  context "with a false group" do
    subject(:result) { find("file", "(", "-false", ")") }

    it_behaves_like "a successful search"
  end

  context "with empty parentheses" do
    subject(:result) { find(".", "(", ")") }

    it_behaves_like "a rejected command line", "find: invalid expression; empty parentheses are not allowed."
  end

  context "with empty parentheses nested in a group" do
    subject(:result) { find(".", "(", "(", ")", ")") }

    it_behaves_like "a rejected command line", "find: invalid expression; empty parentheses are not allowed."
  end

  context "with empty parentheses closed by the deprecated -) form", upstream: "find.posix/empty-parens.exp" do
    subject(:result) { find(".", "(", "-)") }

    it_behaves_like "a rejected command line", "find: invalid expression; empty parentheses are not allowed."
  end

  context "with an opening parenthesis ending the expression" do
    subject(:result) { find(".", "(") }

    it_behaves_like "a rejected command line",
                    "find: invalid expression; expected to find a ')' but didn't see one. " \
                    "Perhaps you need an extra predicate after '('"
  end

  [
    ["without a closing parenthesis", ["(", "-name", "file"]],
    ["without a closing parenthesis in an expression containing an action", ["(", "-print"]],
    ["with one closing parenthesis fewer than opening ones", ["(", "(", "-true", ")"]],
  ].each do |description, expression|
    context description do
      subject(:result) { find(".", *expression) }

      it_behaves_like "a rejected command line",
                      "find: invalid expression; I was expecting to find a ')' somewhere but did not see one."
    end
  end

  [
    ["with one closing parenthesis more than opening ones", ["(", "-true", ")", ")"]],
    ["with a closing parenthesis after an action", ["-print", ")"]],
  ].each do |description, expression|
    context description do
      subject(:result) { find(".", *expression) }

      it_behaves_like "a rejected command line", "find: you have too many ')'"
    end
  end

  ["(", ")"].each do |parenthesis|
    context "with an argument merely beginning with #{parenthesis}" do
      subject(:result) { find(".", "-true", "#{parenthesis}x") }

      it_behaves_like "a rejected command line", "find: paths must precede expression: `#{parenthesis}x'"
    end
  end
end
