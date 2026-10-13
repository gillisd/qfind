RSpec.describe "find -ok" do
  let(:without_leak_check) { { "GNU_FINDUTILS_FD_LEAK_CHECK" => nil } }

  shared_examples "an accepted answer" do
    it "runs the command" do
      expect(result).to print_exactly("ran file\n")
    end
  end

  shared_examples "a refused answer" do
    it "does not run the command" do
      expect(result).to print_nothing
    end
  end

  it_behaves_like "an action running a command from the starting directory", "-ok" do
    let(:find_options) { { stdin: "y\n", env: without_leak_check } }
  end

  context "with a file below a directory" do
    subject(:result) { find("dir/file", "-ok", "echo", "ran", "{}", ";", stdin: "n\n") }

    before do
      sandbox.mkdir("dir")
      sandbox.touch("dir/file")
    end

    it "asks on standard error, naming only the command and the file" do
      expect(result).to have_attributes(stderr: "< echo ... dir/file > ? ")
    end
  end

  context "with the braces as the command" do
    subject(:result) { find("./script", "-ok", "{}", ";", stdin: "n\n") }

    before { sandbox.touch("script") }

    it "names the command as written" do
      expect(result).to have_attributes(stderr: "< {} ... ./script > ? ")
    end
  end

  context "with several files" do
    subject(:result) do
      find("a", "b", "c", "-ok", "echo", "ran", "{}", ";", stdin: "y\nn\ny\n", env: without_leak_check)
    end

    before { sandbox.touch("a", "b", "c") }

    it "asks about each file in turn" do
      expect(result).to have_attributes(stderr: "< echo ... a > ? < echo ... b > ? < echo ... c > ? ")
    end

    it "reads one answer per question" do
      expect(result).to print_lines("ran a", "ran c").in_order
    end
  end

  context "when asked about a file" do
    subject(:result) do
      find("file", "-ok", "echo", "ran", "{}", ";", stdin: answer, env: without_leak_check.merge(environment))
    end

    let(:environment) { {} }

    before { sandbox.touch("file") }

    {
      "y\n" => "with a lowercase y as the answer",
      "Y\n" => "with an uppercase Y as the answer",
      "yes\n" => "with a word starting with y as the answer",
      "y" => "with a y and no newline before the end of input",
    }.each do |reply, description|
      context description do
        let(:answer) { reply }

        it_behaves_like "an accepted answer"
      end
    end

    {
      "n\n" => "with n as the answer",
      "\n" => "with an empty line as the answer",
      "" => "without any answer before the end of input",
      "x\n" => "with another letter as the answer",
      " y\n" => "with a y after a blank as the answer",
      "1\n" => "with 1 as the answer",
    }.each do |reply, description|
      context description do
        let(:answer) { reply }

        it_behaves_like "a refused answer"
      end
    end

    context "with y as the answer and POSIXLY_CORRECT set" do
      let(:answer) { "y\n" }
      let(:environment) { { "POSIXLY_CORRECT" => "1" } }

      it_behaves_like "an accepted answer"
    end

    context "with 1 as the answer and POSIXLY_CORRECT set" do
      let(:answer) { "1\n" }
      let(:environment) { { "POSIXLY_CORRECT" => "1" } }

      it_behaves_like "a refused answer"
    end

    context "with 1 as the answer and LC_MESSAGES set to en_US.UTF-8", locale: "en_US.UTF-8" do
      let(:answer) { "1\n" }
      let(:environment) { { "LC_ALL" => nil, "LC_MESSAGES" => "en_US.UTF-8" } }

      it_behaves_like "an accepted answer"
    end

    context "with 1 as the answer, LC_MESSAGES set to en_US.UTF-8 and POSIXLY_CORRECT set", locale: "en_US.UTF-8" do
      let(:answer) { "1\n" }
      let(:environment) { { "LC_ALL" => nil, "LC_MESSAGES" => "en_US.UTF-8", "POSIXLY_CORRECT" => "1" } }

      it_behaves_like "an accepted answer"
    end

    context "with 1 as the answer and only LC_CTYPE set to en_US.UTF-8", locale: "en_US.UTF-8" do
      let(:answer) { "1\n" }
      let(:environment) { { "LC_ALL" => nil, "LC_CTYPE" => "en_US.UTF-8" } }

      it_behaves_like "a refused answer"
    end
  end

  context "with -files0-from reading the starting points from a file" do
    subject(:result) do
      find("-files0-from", "list", "-ok", "echo", "ran", "{}", ";", stdin: "y\n", env: without_leak_check)
    end

    before do
      sandbox.touch("file")
      sandbox.write("list", "file#{0.chr}")
    end

    it_behaves_like "an accepted answer"
  end

  context "with a refused answer and -o" do
    subject(:result) { find("file", "-ok", "echo", "ran", "{}", ";", "-o", "-print", stdin: "n\n") }

    before { sandbox.touch("file") }

    it "evaluates to false" do
      expect(result).to print_exactly("file\n")
    end
  end

  context "with a command inspecting its standard input" do
    subject(:result) { find("file", "-ok", "./stdin-source", ";", stdin: "y\n", env: without_leak_check) }

    before do
      sandbox.touch("file")
      sandbox.write("stdin-source", "#!/bin/sh\nreadlink /proc/self/fd/0 || echo closed\n")
      sandbox.chmod(0o755, "stdin-source")
    end

    it "runs it with its standard input redirected from /dev/null" do
      pending "findutils 4.11.0 closes the standard input of the command instead"
      expect(result).to print_exactly("/dev/null\n")
    end
  end

  context "with an accepted answer and the descriptor leak check enabled" do
    subject(:result) { find("file", "-ok", "echo", "ran", "{}", ";", stdin: "y\n") }

    before { sandbox.touch("file") }

    it "runs the command without leaking a descriptor into it" do
      pending "findutils 4.11.0 leaves /dev/null open on a high descriptor after closing standard input"
      expect(result).to print_exactly("ran file\n")
    end
  end

  context "with a plus sign after the braces" do
    subject(:result) { find("file", "-ok", "echo", "{}", "+") }

    it_behaves_like "a rejected command line", "find: missing argument to `-ok'"
  end

  context "with a plus sign after the braces and a semicolon" do
    subject(:result) { find("file", "-ok", "echo", "{}", "+", ";", stdin: "y\n", env: without_leak_check) }

    before { sandbox.touch("file") }

    it "passes the plus sign on to the command" do
      expect(result).to print_exactly("file +\n")
    end
  end

  context "when the question cannot be written" do
    subject(:result) do
      find_with_stdin_from("answers", "file", "-ok", "touch", "ran", ";", err: "/dev/full", env: without_leak_check)
    end

    before do
      sandbox.touch("file")
      sandbox.write("answers", "y\n")
    end

    it { is_expected.to have_exit_status(1) }

    it "gives up without running the command" do
      expect { result }.not_to change { sandbox.exist?("ran") }.from(false)
    end
  end
end
