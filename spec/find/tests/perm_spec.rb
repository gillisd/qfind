RSpec.describe "find -perm" do
  def create_file(path, mode)
    sandbox.touch(path)
    sandbox.chmod(mode, path)
  end

  def change_of_meaning_warning(mode)
    "find: warning: you have specified a mode pattern #{mode} (which is equivalent to /000). The meaning of " \
      "-perm /000 was changed in 2007 to be consistent with -perm -000; that is, while it used to match no " \
      "files, it now matches all files. This warning message will be removed in a future findutils release."
  end

  shared_context "with files of assorted modes" do
    before do
      [0o000, 0o002, 0o020, 0o200, 0o220, 0o444, 0o555, 0o644, 0o664, 0o755, 0o777].each do |mode|
        create_file(format("%03o", mode), mode)
      end
    end
  end

  shared_examples "a slash mode without any bits" do |mode, *lines|
    at_every_optimisation_level do
      it { is_expected.to be_successful }

      it { is_expected.to print_lines(*lines) }
    end

    it { is_expected.to diagnose(change_of_meaning_warning(mode)) }
  end

  context "with -0100 against a searchable directory and a read-only file", upstream: "find.gnu/perm.exp" do
    subject(:result) { find("tmp", "-perm", "-0100") }

    before do
      sandbox.mkdir("tmp")
      sandbox.chmod(0o755, "tmp")
      create_file("tmp/fred", 0o444)
    end

    it_behaves_like "a successful search", "tmp"
  end

  context "with /555 against assorted modes", upstream: "find.gnu/perm-slash.exp" do
    subject(:result) { find("tmp", "-mindepth", "1", "-perm", "/555", "-printf", "p/555 %p\n") }

    before do
      sandbox.mkdir("tmp")
      [0o400, 0o200, 0o555, 0o700, 0o000, 0o050].each { create_file(format("tmp/%03o", it), it) }
    end

    it_behaves_like "a successful search", "p/555 tmp/050", "p/555 tmp/555", "p/555 tmp/400", "p/555 tmp/700"
  end

  context "with an exact mode and a mode prefixed by a minus sign", upstream: "find.posix/perm-vanilla.exp" do
    subject(:result) do
      find("tmp/400", "tmp/555", "tmp/700",
           "(", "-perm", "400", "-exec", "echo", "p400", "{}", ";", "-exec", "false", ";", ")", "-o",
           "(", "-perm", "-400", "-exec", "echo", "p-400", "{}", ";", "-exec", "false", ";", ")")
    end

    before do
      sandbox.mkdir("tmp")
      [0o400, 0o555, 0o700].each { create_file(format("tmp/%03o", it), it) }
    end

    it_behaves_like "a successful search", "p400 tmp/400", "p-400 tmp/400", "p-400 tmp/555", "p-400 tmp/700"
  end

  context "with X in a symbolic mode prefixed by a minus sign", upstream: "find.posix/perm-X.exp" do
    subject(:result) { find("tmp", "(", "!", "-name", "sub", "-o", "-prune", ")", "-perm", "-u+w,a+X", "-print") }

    before do
      sandbox.mkdir("tmp/sub")
      sandbox.chmod(0o755, "tmp")
      create_file("tmp/empty", 0o644)
      create_file("tmp/empty-xxx", 0o711)
      sandbox.chmod(0o300, "tmp/sub")
    end

    it_behaves_like "a successful search", "tmp", "tmp/empty", "tmp/empty-xxx"
  end

  context "with X in an exact symbolic mode" do
    subject(:result) { find(".", "-mindepth", "1", "-perm", "a=rX") }

    before do
      create_file("file-444", 0o444)
      create_file("file-555", 0o555)
      sandbox.mkdir("dir-444", "dir-555")
      sandbox.chmod(0o444, "dir-444")
      sandbox.chmod(0o555, "dir-555")
    end

    it_behaves_like "a successful search", "./file-444", "./dir-555"
  end

  context "with an exact mode" do
    include_context "with files of assorted modes"

    {
      "644" => ["./644"],
      "0644" => ["./644"],
      "000" => ["./000"],
      "u=rw,g=r,o=r" => ["./644"],
      "u=rw,go=r" => ["./644"],
      "ug=rw,o=r" => ["./664"],
      "a=rw,o-w" => ["./664"],
      "a=rwx,go-w" => ["./755"],
      "g=w" => ["./020"],
    }.each do |mode, matches|
      context "with the argument #{mode}" do
        subject(:result) { find(".", "-type", "f", "-perm", mode) }

        it_behaves_like "a successful search", *matches
      end
    end
  end

  context "with a mode prefixed by a minus sign" do
    include_context "with files of assorted modes"

    {
      "-644" => ["./644", "./664", "./755", "./777"],
      "-g=w" => ["./020", "./220", "./664", "./777"],
      "-220" => ["./220", "./664", "./777"],
      "-g+w,u+w" => ["./220", "./664", "./777"],
      "-444" => ["./444", "./555", "./644", "./664", "./755", "./777"],
      "-a+r" => ["./444", "./555", "./644", "./664", "./755", "./777"],
      "-000" => ["./000", "./002", "./020", "./200", "./220", "./444", "./555", "./644", "./664", "./755", "./777"],
    }.each do |mode, matches|
      context "with the argument #{mode}" do
        subject(:result) { find(".", "-type", "f", "-perm", mode) }

        it_behaves_like "a successful search", *matches
      end
    end
  end

  context "with a mode prefixed by a slash" do
    include_context "with files of assorted modes"

    {
      "/222" => ["./002", "./020", "./200", "./220", "./644", "./664", "./755", "./777"],
      "/a+w" => ["./002", "./020", "./200", "./220", "./644", "./664", "./755", "./777"],
      "/220" => ["./020", "./200", "./220", "./644", "./664", "./755", "./777"],
      "/u+w,g+w" => ["./020", "./200", "./220", "./644", "./664", "./755", "./777"],
      "/u=w,g=w" => ["./020", "./200", "./220", "./644", "./664", "./755", "./777"],
      "/111" => ["./555", "./755", "./777"],
      "/a+x" => ["./555", "./755", "./777"],
    }.each do |mode, matches|
      context "with the argument #{mode}" do
        subject(:result) { find(".", "-type", "f", "-perm", mode) }

        it_behaves_like "a successful search", *matches
      end
    end
  end

  context "with a symbolic mode naming no class of users" do
    include_context "with files of assorted modes"

    context "with a minus sign" do
      subject(:result) { find(".", "-type", "f", "-perm", "-+w") }

      it_behaves_like "a successful search", "./777"
    end

    context "with a slash" do
      subject(:result) { find(".", "-type", "f", "-perm", "/+w") }

      it_behaves_like "a successful search", "./002", "./020", "./200", "./220", "./644", "./664", "./755", "./777"
    end
  end

  context "with modes combined to select files readable by all, writable by some and executable by none" do
    include_context "with files of assorted modes"

    context "with octal modes" do
      subject(:result) { find(".", "-type", "f", "-perm", "-444", "-perm", "/222", "!", "-perm", "/111") }

      it_behaves_like "a successful search", "./644", "./664"
    end

    context "with symbolic modes" do
      subject(:result) { find(".", "-type", "f", "-perm", "-a+r", "-perm", "/a+w", "!", "-perm", "/a+x") }

      it_behaves_like "a successful search", "./644", "./664"
    end
  end

  context "with /000", upstream: "find.gnu/perm000.exp" do
    subject(:result) { find("tmp", "-perm", "/000", "-print") }

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/wilma")
      create_file("tmp/fred", 0o000)
    end

    it_behaves_like "a slash mode without any bits", "/000", "tmp", "tmp/fred", "tmp/wilma"
  end

  context "with a symbolic mode setting no bits after a slash" do
    subject(:result) { find("tmp", "-perm", "/a=") }

    before do
      sandbox.mkdir("tmp")
      create_file("tmp/none", 0o000)
    end

    it_behaves_like "a slash mode without any bits", "/a=", "tmp", "tmp/none"
  end

  context "with -000" do
    subject(:result) { find("tmp", "-perm", "-000") }

    before do
      sandbox.mkdir("tmp")
      create_file("tmp/none", 0o000)
    end

    it_behaves_like "a successful search", "tmp", "tmp/none"
  end

  context "with set-user-ID, set-group-ID and sticky bits" do
    before do
      [0o755, 0o4755, 0o2755, 0o6755].each { create_file(format("%04o", it), it) }
      sandbox.mkdir("sticky", "plain")
      sandbox.chmod(0o1777, "sticky")
      sandbox.chmod(0o777, "plain")
    end

    {
      "-4000" => ["./4755", "./6755"],
      "-u+s" => ["./4755", "./6755"],
      "-2000" => ["./2755", "./6755"],
      "-g+s" => ["./2755", "./6755"],
      "/6000" => ["./4755", "./2755", "./6755"],
      "4755" => ["./4755"],
      "u=rwxs,go=rx" => ["./4755"],
      "-1000" => ["./sticky"],
      "-o+t" => ["./sticky"],
      "1777" => ["./sticky"],
    }.each do |mode, matches|
      context "with the argument #{mode}" do
        subject(:result) { find(".", "-mindepth", "1", "-perm", mode) }

        it_behaves_like "a successful search", *matches
      end
    end
  end

  context "with a symbolic link" do
    before do
      create_file("target", 0o644)
      sandbox.symlink("target", "link")
    end

    context "with the mode of the link itself" do
      subject(:result) { find("link", "-perm", "777") }

      it_behaves_like "a successful search", "link"
    end

    context "with the mode of its target and -L" do
      subject(:result) { find("-L", "link", "-perm", "644") }

      it_behaves_like "a successful search", "link"
    end
  end

  context "with a +mode in octal" do
    subject(:result) { find(".", "-perm", "+111") }

    it_behaves_like "a rejected command line", "find: invalid file mode '+111'"
  end

  context "with a +mode in octal and POSIXLY_CORRECT" do
    subject(:result) { find(".", "-perm", "+111", env: { "POSIXLY_CORRECT" => "1" }) }

    it_behaves_like "a rejected command line", "find: invalid file mode '+111'"
  end

  context "with a +mode that is not a valid symbolic mode" do
    subject(:result) { find(".", "-perm", "+a+x") }

    it_behaves_like "a rejected command line", "find: invalid file mode '+a+x'"
  end

  context "with a +mode that is not a valid symbolic mode and POSIXLY_CORRECT",
          upstream: "find.gnu/posix-perminvalid.exp" do
    subject(:result) { find(".", "-perm", "+a+x", env: { "POSIXLY_CORRECT" => "1" }) }

    it_behaves_like "a rejected command line", "find: invalid file mode '+a+x'"
  end

  context "with a +mode that is a valid symbolic mode" do
    before { [0o222, 0o644, 0o666].each { create_file(format("%03o", it), it) } }

    context "without POSIXLY_CORRECT" do
      subject(:result) { find(".", "-type", "f", "-perm", "+w") }

      it_behaves_like "a successful search", "./222"
    end

    context "with POSIXLY_CORRECT" do
      subject(:result) { find(".", "-type", "f", "-perm", "+w", env: { "POSIXLY_CORRECT" => "1" }) }

      it_behaves_like "a successful search", "./222"
    end
  end

  context "with an invalid mode" do
    {
      "a digit that is not octal" => "8",
      "an octal number above 07777" => "10000",
      "an unknown permission letter" => "u+q",
      "an unknown class of users" => "z=w",
      "a trailing comma" => "u=w,",
      "a permission letter without an operator" => "-w",
      "nothing after the minus sign" => "-",
      "nothing after the slash" => "/",
      "an empty argument" => "",
    }.each do |reason, mode|
      context "with #{reason}" do
        subject(:result) { find(".", "-perm", mode) }

        it_behaves_like "a rejected command line", "find: invalid file mode '#{mode}'"
      end
    end
  end

  context "without a mode" do
    subject(:result) { find(".", "-perm") }

    it_behaves_like "a rejected command line", "find: missing argument to `-perm'"
  end
end
