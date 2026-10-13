RSpec.describe "find operator precedence" do
  before { sandbox.touch("file") }

  context "with ! ( -false -o -true ), negating the whole group" do
    subject(:result) { find("file", "!", "(", "-false", "-o", "-true", ")") }

    it_behaves_like "a successful search"
  end

  {
    "! -false -a -false" => ["( ! -false ) -a -false", []],
    "! -true -o -true" => ["( ! -true ) -o -true", ["file"]],
    "! -true , -true" => ["( ! -true ) , -true", ["file"]],
    "-false -a -false -o -true" => ["( -false -a -false ) -o -true", ["file"]],
    "-true -o -true -a -false" => ["-true -o ( -true -a -false )", ["file"]],
    "-true -o -true -false" => ["-true -o ( -true -false )", ["file"]],
    "-false -a -true , -true" => ["( -false -a -true ) , -true", ["file"]],
    "-true -o -false , -false" => ["( -true -o -false ) , -false", []],
  }.each do |expression, (reading, lines)|
    context "with #{expression}, read as #{reading}" do
      subject(:result) { find("file", *expression.split) }

      it_behaves_like "a successful search", *lines
    end
  end

  context "with -name afile -o -name bfile -print, read as -name afile -o ( -name bfile -a -print )" do
    subject(:result) { find(".", "-name", "afile", "-o", "-name", "bfile", "-print") }

    before { sandbox.touch("afile", "bfile") }

    it_behaves_like "a successful search", "./bfile"
  end

  context "with the same expression grouped explicitly as find reads it" do
    subject(:result) { find(".", "-name", "afile", "-o", "(", "-name", "bfile", "-a", "-print", ")") }

    before { sandbox.touch("afile", "bfile") }

    it_behaves_like "a successful search", "./bfile"
  end
end
