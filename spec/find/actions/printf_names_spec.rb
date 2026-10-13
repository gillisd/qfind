RSpec.describe "find -printf file name directives" do
  shared_context "with a file two levels below dir" do
    before do
      sandbox.mkdir("dir/sub")
      sandbox.touch("dir/sub/file")
    end
  end

  context "with %p" do
    include_context "with a file two levels below dir"

    context "with files below a starting point" do
      subject(:result) { find("dir", "-printf", "%p\n") }

      it_behaves_like "a successful search", "dir", "dir/sub", "dir/sub/file"
    end

    context "with a starting point ending in slashes" do
      subject(:result) { find("dir//", "-printf", "%p\n") }

      it_behaves_like "a successful search", "dir//", "dir//sub", "dir//sub/file"
    end
  end

  context "with %P" do
    include_context "with a file two levels below dir"

    context "with files below a starting point" do
      subject(:result) { find("dir", "-printf", "[%P]\n") }

      it_behaves_like "a successful search", "[]", "[sub]", "[sub/file]"
    end

    context "with a starting point ending in slashes" do
      subject(:result) { find("dir//", "-printf", "[%P]\n") }

      it_behaves_like "a successful search", "[]", "[sub]", "[sub/file]"
    end

    context "with the current directory as starting point" do
      subject(:result) { find(".", "-printf", "[%P]\n") }

      it_behaves_like "a successful search", "[]", "[dir]", "[dir/sub]", "[dir/sub/file]"
    end

    context "with an absolute starting point" do
      subject(:result) { find(sandbox.join("dir").to_s, "-printf", "[%P]\n") }

      it_behaves_like "a successful search", "[]", "[sub]", "[sub/file]"
    end
  end

  context "with %f" do
    context "with files below a starting point" do
      subject(:result) { find("dir", "-printf", "%f\n") }

      include_context "with a file two levels below dir"

      it_behaves_like "a successful search", "dir", "sub", "file"
    end

    context "with the root directory spelt with one and with three slashes", upstream: "find.gnu/printf-slash.exp" do
      subject(:result) { find("/", "///", "-maxdepth", "0", "-printf", "%p %f\n") }

      it_behaves_like "a successful search", "/ /", "/// /"
    end
  end

  context "with %h" do
    context "with files below a starting point" do
      subject(:result) { find("dir", "-printf", "%h\n") }

      include_context "with a file two levels below dir"

      it_behaves_like "a successful search", ".", "dir", "dir/sub"
    end

    context "with files below a starting point ending in slashes" do
      subject(:result) { find("dir//", "-mindepth", "1", "-printf", "%h\n") }

      include_context "with a file two levels below dir"

      it_behaves_like "a successful search", "dir/", "dir//sub"
    end

    context "with the corner cases shown in the manual" do
      subject(:result) do
        find(".", "..", "/", "/dev", File::NULL, "compile", "compile/64/tests/find",
             "-maxdepth", "0", "-printf", "[%h][%f]\n")
      end

      before { sandbox.mkdir("compile/64/tests/find") }

      it_behaves_like "a successful search",
                      "[.][.]", "[.][..]", "[][/]", "[][dev]", "[/dev][null]", "[.][compile]",
                      "[compile/64/tests][find]"
    end
  end

  context "with %H" do
    context "without a starting point", upstream: "find.gnu/printfHdfl.exp" do
      subject(:result) { find("-maxdepth", "0", "-printf", "%H") }

      it_behaves_like "a successful search printing exactly", "."
    end

    context "with a second starting point longer than the first", upstream: "find.gnu/sv-bug-18222.exp" do
      subject(:result) { find("tmp/short", "tmp/longer", "-printf", "%H\n") }

      before do
        sandbox.mkdir("tmp")
        sandbox.touch("tmp/short", "tmp/longer")
      end

      it_behaves_like "a successful search", "tmp/short", "tmp/longer"
    end

    context "with files below a starting point ending in slashes" do
      subject(:result) { find("dir//", "-printf", "%H\n") }

      include_context "with a file two levels below dir"

      it_behaves_like "a successful search", "dir//", "dir//", "dir//"
    end

    context "with files below several starting points" do
      subject(:result) { find("a", "b/c", "-mindepth", "1", "-printf", "%H:%P\n") }

      before { sandbox.mkdir("a/x", "b/c/y") }

      it_behaves_like "a successful search", "a:x", "b/c:y"
    end
  end

  context "with %l" do
    before do
      sandbox.mkdir("dir")
      sandbox.touch("dir/file")
      sandbox.symlink("file", "dir/link")
      sandbox.symlink("/NOSUCHFILE", "dir/dangling")
    end

    context "with symbolic links" do
      subject(:result) { find("dir", "-type", "l", "-printf", "%p -> %l\n") }

      it_behaves_like "a successful search", "dir/link -> file", "dir/dangling -> /NOSUCHFILE"
    end

    context "with files that are not symbolic links" do
      subject(:result) { find("dir", "!", "-type", "l", "-printf", "[%l]\n") }

      it_behaves_like "a successful search", "[]", "[]"
    end

    context "with a field width and a file that is not a symbolic link" do
      subject(:result) { find("dir/file", "-printf", "[%5l]") }

      it "pads the empty target to the width" do
        expect(result).to print_exactly("[     ]")
      end
    end

    context "with -L" do
      subject(:result) { find("-L", "dir", "-printf", "%p [%l]\n") }

      it_behaves_like "a successful search", "dir []", "dir/file []", "dir/link []", "dir/dangling [/NOSUCHFILE]"
    end

    context "with -H and a symbolic link given both as starting point and below one" do
      subject(:result) { find("-H", "dir/link", "dir", "-name", "link", "-printf", "%p [%l]\n") }

      it_behaves_like "a successful search", "dir/link []", "dir/link [file]"
    end
  end

  context "with a field width or a precision" do
    before { sandbox.touch("abc") }

    {
      "%6f" => ["right-aligns the name in a field of that width", "[   abc]"],
      "%-6f" => ["left-aligns the name with the - flag", "[abc   ]"],
      "%2f" => ["keeps a name wider than the field whole", "[abc]"],
      "%.2f" => ["truncates the name to the precision", "[ab]"],
      "%-6.2f" => ["truncates the name, then aligns it", "[ab    ]"],
    }.each do |directive, (behaviour, output)|
      context "with #{directive}" do
        subject(:result) { find("abc", "-printf", "[#{directive}]") }

        it behaviour do
          expect(result).to print_exactly(output)
        end
      end
    end
  end

  {
    "the root directory" => {
      "/" => ["", "/"],
      "//" => ["/", "/"],
      "///" => ["//", "/"],
      "///." => ["//", "."],
      "//." => ["/", "."],
    },
    "the current directory" => {
      "." => [".", "."],
      "./" => [".", "./"],
      ".//" => ["./", "./"],
      ".///" => [".//", "./"],
      ".////" => [".///", "./"],
      "./////" => [".////", "./"],
      "./////." => [".////", "."],
      ".////." => [".///", "."],
      ".///." => [".//", "."],
      ".//." => ["./", "."],
      ".//./" => ["./", "./"],
      ".//.//" => ["./", "./"],
      ".//.//." => [".//./", "."],
      ".//./." => [".//.", "."],
      "./." => [".", "."],
      "././" => [".", "./"],
      "././/" => [".", "./"],
      "././/." => ["././", "."],
      "././." => ["./.", "."],
    },
    "a directory" => {
      ".//foo" => ["./", "foo"],
      ".//foo/" => ["./", "foo/"],
      ".//foo//" => ["./", "foo/"],
      ".//foo//." => [".//foo/", "."],
      ".//foo/." => [".//foo", "."],
      "./foo" => [".", "foo"],
      "./foo/" => [".", "foo/"],
      "./foo//" => [".", "foo/"],
      "./foo//." => ["./foo/", "."],
      "./foo/." => ["./foo", "."],
      "foo" => [".", "foo"],
      "foo/" => [".", "foo/"],
      "foo//" => [".", "foo/"],
      "foo//." => ["foo/", "."],
      "foo/." => ["foo", "."],
    },
    "a subdirectory" => {
      ".//foo/bar" => [".//foo", "bar"],
      ".//foo/bar/" => [".//foo", "bar/"],
      ".//foo/bar//" => [".//foo", "bar/"],
      ".//foo/bar//." => [".//foo/bar/", "."],
      ".//foo/bar/." => [".//foo/bar", "."],
      "./foo/bar" => ["./foo", "bar"],
      "./foo/bar/" => ["./foo", "bar/"],
      "./foo/bar//" => ["./foo", "bar/"],
      "./foo/bar//." => ["./foo/bar/", "."],
      "./foo/bar/." => ["./foo/bar", "."],
      "foo/bar" => ["foo", "bar"],
      "foo/bar/" => ["foo", "bar/"],
      "foo/bar//" => ["foo", "bar/"],
      "foo/bar//." => ["foo/bar/", "."],
      "foo/bar/." => ["foo/bar", "."],
    },
    "a symbolic link to a directory" => {
      ".//foolink" => ["./", "foolink"],
      ".//foolink/" => ["./", "foolink/"],
      ".//foolink//" => ["./", "foolink/"],
      ".//foolink//." => [".//foolink/", "."],
      ".//foolink/." => [".//foolink", "."],
      "./foolink" => [".", "foolink"],
      "./foolink/" => [".", "foolink/"],
      "./foolink//" => [".", "foolink/"],
      "./foolink//." => ["./foolink/", "."],
      "./foolink/." => ["./foolink", "."],
      "foolink" => [".", "foolink"],
      "foolink/" => [".", "foolink/"],
      "foolink//" => [".", "foolink/"],
      "foolink//." => ["foolink/", "."],
      "foolink/." => ["foolink", "."],
    },
  }.each do |kind, spellings|
    context "with %p, %h and %f on spellings of #{kind}", upstream: "find.gnu/printf-h.exp" do
      before do
        sandbox.mkdir("foo/bar")
        sandbox.symlink("foo", "foolink")
      end

      spellings.each do |starting_point, (leading_directories, basename)|
        context "with the starting point #{starting_point}" do
          subject(:result) { find(starting_point, "-maxdepth", "0", "-printf", "%p,%h,%f\n") }

          it_behaves_like "a successful search", "#{starting_point},#{leading_directories},#{basename}"
        end
      end
    end
  end
end
