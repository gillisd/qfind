RSpec.describe "find -H, -L and -P" do
  shared_examples "a search following no links" do
    it_behaves_like "a successful search", "l dirlink", "d top", "l top/link"
  end

  shared_examples "a search following links named as starting points" do
    it_behaves_like "a successful search", "d dirlink", "f dirlink/file", "d top", "l top/link"
  end

  shared_examples "a search following every link" do
    it_behaves_like "a successful search", "d dirlink", "f dirlink/file", "d top", "d top/link", "f top/link/file"
  end

  shared_context "with two links pointing at each other" do
    before do
      sandbox.mkdir("tmp")
      sandbox.symlink("a", "tmp/b")
      sandbox.symlink("b", "tmp/a")
    end
  end

  shared_context "with a link to a file in a directory that cannot be searched" do
    before do
      sandbox.mkdir("sealed")
      sandbox.touch("sealed/file", "mid")
      sandbox.symlink("sealed/file", "link")
      sandbox.chmod(0o000, "sealed")
    end
  end

  shared_examples "a search stopped by the inaccessible link target" do
    it_behaves_like "a failed search"

    it { is_expected.to diagnose("find: 'link': Permission denied") }
  end

  context "without -H, -L or -P" do
    context "with links as starting points" do
      subject(:result) { find("tmp/LINK", "tmp/BROKEN", "tmp", "-printf", "%y %d %p\n") }

      include_context "with a link to a file and a broken link"

      it_behaves_like "a successful search",
                      "d 0 tmp", "l 0 tmp/BROKEN", "l 0 tmp/LINK", "l 1 tmp/BROKEN", "l 1 tmp/LINK"
    end

    context "with links to a directory" do
      subject(:result) { find("dirlink", "top", "-printf", "%y %p\n") }

      include_context "with links to a directory at and below the starting points"

      it_behaves_like "a search following no links"
    end

    context "with a starting point reached through a link to its parent directory",
            upstream: "find.gnu/follow-arg-parent-symlink.exp" do
      subject(:result) { find("tmp/link1/dir2", "-type", "f", "-print") }

      before do
        sandbox.mkdir("tmp/dir1/dir2")
        sandbox.touch("tmp/dir1/dir2/foo")
        sandbox.symlink("dir1", "tmp/link1")
      end

      it_behaves_like "a successful search", "tmp/link1/dir2/foo"
    end
  end

  context "with -P" do
    context "with links as starting points", upstream: "find.gnu/posix-dflt.exp" do
      subject(:result) { find("-P", "tmp/LINK", "tmp/BROKEN", "tmp", "-printf", "%y %d %p\n") }

      include_context "with a link to a file and a broken link"

      it_behaves_like "a successful search",
                      "d 0 tmp", "l 0 tmp/BROKEN", "l 0 tmp/LINK", "l 1 tmp/BROKEN", "l 1 tmp/LINK"
    end

    context "with links to a directory" do
      subject(:result) { find("-P", "dirlink", "top", "-printf", "%y %p\n") }

      include_context "with links to a directory at and below the starting points"

      it_behaves_like "a search following no links"
    end

    context "with a link to an ancestor directory" do
      subject(:result) { find("-P", "top") }

      before do
        sandbox.mkdir("top/sub")
        sandbox.symlink("..", "top/sub/up")
      end

      it_behaves_like "a successful search", "top", "top/sub", "top/sub/up"
    end

    context "with a symbolic link loop as starting point" do
      subject(:result) { find("-P", "tmp/a", "-printf", "%y %p\n") }

      include_context "with two links pointing at each other"

      it_behaves_like "a successful search", "l tmp/a"
    end

    context "with a link to an inaccessible file as starting point", :unprivileged do
      subject(:result) { find("-P", "link", "-printf", "%y %p\n") }

      include_context "with a link to a file in a directory that cannot be searched"

      it_behaves_like "a successful search", "l link"
    end

    ["-newer", "-anewer", "-cnewer", "-newermm"].each do |test|
      context "with a link as the reference file of #{test}" do
        subject(:result) { find("-P", "mid", test, "ref") }

        include_context "with a recent link to an old file as timestamp reference"

        it_behaves_like "a successful search"
      end
    end
  end

  context "with -H" do
    context "with links as starting points", upstream: "find.gnu/posix-h.exp" do
      subject(:result) { find("-H", "tmp/LINK", "tmp/BROKEN", "tmp", "-printf", "H1: %y %d %p\n") }

      include_context "with a link to a file and a broken link"

      it_behaves_like "a successful search",
                      "H1: d 0 tmp", "H1: f 0 tmp/LINK", "H1: l 0 tmp/BROKEN", "H1: l 1 tmp/BROKEN", "H1: l 1 tmp/LINK"
    end

    context "with links to a directory" do
      subject(:result) { find("-H", "dirlink", "top", "-printf", "%y %p\n") }

      include_context "with links to a directory at and below the starting points"

      it_behaves_like "a search following links named as starting points"
    end

    context "with a link to an empty directory as starting point", upstream: "find.posix/sv-bug-12181.exp" do
      subject(:result) { find("-H", "link") }

      before do
        sandbox.mkdir("tmp")
        sandbox.symlink("tmp", "link")
      end

      it_behaves_like "a successful search", "link"
    end

    context "with -maxdepth 0 and a link to a directory as starting point" do
      subject(:result) { find("-H", "dirlink", "-maxdepth", "0", "-printf", "%y %p\n") }

      include_context "with links to a directory at and below the starting points"

      it_behaves_like "a successful search", "d dirlink"
    end

    context "with a link to the root directory below the starting point", upstream: "find.posix/sv-bug-25359.exp" do
      subject(:result) { find("-H", "tmp", "-type", "l", "-print") }

      before do
        sandbox.mkdir("tmp")
        sandbox.symlink("/", "tmp/symlink")
      end

      it_behaves_like "a successful search", "tmp/symlink"
    end

    context "with a symbolic link loop as starting point" do
      subject(:result) { find("-H", "tmp/a", "-printf", "%y %p\n") }

      include_context "with two links pointing at each other"

      it_behaves_like "a failed search"

      it { is_expected.to diagnose("find: 'tmp/a': Too many levels of symbolic links") }
    end

    context "with a link to an inaccessible file as starting point", :unprivileged do
      subject(:result) { find("-H", "link", "-printf", "%y %p\n") }

      include_context "with a link to a file in a directory that cannot be searched"

      it_behaves_like "a search stopped by the inaccessible link target"
    end

    context "with a link to an inaccessible file as the reference file of -newer", :unprivileged do
      subject(:result) { find("-H", "mid", "-newer", "link") }

      include_context "with a link to a file in a directory that cannot be searched"

      it_behaves_like "a search stopped by the inaccessible link target"
    end

    ["-newer", "-anewer", "-cnewer", "-newermm"].each do |test|
      context "with a link as the reference file of #{test}" do
        subject(:result) { find("-H", "mid", test, "ref") }

        include_context "with a recent link to an old file as timestamp reference"

        it_behaves_like "a successful search", "mid"
      end
    end
  end

  context "with -L" do
    context "with links as starting points", upstream: "find.gnu/posix-l.exp" do
      subject(:result) { find("-L", "tmp/LINK", "tmp/BROKEN", "tmp", "-printf", "%y %d %p\n") }

      include_context "with a link to a file and a broken link"

      it_behaves_like "a successful search",
                      "d 0 tmp", "f 0 tmp/LINK", "f 1 tmp/LINK", "l 0 tmp/BROKEN", "l 1 tmp/BROKEN"
    end

    context "with links to a directory" do
      subject(:result) { find("-L", "dirlink", "top", "-printf", "%y %p\n") }

      include_context "with links to a directory at and below the starting points"

      it_behaves_like "a search following every link"
    end

    context "with a symbolic link loop and a false expression", upstream: "find.posix/sv-bug-19605.exp" do
      subject(:result) { find("-L", "tmp", "-false", "-print") }

      include_context "with two links pointing at each other"

      it_behaves_like "a failed search"

      it { is_expected.to diagnose("find: 'tmp/b': Too many levels of symbolic links") }
    end

    context "with a symbolic link loop in a subdirectory", upstream: "find.posix/sv-bug-19613.exp" do
      subject(:result) { find("-L", "tmp", "-depth", "-type", "f") }

      before do
        sandbox.mkdir("tmp/subdir")
        sandbox.symlink("a", "tmp/subdir/b")
        sandbox.symlink("b", "tmp/subdir/a")
        sandbox.touch("tmp/vanilla")
      end

      it_behaves_like "a failed search", "tmp/vanilla"

      it { is_expected.to diagnose("find: 'tmp/subdir/a': Too many levels of symbolic links") }
    end

    context "with a link to an inaccessible file as starting point", :unprivileged do
      subject(:result) { find("-L", "link", "-printf", "%y %p\n") }

      include_context "with a link to a file in a directory that cannot be searched"

      it_behaves_like "a search stopped by the inaccessible link target"
    end

    context "with a link to an inaccessible file as the reference file of -newer", :unprivileged do
      subject(:result) { find("-L", "mid", "-newer", "link") }

      include_context "with a link to a file in a directory that cannot be searched"

      it_behaves_like "a search stopped by the inaccessible link target"
    end

    it_behaves_like "a search dereferencing symbolic links", ["-L"], []
  end

  ["-P", "-H", "-L"].each do |option|
    context "with #{option} and a starting point that does not exist", upstream: "find.posix/sv-bug-19617.exp" do
      subject(:result) { find(option, "tmp/nosuchfile", "-print") }

      before { sandbox.mkdir("tmp") }

      it_behaves_like "a failed search"

      it { is_expected.to diagnose("find: 'tmp/nosuchfile': No such file or directory") }
    end

    context "with #{option} and a broken link as the reference file of -newer" do
      subject(:result) { find(option, "future", "-newer", "broken") }

      before do
        sandbox.touch("future")
        sandbox.utime("future", atime: Time.utc(2100), mtime: Time.utc(2100))
        sandbox.symlink("nowhere", "broken")
      end

      it_behaves_like "a successful search", "future"
    end
  end

  {
    ["-L", "-P"] => "a search following no links",
    ["-H", "-P"] => "a search following no links",
    ["-L", "-H"] => "a search following links named as starting points",
    ["-P", "-H"] => "a search following links named as starting points",
    ["-H", "-L"] => "a search following every link",
    ["-P", "-L"] => "a search following every link",
  }.each do |options, behaviour|
    context "with #{options.join(" then ")} and links to a directory" do
      subject(:result) { find(*options, "dirlink", "top", "-printf", "%y %p\n") }

      include_context "with links to a directory at and below the starting points"

      it_behaves_like behaviour
    end
  end

  ["-H", "-L", "-P"].each do |option|
    context "with #{option} after a starting point" do
      subject(:result) { find(".", option) }

      it_behaves_like "a rejected command line", "find: unknown predicate `#{option}'"
    end
  end
end
