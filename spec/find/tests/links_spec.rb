RSpec.describe "find -links" do
  context "with files of four, five and six links", upstream: "find.posix/links.exp" do
    subject(:result) do
      find("tmp/file1", "tmp/file2", "tmp/file3", "-type", "f",
           "(", "(", "-links", "-5", "-exec", "echo", "under5", "{}", ";", ")",
           "-o", "(", "-links", "5", "-exec", "echo", "exactly5", "{}", ";", ")",
           "-o", "(", "-links", "+5", "-exec", "echo", "over5", "{}", ";", ")", ")")
    end

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/file1", "tmp/file2", "tmp/file3")
      ["a", "b", "c"].each { sandbox.link("tmp/file1", "tmp/1#{it}") }
      ["a", "b", "c", "d"].each { sandbox.link("tmp/file2", "tmp/2#{it}") }
      ["a", "b", "c", "d", "e"].each { sandbox.link("tmp/file3", "tmp/3#{it}") }
    end

    it_behaves_like "a successful search", "exactly5 tmp/file2", "over5 tmp/file3", "under5 tmp/file1"
  end

  context "with a file of one link and a file of two links" do
    before do
      sandbox.touch("single", "double")
      sandbox.link("double", "double-twin")
    end

    {
      "1" => ["./single"],
      "2" => ["./double", "./double-twin"],
      "+1" => ["./double", "./double-twin"],
      "-2" => ["./single"],
      "+2" => [],
      "-1" => [],
    }.each do |count, matches|
      context "with the argument #{count}" do
        subject(:result) { find(".", "-type", "f", "-links", count) }

        it_behaves_like "a successful search", *matches
      end
    end
  end

  context "with a symbolic link to a file of two links" do
    before do
      sandbox.touch("target")
      sandbox.link("target", "twin")
      sandbox.symlink("target", "link")
    end

    context "with the link count of the link itself" do
      subject(:result) { find("link", "-links", "1") }

      it_behaves_like "a successful search", "link"
    end

    context "with the link count of its target and -L" do
      subject(:result) { find("-L", "link", "-links", "2") }

      it_behaves_like "a successful search", "link"
    end
  end

  context "with an invalid count" do
    {
      "a word" => "x",
      "a fraction" => "1.5",
      "a sign before a negative number" => "+-1",
      "a lone sign" => "+",
      "a number too large to represent" => "99999999999999999999",
      "an empty argument" => "",
    }.each do |reason, count|
      context "with #{reason}" do
        subject(:result) { find(".", "-links", count) }

        it_behaves_like "a rejected command line", "find: non-numeric argument to -links: '#{count}'"
      end
    end
  end

  context "without a count" do
    subject(:result) { find(".", "-links") }

    it_behaves_like "a rejected command line", "find: missing argument to `-links'"
  end
end
