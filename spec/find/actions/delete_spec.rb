RSpec.describe "find -delete" do
  context "with a path matching an empty directory", upstream: "find.gnu/delete.exp" do
    subject(:deletion) { find("tmp", "-path", "tmp/top/two", "-delete") }

    before { sandbox.mkdir("tmp/top/one/foo", "tmp/top/two") }

    it "removes the directory" do
      expect { deletion }.to change { sandbox.exist?("tmp/top/two") }.from(true).to(false)
    end

    context "when listing the hierarchy afterwards" do
      subject(:result) { find("tmp", "-print") }

      before { deletion }

      it_behaves_like "a successful search", "tmp", "tmp/top", "tmp/top/one", "tmp/top/one/foo"
    end
  end

  context "with a path matching a regular file", upstream: "find.gnu/deletefile.exp" do
    subject(:result) { find("tmp", "-path", "tmp/top/file", "-delete", "-print") }

    before do
      sandbox.mkdir("tmp/top")
      sandbox.touch("tmp/top/file")
    end

    it_behaves_like "a successful search", "tmp/top/file"

    it "removes the file" do
      expect { result }.to change { sandbox.exist?("tmp/top/file") }.from(true).to(false)
    end
  end

  context "with a path matching a directory", upstream: "find.gnu/deletedir.exp" do
    subject(:result) { find("tmp", "-path", "tmp/top/dir", "-delete", "-print") }

    before { sandbox.mkdir("tmp/top/dir") }

    it_behaves_like "a successful search", "tmp/top/dir"
  end

  context "with a whole hierarchy" do
    subject(:result) { find("tree", "-delete", "-print") }

    before do
      sandbox.mkdir("tree/branch")
      sandbox.touch("tree/branch/leaf")
    end

    it "deletes the contents of each directory before the directory itself" do
      expect(result).to print_lines("tree/branch/leaf", "tree/branch", "tree").in_order
    end

    it "removes the starting point" do
      expect { result }.to change { sandbox.exist?("tree") }.from(true).to(false)
    end
  end

  context "with a symbolic link" do
    subject(:result) { find("dir", "-type", "l", "-delete") }

    before do
      sandbox.mkdir("dir")
      sandbox.touch("target")
      sandbox.symlink("../target", "dir/link")
    end

    it "removes the link" do
      expect { result }.to change { sandbox.exist?("dir/link") }.from(true).to(false)
    end

    it "keeps the file the link points to" do
      expect { result }.not_to change { sandbox.exist?("target") }.from(true)
    end
  end

  context "with a directory that is not empty" do
    subject(:result) { find("dir", "-name", "sub", "-delete") }

    before do
      sandbox.mkdir("dir/sub")
      sandbox.touch("dir/sub/file")
    end

    it_behaves_like "a failed search"

    it { is_expected.to diagnose("find: cannot delete 'dir/sub': Directory not empty") }

    it "keeps the directory" do
      expect { result }.not_to change { sandbox.exist?("dir/sub") }.from(true)
    end
  end

  context "when the removal fails" do
    subject(:result) { find("dir", "-name", "sub", "(", "-delete", "-o", "-printf", "kept %p\n", ")") }

    before do
      sandbox.mkdir("dir/sub")
      sandbox.touch("dir/sub/file")
    end

    it "evaluates to false" do
      expect(result).to print_lines("kept dir/sub")
    end
  end

  context "with -prune" do
    subject(:result) { find("dir", "-prune", "-delete") }

    before { sandbox.mkdir("dir") }

    it_behaves_like "a rejected command line",
                    "find: The -delete action automatically turns on -depth, but -prune does nothing when -depth is " \
                    "in effect.  If you want to carry on anyway, just explicitly use the -depth option."

    it "deletes nothing" do
      expect { result }.not_to change { sandbox.exist?("dir") }.from(true)
    end
  end

  context "with -prune and an explicit -depth" do
    subject(:result) { find("dir", "-depth", "-prune", "-delete") }

    before { sandbox.mkdir("dir") }

    it "carries on deleting" do
      expect { result }.to change { sandbox.exist?("dir") }.from(true).to(false)
    end
  end
end
