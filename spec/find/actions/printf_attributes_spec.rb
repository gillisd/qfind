RSpec.describe "find -printf attribute directives" do
  def status_of(path) = File.lstat(sandbox.join(path))

  def number_for(directive, path)
    status = status_of(path)
    {
      "b" => status.blocks,
      "D" => status.dev,
      "G" => status.gid,
      "i" => status.ino,
      "k" => (status.blocks * 512).ceildiv(1024),
      "n" => status.nlink,
      "s" => status.size,
      "U" => status.uid,
    }.fetch(directive).to_s
  end

  def sparseness_of(path) = status_of(path).then { it.blocks * 512.0 / it.size }

  context "with every kind of directive on a file last modified at noon on 22 June 1990",
          upstream: "find.gnu/printf.exp" do
    subject(:result) { find("tmp", "-type", "f", *formats.flat_map { ["-printf", it] }) }

    let(:formats) do
      [
        "a %d\n", "b %f\n", "c %h\n", "d %H\n", "e %l\n", "f %m\n", "g %n\n", "h %p\n", "i %P\n", "j %y\n",
        "k %Y\n", "percent it%%works%%\n", "string-right !%10p!\n", "string-left !%-10p!\n",
        "trunc-R !%10.6p!\n", "trunc-L !%-10.6p!\n", "a5 !%5d!\n", "a05 !%05d!\n", "a+ %+d\n",
        "l1 %T+ %T@ TH=%TH TI=%TI TM=%TM TT=%TT Td=%Td TD=%TD Tj=%Tj Tm=%Tm TU=%TU Tw=%Tw TW=%TW " \
        "Ty=%Ty TY=%TY\n",
        "l2 M=%M\n", "l3 t=%t\n", "l4 y=%y Y=%Y\n"
      ]
    end

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/file")
      sandbox.chmod(0o600, "tmp/file")
      sandbox.utime("tmp/file", atime: Time.utc(1990, 6, 22, 12), mtime: Time.utc(1990, 6, 22, 12))
    end

    it_behaves_like "a successful search",
                    "a 1", "a+ +1", "a5 !    1!", "a05 !00001!", "b file", "c tmp", "d tmp", "e ", "f 600", "g 1",
                    "h tmp/file", "i file", "j f", "k f",
                    "l1 1990-06-22+12:00:00.0000000000 646056000.0000000000 TH=12 TI=12 TM=00 " \
                    "TT=12:00:00.0000000000 Td=22 TD=06/22/90 Tj=173 Tm=06 TU=24 Tw=5 TW=25 Ty=90 TY=1990",
                    "l2 M=-rw-------", "l3 t=Fri Jun 22 12:00:00.0000000000 1990", "l4 y=f Y=f",
                    "percent it%works%", "string-left !tmp/file  !", "string-right !  tmp/file!",
                    "trunc-L !tmp/fi    !", "trunc-R !    tmp/fi!"
  end

  context "with the inode number before the name", upstream: "tests/find/printf_inode.sh" do
    subject(:result) { find("file", "-printf", "%i_%p\n") }

    before { sandbox.touch("file") }

    it "prints the inode number the file system reports" do
      expect(result).to print_exactly("#{status_of("file").ino}_file\n")
    end
  end

  context "with a file of 12345 bytes and two more hard links to it" do
    before do
      sandbox.write("file", "x" * 12_345)
      sandbox.link("file", "second")
      sandbox.link("file", "third")
    end

    {
      "b" => "the disk space used in 512-byte blocks",
      "D" => "the number of the device holding the file",
      "G" => "the numeric group id",
      "i" => "the inode number",
      "k" => "the disk space used in 1 KiB blocks",
      "n" => "the number of hard links",
      "s" => "the size in bytes",
      "U" => "the numeric user id",
    }.each do |directive, meaning|
      context "with %#{directive}" do
        subject(:result) { find("file", "-printf", "%#{directive}") }

        it "prints #{meaning}" do
          expect(result).to print_exactly(number_for(directive, "file"))
        end
      end

      context "with the #, + and 0 flags and a width on %#{directive}" do
        subject(:result) { find("file", "-printf", "%#+010#{directive}") }

        it "ignores the flags and pads #{meaning} with spaces" do
          expect(result).to print_exactly(number_for(directive, "file").rjust(10))
        end
      end
    end

    context "with a precision on %s" do
      subject(:result) { find("file", "-printf", "%.2s") }

      it "truncates the size like a string" do
        expect(result).to print_exactly("12")
      end
    end

    context "with %S" do
      subject(:result) { find("file", "-printf", "%S") }

      it "prints the disk space used divided by the size" do
        expect(result).to print_exactly(format("%g", sparseness_of("file")))
      end
    end

    context "with a precision on %S" do
      subject(:result) { find("file", "-printf", "%.2S") }

      it "limits the number of significant digits" do
        expect(result).to print_exactly(format("%.2g", sparseness_of("file")))
      end
    end
  end

  context "with a sparse file of one mebibyte" do
    before do
      sandbox.touch("sparse")
      sandbox.truncate("sparse", 1_048_576)
    end

    context "with %s" do
      subject(:result) { find("sparse", "-printf", "%s") }

      it "prints the apparent size" do
        expect(result).to print_exactly("1048576")
      end
    end

    context "with %b" do
      subject(:result) { find("sparse", "-printf", "%b") }

      it "prints the disk space actually allocated" do
        expect(result).to print_exactly(number_for("b", "sparse"))
      end
    end

    context "with %S" do
      subject(:result) { find("sparse", "-printf", "%S") }

      it "prints a sparseness below 1" do
        expect(result).to print_exactly(format("%g", sparseness_of("sparse")))
      end
    end
  end

  context "with an empty file" do
    before { sandbox.touch("empty") }

    context "with %S" do
      subject(:result) { find("empty", "-printf", "%S") }

      it "prints a sparseness of 1" do
        expect(result).to print_exactly("1")
      end
    end

    {
      "%#S" => ["the # flag", "1.00000"],
      "%+S" => ["the + flag", "+1"],
      "% S" => ["the space flag", " 1"],
      "%05S" => ["the 0 flag and a width", "00001"],
      "%-5S" => ["the - flag and a width", "1    "],
    }.each do |directive, (flags, output)|
      context "with #{flags} on %S" do
        subject(:result) { find("empty", "-printf", directive) }

        it { is_expected.to print_exactly(output) }
      end
    end
  end

  context "with a symbolic link to a file of 12345 bytes" do
    before do
      sandbox.write("file", "x" * 12_345)
      sandbox.symlink("file", "link")
    end

    context "without -L" do
      subject(:result) { find("link", "-printf", "%s") }

      it "prints the size of the link itself" do
        expect(result).to print_exactly(status_of("link").size.to_s)
      end
    end

    context "with -L" do
      subject(:result) { find("-L", "link", "-printf", "%s") }

      it "prints the size of the file it points to" do
        expect(result).to print_exactly("12345")
      end
    end
  end

  context "with a file owned by the current user" do
    before { sandbox.touch("file") }

    context "with %u" do
      subject(:result) { find("file", "-printf", "%u") }

      it "prints the name of the owner" do
        expect(result).to print_exactly(Etc.getpwuid(status_of("file").uid).name)
      end
    end

    context "with %g" do
      subject(:result) { find("file", "-printf", "%g") }

      it "prints the name of the group" do
        expect(result).to print_exactly(Etc.getgrgid(status_of("file").gid).name)
      end
    end

    context "with the - flag and a width on %u" do
      subject(:result) { find("file", "-printf", "%-20u") }

      it "left-aligns the name of the owner" do
        expect(result).to print_exactly(Etc.getpwuid(status_of("file").uid).name.ljust(20))
      end
    end

    context "with a precision on %g" do
      subject(:result) { find("file", "-printf", "%.1g") }

      it "truncates the name of the group" do
        expect(result).to print_exactly(Etc.getgrgid(status_of("file").gid).name[0])
      end
    end
  end

  context "with a file owned by a user id unknown to the system", :root do
    subject(:result) { find("orphan", "-printf", "%u") }

    include_context "with a file owned by a user id unknown to the system"

    it "prints the numeric user id instead of a name" do
      expect(result).to print_exactly("4000000000")
    end
  end

  context "with a file owned by a group id unknown to the system", :root do
    subject(:result) { find("orphan", "-printf", "%g") }

    include_context "with a file owned by a group id unknown to the system"

    it "prints the numeric group id instead of a name" do
      expect(result).to print_exactly("4000000000")
    end
  end

  context "with %F on files in two file systems" do
    subject(:result) { find("file", "/proc", "-maxdepth", "0", "-printf", "%F\n") }

    let(:types) { [sandbox.join("file"), "/proc"].map { MountTable.file_system_type(it) } }

    before { sandbox.touch("file") }

    it "prints the type of the file system holding each file" do
      expect(result).to print_lines(*types).in_order
    end
  end

  context "with %m and %M on files of each type" do
    subject(:result) { find("file", "dir", "fifo", "sock", File::NULL, "-printf", "%m %M %p\n") }

    before do
      sandbox.mkdir("dir")
      sandbox.touch("file")
      sandbox.mkfifo("fifo")
      sandbox.socket("sock")
      sandbox.chmod(0o640, "file")
      sandbox.chmod(0o750, "dir")
      sandbox.chmod(0o600, "fifo")
      sandbox.chmod(0o755, "sock")
    end

    it_behaves_like "a successful search",
                    "640 -rw-r----- file", "750 drwxr-x--- dir", "600 prw------- fifo", "755 srwxr-xr-x sock",
                    "666 crw-rw-rw- /dev/null"
  end

  context "with %m and %M on a symbolic link" do
    subject(:result) { find("link", "-printf", "%m %M") }

    before { sandbox.symlink("nowhere", "link") }

    it "describes the link itself" do
      expect(result).to print_exactly("777 lrwxrwxrwx")
    end
  end

  context "with %m and %M on files with the set-user-ID, set-group-ID and sticky bits" do
    subject(:result) { find("executable", "plain", "-printf", "%m %M %p\n") }

    before do
      sandbox.touch("executable", "plain")
      sandbox.chmod(0o7755, "executable")
      sandbox.chmod(0o7644, "plain")
    end

    it_behaves_like "a successful search", "7755 -rwsr-sr-t executable", "7644 -rwSr-Sr-T plain"
  end

  context "with a file with mode 640" do
    before do
      sandbox.touch("file")
      sandbox.chmod(0o640, "file")
    end

    {
      "%#m" => ["the # flag on %m", "0640"],
      "%06m" => ["the 0 flag and a width on %m", "000640"],
      "%+m" => ["the + flag on %m", "640"],
      "% m" => ["the space flag on %m", "640"],
      "%-6m" => ["the - flag and a width on %m", "640   "],
      "%.4m" => ["a precision on %m", "0640"],
      "%12M" => ["a width on %M", "  -rw-r-----"],
      "%-12M" => ["the - flag and a width on %M", "-rw-r-----  "],
      "%.4M" => ["a precision on %M", "-rw-"],
      "%012M" => ["the 0 flag and a width on %M", "  -rw-r-----"],
    }.each do |directive, (flags, output)|
      context "with #{flags}" do
        subject(:result) { find("file", "-printf", directive) }

        it { is_expected.to print_exactly(output) }
      end
    end
  end

  context "with a file one level below the starting point" do
    before do
      sandbox.mkdir("dir")
      sandbox.touch("dir/file")
    end

    {
      "%d" => ["no flags on %d", "1"],
      "%+d" => ["the + flag on %d", "+1"],
      "% d" => ["the space flag on %d", " 1"],
      "%#d" => ["the # flag on %d", "1"],
      "%.3d" => ["a precision on %d", "001"],
      "%-5d" => ["the - flag and a width on %d", "1    "],
      "%+05d" => ["the + and 0 flags and a width on %d", "+0001"],
      "%-05d" => ["the - flag before the 0 flag on %d", "1    "],
    }.each do |directive, (flags, output)|
      context "with #{flags}" do
        subject(:result) { find("dir", "-mindepth", "1", "-printf", directive) }

        it { is_expected.to print_exactly(output) }
      end
    end

    context "with the 0 flag before the - flag on %d" do
      subject(:result) { find("dir", "-mindepth", "1", "-printf", "%0-5d") }

      it_behaves_like "a successful search that warns", "find: warning: unrecognized format directive `%-'", "%0-5d"
    end
  end

  context "with %d on a hierarchy" do
    subject(:result) { find("dir", "-printf", "%d %p\n") }

    before do
      sandbox.mkdir("dir/sub")
      sandbox.touch("dir/sub/file")
    end

    it_behaves_like "a successful search", "0 dir", "1 dir/sub", "2 dir/sub/file"
  end

  context "with %y on a directory of every file type" do
    subject(:result) { find("dir", File::NULL, "-printf", "%y %p\n") }

    include_context "with a directory of every file type a user can create"

    it_behaves_like "a successful search",
                    "d dir", "d dir/dir", "f dir/reg", "p dir/fifo", "s dir/sock", "c /dev/null", "l dir/reg-link",
                    "l dir/dir-link", "l dir/fifo-link", "l dir/sock-link", "l dir/dangling-link"
  end

  context "with %Y on a directory of every file type" do
    subject(:result) { find("dir", File::NULL, "-printf", "%Y %p\n") }

    include_context "with a directory of every file type a user can create"

    it_behaves_like "a successful search",
                    "d dir", "d dir/dir", "f dir/reg", "p dir/fifo", "s dir/sock", "c /dev/null", "f dir/reg-link",
                    "d dir/dir-link", "p dir/fifo-link", "s dir/sock-link", "N dir/dangling-link"
  end

  context "with %y and %Y on a directory of every file type and -L" do
    subject(:result) { find("-L", "dir", "-printf", "%y %Y %p\n") }

    include_context "with a directory of every file type a user can create"

    it_behaves_like "a successful search",
                    "d d dir", "d d dir/dir", "f f dir/reg", "p p dir/fifo", "s s dir/sock", "f f dir/reg-link",
                    "d d dir/dir-link", "p p dir/fifo-link", "s s dir/sock-link", "l N dir/dangling-link"
  end

  context "with %y and %Y on devices" do
    subject(:result) { find("dir/blk", "dir/chr", "dir/blk-link", "dir/chr-link", "-printf", "%y %Y %p\n") }

    include_context "with a directory of every file type including devices"

    it_behaves_like "a successful search", "b b dir/blk", "c c dir/chr", "l b dir/blk-link", "l c dir/chr-link"
  end

  context "with %y and %Y on links to a file, to nothing, through a file and to themselves",
          upstream: "find.gnu/printf-symlink.exp" do
    subject(:result) do
      find("tmp/LINK", "tmp/DANGLE", "tmp/ENOTDIR", "tmp/SELF",
           "-printf", "RESULT: %y %Y %p\n", "-printf", "RESULT2: %Y %y %p\n")
    end

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/file")
      sandbox.symlink("file", "tmp/LINK")
      sandbox.symlink("ENOENT", "tmp/DANGLE")
      sandbox.symlink("file/ENOTDIR", "tmp/ENOTDIR")
      sandbox.symlink("SELF", "tmp/SELF")
    end

    it_behaves_like "a successful search",
                    "RESULT: l f tmp/LINK", "RESULT2: f l tmp/LINK", "RESULT: l N tmp/DANGLE",
                    "RESULT2: N l tmp/DANGLE", "RESULT: l N tmp/ENOTDIR", "RESULT2: N l tmp/ENOTDIR",
                    "RESULT: l L tmp/SELF", "RESULT2: L l tmp/SELF"
  end

  context "with %Y on a relative link in a subdirectory", upstream: "find.gnu/printf-nonlocal-symlink.exp" do
    subject(:result) { find("tmp", "-type", "l", "-printf", "%p: %Y %y\n") }

    before do
      sandbox.mkdir("tmp/foo")
      sandbox.touch("tmp/foo/file")
      sandbox.symlink("file", "tmp/foo/LINK")
    end

    it_behaves_like "a successful search", "tmp/foo/LINK: f l"
  end

  context "with %Y on a link into a directory that cannot be searched", :unprivileged do
    subject(:result) { find("link", "-printf", "%y %Y\n") }

    before do
      sandbox.mkdir("locked")
      sandbox.touch("locked/file")
      sandbox.symlink("locked/file", "link")
      sandbox.chmod(0o000, "locked")
    end

    it { is_expected.to be_successful }
    it { is_expected.to print_exactly("l ?\n") }
    it { is_expected.to diagnose("find: 'link': Permission denied") }
  end

  context "with %Z on a file without a security context" do
    subject(:result) { find("file", "-printf", "[%Z]\n") }

    before do
      skip "SELinux labels every file here" if File.exist?("/sys/fs/selinux/enforce")
      sandbox.touch("file")
    end

    it { is_expected.to have_exit_status(1) }
    it { is_expected.to print_exactly("[]\n") }
    it { is_expected.to diagnose(a_string_starting_with("find: getfilecon failed: 'file': ")) }
  end
end
