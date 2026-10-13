RSpec.describe "find with unusual characters in file names" do
  context "when -print writes to a pipe" do
    subject(:result) { find(".", "-name", "bell*") }

    before { sandbox.touch("bell#{7.chr}name") }

    it_behaves_like "a successful search", "./bell#{7.chr}name"
  end

  context "when -print writes to a terminal" do
    subject(:result) { find_with_terminal_output(".", "-name", "bell*") }

    before { sandbox.touch("bell#{7.chr}name") }

    it_behaves_like "a successful search", "./bell?name"
  end

  context "when -fprint writes to /dev/stdout on a terminal" do
    subject(:result) { find_with_terminal_output(".", "-name", "bell*", "-fprint", "/dev/stdout") }

    before { sandbox.touch("bell#{7.chr}name") }

    it_behaves_like "a successful search", "./bell?name"
  end

  context "when -print0 writes to a terminal" do
    subject(:result) { find_with_terminal_output(".", "-name", "bell*", "-print0") }

    before { sandbox.touch("bell#{7.chr}name") }

    it { is_expected.to print_exactly("./bell#{7.chr}name#{0.chr}") }
  end

  context "when -printf writes to a terminal" do
    before do
      sandbox.mkdir("dir#{7.chr}name")
      sandbox.touch("dir#{7.chr}name/fi\tle")
    end

    {
      "%p" => "dir?name/fi?le",
      "%f" => "fi?le",
      "%h" => "dir?name",
      "%P" => "fi?le",
      "%H" => "dir#{7.chr}name",
      "%y" => "f",
    }.each do |directive, printed|
      context "with the #{directive} directive" do
        subject(:result) { find_with_terminal_output("dir#{7.chr}name", "-type", "f", "-printf", "#{directive}\n") }

        it_behaves_like "a successful search", printed
      end
    end
  end

  context "when -printf %l writes to a terminal" do
    subject(:result) { find_with_terminal_output(".", "-type", "l", "-printf", "%l\n") }

    before { sandbox.symlink("tar#{8.chr}get", "link") }

    it_behaves_like "a successful search", "tar?get"
  end

  context "when -printf writes to a pipe" do
    subject(:result) { find(".", "-type", "l", "-printf", "%l %p\n") }

    before { sandbox.symlink("tar#{8.chr}get", "li#{8.chr}nk") }

    it_behaves_like "a successful search", "tar#{8.chr}get ./li#{8.chr}nk"
  end

  context "with a UTF-8 name on a terminal in the C locale" do
    subject(:result) { find_with_terminal_output(".", "-name", "caf*") }

    before { sandbox.touch("café") }

    it_behaves_like "a successful search", "./caf??"
  end

  context "with a UTF-8 name on a terminal in a UTF-8 locale", locale: "C.UTF-8" do
    subject(:result) { find_with_terminal_output(".", "-name", "caf*", env: { "LC_ALL" => "C.UTF-8" }) }

    before { sandbox.touch("café") }

    it_behaves_like "a successful search", "./café".b
  end

  context "when -ok prompts for a file" do
    subject(:result) { find(".", "-name", "bell*", "-ok", "echo", "{}", ";", stdin: "n\n") }

    before { sandbox.touch("bell#{7.chr}name") }

    it "shows the name as-is" do
      expect(result).to have_attributes(stderr: "< echo ... ./bell#{7.chr}name > ? ")
    end
  end
end
