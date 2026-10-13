RSpec.shared_examples "an action running a command from the starting directory" do |action|
  let(:find_options) { {} }

  before { sandbox.touch("file") }

  context "without a command before the semicolon" do
    subject(:result) { find("file", action, ";") }

    it_behaves_like "a rejected command line", "find: invalid argument `;' to `#{action}'"
  end

  context "without a terminating semicolon" do
    subject(:result) { find("file", action, "echo", "{}") }

    it_behaves_like "a rejected command line", "find: missing argument to `#{action}'"
  end

  context "without any arguments" do
    subject(:result) { find("file", action) }

    it_behaves_like "a rejected command line", "find: missing argument to `#{action}'"
  end

  context "with the braces inside larger arguments" do
    subject(:result) { find("file", action, "echo", "pre{}post", "{}-{}", "{}", ";", **find_options) }

    it "replaces every occurrence with the file name" do
      expect(result).to print_exactly("prefilepost file-file file\n")
    end
  end

  context "with the braces as the command" do
    subject(:result) { find("./script", action, "{}", "argument", ";", **find_options) }

    before do
      sandbox.write("script", "#!/bin/sh\necho \"script run with $*\"\n")
      sandbox.chmod(0o755, "script")
    end

    it "runs the file itself" do
      expect(result).to print_exactly("script run with argument\n")
    end
  end

  context "with a semicolon inside a larger argument" do
    subject(:result) { find("file", action, "echo", "a;", "{}", ";", **find_options) }

    it "passes it on as an ordinary argument" do
      expect(result).to print_exactly("a; file\n")
    end
  end

  context "with arguments a shell would expand" do
    subject(:result) { find("file", action, "echo", "*", "$HOME", "{}", ";", **find_options) }

    it "passes them on verbatim" do
      expect(result).to print_exactly("* $HOME file\n")
    end
  end

  context "with a file below the starting point" do
    subject(:result) { find("dir", "-name", "deep", action, "./where", ";", **find_options) }

    before do
      sandbox.mkdir("dir/sub")
      sandbox.touch("dir/sub/deep")
      sandbox.write("where", "#!/bin/sh\npwd -P\n")
      sandbox.chmod(0o755, "where")
    end

    it "runs the command in the directory find was started from" do
      expect(result).to print_lines(sandbox.root.to_s)
    end
  end

  context "without any other action" do
    subject(:result) { find("file", action, "true", ";", **find_options) }

    it "does not print the file" do
      expect(result).to print_nothing
    end
  end

  context "with a command that succeeds" do
    subject(:result) { find("file", action, "true", ";", "-print", **find_options) }

    it "evaluates to true" do
      expect(result).to print_lines("file")
    end
  end

  context "with a command that fails" do
    subject(:result) { find("file", action, "false", ";", "-o", "-print", **find_options) }

    it "evaluates to false" do
      expect(result).to print_lines("file")
    end

    it { is_expected.to be_successful }
  end

  context "with a command that does not exist" do
    subject(:result) { find("file", action, "no-such-command", ";", "-o", "-print", **find_options) }

    it { is_expected.to diagnose(a_string_ending_with("find: 'no-such-command': No such file or directory")) }

    it "evaluates to false" do
      expect(result).to print_lines("file")
    end

    it { is_expected.to be_successful }
  end

  context "with a command killed by a signal" do
    subject(:result) { find("file", action, "sh", "-c", "kill -TERM $$", ";", "-o", "-print", **find_options) }

    it { is_expected.to diagnose(a_string_ending_with("find: 'sh' terminated by signal 15")) }

    it "evaluates to false" do
      expect(result).to print_lines("file")
    end

    it { is_expected.to be_successful }
  end

  context "when no more processes can be created", :unprivileged do
    subject(:result) { find("file", "other", "-print", action, "true", ";", **find_options, rlimit_nproc: 0) }

    before { sandbox.touch("other") }

    it { is_expected.to have_exit_status(1) }

    it { is_expected.to diagnose(a_string_ending_with("find: cannot fork: Resource temporarily unavailable")) }

    it "gives up at the first file" do
      expect(result).to print_lines("file")
    end
  end
end
