RSpec.describe "find -depth and -d" do
  shared_context "with empty and non-empty subdirectories" do
    before do
      sandbox.mkdir("tmp/top/one", "tmp/top/two")
      sandbox.touch("tmp/top/one/foo")
    end
  end

  shared_context "with a chain of nested directories" do
    before do
      sandbox.mkdir("tree/branch")
      sandbox.touch("tree/branch/leaf")
    end
  end

  shared_examples "a post-order traversal" do
    at_every_optimisation_level do
      it "lists the contents of each directory before the directory itself" do
        expect(result).to print_lines("tree/branch/leaf", "tree/branch", "tree").in_order
      end
    end
  end

  shared_examples "a search warning that -d is deprecated" do
    it_behaves_like "a successful search that warns",
                    "find: warning: the -d option is deprecated; please use -depth instead, because the latter is a " \
                    "POSIX-compliant feature.",
                    "tree/branch/leaf", "tree/branch", "tree"
  end

  context "with -depth selecting the empty directories below the starting point", upstream: "find.gnu/depth.exp" do
    subject(:result) { find("tmp/top", "-depth", "-mindepth", "1", "-type", "d", "-empty", "-print") }

    include_context "with empty and non-empty subdirectories"

    it_behaves_like "a successful search", "tmp/top/two"
  end

  context "with -d selecting the empty directories below the starting point", upstream: "find.gnu/depth-d.exp" do
    subject(:result) { find("tmp/top", "-d", "-mindepth", "1", "-type", "d", "-empty", "-print") }

    include_context "with empty and non-empty subdirectories"

    it_behaves_like "a successful search", "tmp/top/two"
  end

  context "with -depth selecting the directories of a tree full of files", upstream: "find.posix/depth1.exp" do
    subject(:result) { find("tmp/1", "-depth", "-type", "d", "-print") }

    before do
      sandbox.mkdir("tmp/1/2/3", "tmp/1/2/20", "tmp/1/2/37", "tmp/1/2/54", *(56..59).map { "tmp/1/2/55/#{it}" })
      { "tmp/1/2/3" => 4..19, "tmp/1/2/20" => 21..36, "tmp/1/2/37" => 38..53, "tmp/1/2" => 60..65, "tmp/1" => 66..81 }
        .each { |directory, names| sandbox.touch(*names.map { "#{directory}/#{it}" }) }
    end

    it_behaves_like "a successful search",
                    "tmp/1", "tmp/1/2", "tmp/1/2/20", "tmp/1/2/3", "tmp/1/2/37", "tmp/1/2/54", "tmp/1/2/55",
                    "tmp/1/2/55/56", "tmp/1/2/55/57", "tmp/1/2/55/58", "tmp/1/2/55/59"
  end

  context "with -depth on a chain of nested directories" do
    subject(:result) { find("tree", "-depth") }

    include_context "with a chain of nested directories"

    it_behaves_like "a post-order traversal"
  end

  context "with -d on a chain of nested directories" do
    subject(:result) { find("tree", "-d") }

    include_context "with a chain of nested directories"

    it_behaves_like "a post-order traversal"
  end

  context "without -depth on a chain of nested directories" do
    subject(:result) { find("tree") }

    include_context "with a chain of nested directories"

    it "lists each directory before its contents" do
      expect(result).to print_lines("tree", "tree/branch", "tree/branch/leaf").in_order
    end
  end

  context "with -delete elsewhere on the command line" do
    subject(:result) { find("tree", "-false", "-delete", "-o", "-print") }

    include_context "with a chain of nested directories"

    it_behaves_like "a post-order traversal"
  end

  context "with -depth and -quit" do
    subject(:result) { find("tree", "-depth", "-print", "-quit") }

    include_context "with a chain of nested directories"

    it_behaves_like "a successful search", "tree/branch/leaf"
  end

  context "with -depth and -maxdepth" do
    subject(:result) { find("tree", "-depth", "-maxdepth", "1") }

    include_context "with a chain of nested directories"

    it "lists the starting point after the entries within the depth limit" do
      expect(result).to print_lines("tree/branch", "tree").in_order
    end
  end

  context "with -depth and a starting point that is not a directory" do
    subject(:result) { find("file", "-depth") }

    before { sandbox.touch("file") }

    it_behaves_like "a successful search", "file"
  end

  context "with -depth followed by a stray argument" do
    subject(:result) { find("tree", "-depth", "1") }

    include_context "with a chain of nested directories"

    it_behaves_like "a rejected command line", "find: paths must precede expression: `1'"
  end

  context "with -depth and a directory that cannot be read", :unprivileged,
          upstream: "tests/find/depth-unreadable-dir.sh" do
    subject(:result) { find("tmp", "-depth", "-name", "dir") }

    before do
      sandbox.mkdir("tmp/dir")
      sandbox.chmod(0o311, "tmp/dir")
    end

    it_behaves_like "a failed search", "tmp/dir"

    it "diagnoses only the directory it cannot read" do
      expect(result).to diagnose_exactly("find: 'tmp/dir': Permission denied")
    end
  end

  context "with -d and -warn" do
    subject(:result) { find("tree", "-warn", "-d") }

    include_context "with a chain of nested directories"

    it_behaves_like "a search warning that -d is deprecated"
  end

  context "with -d when standard input is a terminal" do
    subject(:result) { find_on_terminal("tree", "-d") }

    include_context "with a chain of nested directories"

    it_behaves_like "a search warning that -d is deprecated"
  end

  context "with -d and -nowarn when standard input is a terminal" do
    subject(:result) { find_on_terminal("tree", "-nowarn", "-d") }

    include_context "with a chain of nested directories"

    it_behaves_like "a successful search", "tree/branch/leaf", "tree/branch", "tree"
  end

  context "with -d and POSIXLY_CORRECT set when standard input is a terminal" do
    subject(:result) { find_on_terminal("tree", "-d", env: { "POSIXLY_CORRECT" => "1" }) }

    include_context "with a chain of nested directories"

    it_behaves_like "a successful search", "tree/branch/leaf", "tree/branch", "tree"
  end

  context "with -d when standard input is not a terminal" do
    subject(:result) { find("tree", "-d") }

    include_context "with a chain of nested directories"

    it_behaves_like "a successful search", "tree/branch/leaf", "tree/branch", "tree"
  end

  context "with -depth when standard input is a terminal" do
    subject(:result) { find_on_terminal("tree", "-depth") }

    include_context "with a chain of nested directories"

    it_behaves_like "a successful search", "tree/branch/leaf", "tree/branch", "tree"
  end
end
