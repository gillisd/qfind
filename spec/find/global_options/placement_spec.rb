RSpec.describe "find global options placed after other arguments" do
  shared_examples "a search warning that a global option is misplaced" do |option, argument, *lines|
    it_behaves_like "a successful search that warns",
                    "find: warning: you have specified the global option #{option} after the argument #{argument}, " \
                    "but global options are not positional, i.e., #{option} affects tests specified before it as " \
                    "well as those specified after it.  Please specify global options before other arguments.",
                    *lines
  end

  before { sandbox.mkdir("dir") }

  [
    ["-d"],
    ["-depth"],
    ["-ignore_readdir_race"],
    ["-maxdepth", "1"],
    ["-mindepth", "0"],
    ["-mount"],
    ["-noignore_readdir_race"],
    ["-noleaf"],
    ["-xdev"],
  ].each do |option, *arguments|
    context "with #{[option, *arguments].join(" ")} after a test and -warn" do
      subject(:result) { find("dir", "-warn", "-name", "dir", option, *arguments) }

      it_behaves_like "a search warning that a global option is misplaced", option, "-name", "dir"
    end
  end

  context "with -files0-from after a test and -warn" do
    subject(:result) { find("-warn", "-name", "dir", "-files0-from", "list") }

    before { sandbox.write("list", "dir#{0.chr}") }

    it_behaves_like "a search warning that a global option is misplaced", "-files0-from", "-name", "dir"
  end

  ["-help", "--help", "-version", "--version"].each do |option|
    context "with #{option} after a test, -warn and standard output discarded" do
      subject(:result) { find_with_stdout_to(File::NULL, "dir", "-warn", "-name", "dir", option) }

      it_behaves_like "a search warning that a global option is misplaced", option, "-name"
    end
  end

  context "with a global option after an action and -warn" do
    subject(:result) { find("dir", "-warn", "-print", "-depth") }

    it_behaves_like "a search warning that a global option is misplaced", "-depth", "-print", "dir"
  end

  context "with a global option after a negation and -warn" do
    subject(:result) { find("dir", "-warn", "!", "-depth") }

    it_behaves_like "a search warning that a global option is misplaced", "-depth", "!"
  end

  context "with a global option inside parentheses and -warn" do
    subject(:result) { find("dir", "-warn", "(", "-depth", ")") }

    it_behaves_like "a search warning that a global option is misplaced", "-depth", "(", "dir"
  end

  context "with a global option after several tests and actions and -warn" do
    subject(:result) { find("dir", "-warn", "-type", "d", "-name", "dir", "-print", "-depth") }

    it_behaves_like "a search warning that a global option is misplaced", "-depth", "-type", "dir"
  end

  context "with two global options after a test and -warn" do
    subject(:result) { find("dir", "-warn", "-name", "dir", "-depth", "-noleaf") }

    it_behaves_like "a search warning that a global option is misplaced", "-depth", "-name", "dir"

    it_behaves_like "a search warning that a global option is misplaced", "-noleaf", "-name", "dir"
  end

  context "with a global option after a test when standard input is a terminal" do
    subject(:result) { find_on_terminal("dir", "-name", "dir", "-maxdepth", "0") }

    it_behaves_like "a search warning that a global option is misplaced", "-maxdepth", "-name", "dir"
  end

  context "with a global option after a test when standard input is not a terminal" do
    subject(:result) { find("dir", "-name", "dir", "-maxdepth", "0") }

    it_behaves_like "a successful search", "dir"
  end

  context "with -nowarn and a global option after a test when standard input is a terminal" do
    subject(:result) { find_on_terminal("dir", "-nowarn", "-name", "dir", "-maxdepth", "0") }

    it_behaves_like "a successful search", "dir"
  end

  context "with -warn only after the misplaced global option" do
    subject(:result) { find("dir", "-name", "dir", "-maxdepth", "0", "-warn") }

    it_behaves_like "a successful search", "dir"
  end

  context "with -warn, POSIXLY_CORRECT set and a global option after a test" do
    subject(:result) { find("dir", "-warn", "-name", "dir", "-maxdepth", "0", env: { "POSIXLY_CORRECT" => "1" }) }

    it_behaves_like "a successful search", "dir"
  end

  context "with -warn and global options before every test" do
    subject(:result) { find("dir", "-warn", "-depth", "-noleaf", "-maxdepth", "1", "-name", "dir") }

    it_behaves_like "a successful search", "dir"
  end

  ["-daystart", "-follow", "-regextype posix-basic"].each do |positional|
    context "with -warn and a global option after #{positional}" do
      subject(:result) { find("dir", "-warn", *positional.split, "-depth") }

      it_behaves_like "a successful search", "dir"
    end
  end

  context "with -maxdepth after a test that matches only below it" do
    subject(:result) { find("dir", "-name", "file", "-maxdepth", "1") }

    before do
      sandbox.mkdir("dir/sub")
      sandbox.touch("dir/file", "dir/sub/file")
    end

    it_behaves_like "a successful search", "dir/file"
  end

  context "with -depth after the action it reorders" do
    subject(:result) { find("dir", "-print", "-depth") }

    before { sandbox.touch("dir/file") }

    it "applies to the action before it" do
      expect(result).to print_lines("dir/file", "dir").in_order
    end
  end
end
