RSpec.describe "find -noleaf" do
  shared_context "with directories holding both subdirectories and files" do
    before do
      sandbox.mkdir("top/one/sub", "top/two")
      sandbox.touch("top/file", "top/one/file", "top/one/sub/file", "top/two/file")
    end
  end

  context "with directories holding both subdirectories and files" do
    subject(:result) { find("top", "-noleaf") }

    include_context "with directories holding both subdirectories and files"

    it_behaves_like "a successful search",
                    "top", "top/file", "top/one", "top/one/file", "top/one/sub", "top/one/sub/file", "top/two",
                    "top/two/file"
  end

  context "with a test of the file type" do
    subject(:result) { find("top", "-noleaf", "-type", "d") }

    include_context "with directories holding both subdirectories and files"

    it_behaves_like "a successful search", "top", "top/one", "top/one/sub", "top/two"
  end

  context "with -depth" do
    subject(:result) { find("top", "-noleaf", "-depth", "-path", "top/one/sub*") }

    include_context "with directories holding both subdirectories and files"

    it "still lists the contents of each directory first" do
      expect(result).to print_lines("top/one/sub/file", "top/one/sub").in_order
    end
  end

  context "with symbolic links followed" do
    subject(:result) { find("-L", "top", "-noleaf", "-name", "file") }

    include_context "with directories holding both subdirectories and files"

    before { sandbox.symlink("one", "top/link") }

    it_behaves_like "a successful search",
                    "top/file", "top/one/file", "top/one/sub/file", "top/two/file", "top/link/file",
                    "top/link/sub/file"
  end

  context "when given twice" do
    subject(:result) { find("top", "-noleaf", "-noleaf", "-name", "file") }

    include_context "with directories holding both subdirectories and files"

    it_behaves_like "a successful search", "top/file", "top/one/file", "top/one/sub/file", "top/two/file"
  end

  context "with a stray argument after it" do
    subject(:result) { find("top", "-noleaf", "x") }

    it_behaves_like "a rejected command line", "find: paths must precede expression: `x'"
  end
end
