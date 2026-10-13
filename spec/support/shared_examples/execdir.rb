RSpec.shared_context "with a directory-reporting command on a secure PATH" do
  let(:secure_path) { "#{sandbox.join("bin")}:/usr/bin:/bin" }

  before do
    sandbox.mkdir("bin")
    sandbox.write("bin/report", <<~'SH')
      #!/bin/sh
      cwd=$(pwd -P)
      echo ".${cwd#"${0%/bin/report}"}:" $(printf '%s\n' "$@" | sort)
    SH
    sandbox.chmod(0o755, "bin/report")
  end
end

RSpec.shared_examples "an action running a command from the directory of each file" do |action, confirmations = ""|
  context "with a file in a subdirectory" do
    subject(:result) { find("tmp", "-name", "fred", action, "report", "{}", ";", env:, stdin: confirmations) }

    before { sandbox.mkdir("tmp/fred", "tmp/jim") }

    it "runs the command from that subdirectory on the file name prefixed with ./" do
      expect(result).to print_lines("./tmp: ./fred")
    end
  end

  context "with two files in one directory" do
    subject(:result) { find("tmp", "-type", "f", action, "report", "{}", ";", env:, stdin: confirmations) }

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/one", "tmp/two")
    end

    it "runs the command once for each file" do
      expect(result).to print_lines("./tmp: ./one", "./tmp: ./two")
    end
  end

  context "with a starting point inside another directory" do
    subject(:result) { find("tmp/top", "-maxdepth", "0", action, "report", "{}", ";", env:, stdin: confirmations) }

    before { sandbox.mkdir("tmp/top") }

    it "runs the command from the directory containing the starting point" do
      expect(result).to print_lines("./tmp: ./top")
    end
  end

  context "with an absolute starting point" do
    subject(:result) do
      find(sandbox.join("tmp/top").to_s, "-maxdepth", "0", action, "report", "{}", ";", env:, stdin: confirmations)
    end

    before { sandbox.mkdir("tmp/top") }

    it "passes only the base name prefixed with ./" do
      expect(result).to print_lines("./tmp: ./top")
    end
  end

  context "with a starting point carrying a trailing slash" do
    subject(:result) { find("tmp/top/", "-maxdepth", "0", action, "report", "{}", ";", env:, stdin: confirmations) }

    before { sandbox.mkdir("tmp/top") }

    it "keeps the trailing slash in the name" do
      expect(result).to print_lines("./tmp: ./top/")
    end
  end

  context "with . as the starting point" do
    subject(:result) { find(".", "-maxdepth", "0", action, "report", "{}", ";", env:, stdin: confirmations) }

    it "runs the command from the starting point itself on ./." do
      expect(result).to print_lines(".: ./.")
    end
  end

  context "with the root directory as the starting point" do
    subject(:result) { find("/", "-maxdepth", "0", action, "echo", "{}", ";", env:, stdin: confirmations) }

    it "passes / without the ./ prefix" do
      expect(result).to print_lines("/")
    end
  end

  context "with the root directory as the starting point and pwd as the command" do
    subject(:result) { find("/", "-maxdepth", "0", action, "pwd", ";", env:, stdin: confirmations) }

    it "runs the command from the root directory" do
      expect(result).to print_lines("/")
    end
  end

  context "with -L and a file below a link to a directory" do
    subject(:result) { find("-L", "tmp", "-name", "file", action, "report", "{}", ";", env:, stdin: confirmations) }

    before do
      sandbox.mkdir("tmp", "other")
      sandbox.touch("other/file")
      sandbox.symlink("../other", "tmp/link")
    end

    it "runs the command from the directory the link resolves to" do
      expect(result).to print_lines("./other: ./file")
    end
  end

  context "with a file name that looks like an option" do
    subject(:result) { find("tmp", "-name", "-n", action, "echo", "{}", ";", env:, stdin: confirmations) }

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/-n")
    end

    it "keeps the command from taking the name for an option" do
      expect(result).to print_lines("./-n")
    end
  end

  context "with the braces inside larger arguments" do
    subject(:result) do
      find("tmp", "-name", "fred", action, "echo", "pre{}post", "{}-{}", "{}", ";", env:, stdin: confirmations)
    end

    before { sandbox.mkdir("tmp/fred") }

    it "replaces every occurrence with the file name prefixed with ./" do
      expect(result).to print_lines("pre./fredpost ./fred-./fred ./fred")
    end
  end

  context "with a semicolon inside a larger argument" do
    subject(:result) { find("tmp", "-name", "fred", action, "echo", "a;", "{}", ";", env:, stdin: confirmations) }

    before { sandbox.mkdir("tmp/fred") }

    it "passes it on as an ordinary argument" do
      expect(result).to print_lines("a; ./fred")
    end
  end

  context "without any other action" do
    subject(:result) { find("tmp", action, "true", ";", env:, stdin: confirmations) }

    before { sandbox.mkdir("tmp") }

    it "does not print the file" do
      expect(result).to print_nothing
    end
  end

  context "with a command that succeeds" do
    subject(:result) { find("tmp", "-name", "fred", action, "true", ";", "-print", env:, stdin: confirmations) }

    before { sandbox.mkdir("tmp/fred") }

    it "evaluates to true" do
      expect(result).to print_lines("tmp/fred")
    end
  end

  context "with a command that fails" do
    subject(:result) do
      find("tmp", "-name", "fred", "(", action, "false", ";", "-o", "-print", ")", env:, stdin: confirmations)
    end

    before { sandbox.mkdir("tmp/fred") }

    it "evaluates to false" do
      expect(result).to print_lines("tmp/fred")
    end

    it { is_expected.to be_successful }
  end

  context "with a command that does not exist" do
    subject(:result) do
      find("tmp", "-name", "fred", "(", action, "no-such-command", ";", "-o", "-print", ")",
           env:, stdin: confirmations)
    end

    before { sandbox.mkdir("tmp/fred") }

    it { is_expected.to diagnose(a_string_ending_with("find: 'no-such-command': No such file or directory")) }

    it "evaluates to false" do
      expect(result).to print_lines("tmp/fred")
    end

    it { is_expected.to be_successful }
  end

  context "with a command killed by a signal" do
    subject(:result) do
      find("tmp", "-name", "fred", "(", action, "sh", "-c", "kill -9 $$", ";", "-o", "-print", ")",
           env:, stdin: confirmations)
    end

    before { sandbox.mkdir("tmp/fred") }

    it { is_expected.to diagnose(a_string_ending_with("find: 'sh' terminated by signal 9")) }

    it "evaluates to false" do
      expect(result).to print_lines("tmp/fred")
    end

    it { is_expected.to be_successful }
  end

  context "without PATH set" do
    subject(:result) do
      find("tmp", "-name", "fred", action, "echo", "{}", ";", env: env.merge("PATH" => nil), stdin: confirmations)
    end

    before { sandbox.mkdir("tmp/fred") }

    it "finds the command on the default search path" do
      expect(result).to print_lines("./fred")
    end
  end

  context "with a file in a directory that can be searched but not read", :unprivileged do
    subject(:result) { find("locked/file", "free", action, "echo", "{}", ";", env:, stdin: confirmations) }

    before do
      sandbox.mkdir("locked", "free")
      sandbox.touch("locked/file")
      sandbox.chmod(0o100, "locked")
    end

    it { is_expected.to have_exit_status(1) }

    it "diagnoses the directory it cannot open" do
      expect(result).to diagnose(a_string_ending_with("find: Failed to save working directory in order to run a " \
                                                      "command on 'locked/file': Permission denied"))
    end

    it "gives up before the remaining starting points" do
      expect(result).to print_nothing
    end
  end

  context "when no more processes can be created", :unprivileged do
    subject(:result) do
      find("tmp/fred", "tmp/jim", "-print", action, "true", ";", env:, stdin: confirmations, rlimit_nproc: 0)
    end

    before { sandbox.mkdir("tmp/fred", "tmp/jim") }

    it { is_expected.to have_exit_status(1) }

    it { is_expected.to diagnose(a_string_ending_with("find: cannot fork: Resource temporarily unavailable")) }

    it "gives up at the first file" do
      expect(result).to print_lines("tmp/fred")
    end
  end

  context "without any arguments" do
    subject(:result) { find("tmp", action, env:) }

    it_behaves_like "a rejected command line", "find: missing argument to `#{action}'"
  end

  context "without a command before the semicolon" do
    subject(:result) { find("tmp", action, ";", env:) }

    it_behaves_like "a rejected command line", "find: invalid argument `;' to `#{action}'"
  end

  context "without a terminating semicolon" do
    subject(:result) { find("tmp", action, "echo", "{}", env:) }

    it_behaves_like "a rejected command line", "find: missing argument to `#{action}'"
  end

  {
    "{}" => "as the command",
    "./{}" => "inside the command name",
  }.each do |command, placement|
    context "with the braces #{placement}" do
      subject(:result) { find("tmp", action, command, ";", env:) }

      it_behaves_like "a rejected command line",
                      "find: You may not use {} within the utility name for -execdir and -okdir, because this is a " \
                      "potential security problem."
    end
  end
end

RSpec.shared_examples "an action refusing an insecure PATH" do |action|
  {
    "." => "a lone dot",
    "/usr/bin:." => "a dot entry",
    "/usr/bin::/bin" => "an empty entry between two colons",
    ":/usr/bin" => "a leading colon",
    "/usr/bin:" => "a trailing colon",
    "" => "nothing",
  }.each do |path, description|
    context "with #{description} in PATH" do
      subject(:result) { find("tmp", action, "true", ";", env: env.merge("PATH" => path)) }

      it_behaves_like "a rejected command line",
                      "find: The current directory is included in the PATH environment variable, which is insecure " \
                      "in combination with the #{action} action of find.  Please remove the current directory from " \
                      "your $PATH (that is, remove \".\", doubled colons, or leading or trailing colons)"
    end
  end

  {
    "bin:/usr/bin" => "bin",
    "/usr/bin:./bin" => "./bin",
    "/usr/bin:../bin" => "../bin",
    "/usr/bin:~/bin" => "~/bin",
  }.each do |path, entry|
    context "with the relative entry #{entry} in PATH" do
      subject(:result) { find("tmp", action, "true", ";", env: env.merge("PATH" => path)) }

      it_behaves_like "a rejected command line",
                      "find: The relative path '#{entry}' is included in the PATH environment variable, which is " \
                      "insecure in combination with the #{action} action of find.  Please remove that entry from $PATH"
    end
  end

  context "with an insecure PATH and the action in a branch that never runs" do
    subject(:result) { find("tmp", "-false", action, "true", ";", env: env.merge("PATH" => "bin")) }

    it_behaves_like "a rejected command line",
                    "find: The relative path 'bin' is included in the PATH environment variable, which is insecure " \
                    "in combination with the #{action} action of find.  Please remove that entry from $PATH"
  end
end
