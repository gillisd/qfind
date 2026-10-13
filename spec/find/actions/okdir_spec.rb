RSpec.describe "find -okdir" do
  include_context "with a directory-reporting command on a secure PATH"

  let(:env) { { "PATH" => secure_path, "GNU_FINDUTILS_FD_LEAK_CHECK" => nil } }

  it_behaves_like "an action running a command from the directory of each file", "-okdir", "y\n" * 5
  it_behaves_like "an action refusing an insecure PATH", "-okdir"

  context "with a file to ask about" do
    subject(:result) { find("tmp", "-name", "fred", "-okdir", "echo", "{}", ";", env:, stdin: answers) }

    let(:answers) { "n\n" }

    before { sandbox.mkdir("tmp/fred") }

    it "prompts on standard error with the utility name and the path of the file" do
      expect(result).to have_attributes(stderr: "< echo ... tmp/fred > ? ")
    end

    ["y", "Y", "yes", "yes please"].each do |answer|
      context "with the answer #{answer.inspect}" do
        let(:answers) { "#{answer}\n" }

        it "runs the command" do
          expect(result).to print_lines("./fred")
        end
      end
    end

    ["n", "N", "no", "x", " y", ""].each do |answer|
      context "with the answer #{answer.inspect}" do
        let(:answers) { "#{answer}\n" }

        it "does not run the command" do
          expect(result).to print_nothing
        end
      end
    end

    context "without an answer before the end of standard input" do
      let(:answers) { "" }

      it "does not run the command" do
        expect(result).to print_nothing
      end
    end

    context "with POSIXLY_CORRECT set" do
      let(:env) { super().merge("POSIXLY_CORRECT" => "1") }
      let(:answers) { "y\n" }

      it "still takes y for an affirmative answer" do
        expect(result).to print_lines("./fred")
      end
    end

    context "with arguments around the braces" do
      subject(:result) do
        find("tmp", "-name", "fred", "-okdir", "echo", "before", "{}", "after", ";", env:, stdin: answers)
      end

      it "names only the utility in the prompt" do
        expect(result).to have_attributes(stderr: "< echo ... tmp/fred > ? ")
      end
    end

    context "with the file read from a -files0-from list other than standard input" do
      subject(:result) { find("-files0-from", "list", "-okdir", "echo", "{}", ";", env:, stdin: answers) }

      let(:answers) { "y\n" }

      before { sandbox.write("list", "tmp/fred#{0.chr}") }

      it "runs the confirmed command" do
        expect(result).to print_lines("./fred")
      end
    end

    context "with a declined command" do
      subject(:result) do
        find("tmp", "-name", "fred", "(", "-okdir", "echo", "{}", ";", "-o", "-printf", "declined %p\n", ")",
             env:, stdin: answers)
      end

      it "evaluates to false" do
        expect(result).to print_lines("declined tmp/fred")
      end

      it { is_expected.to be_successful }
    end

    context "with + after the braces and a terminating semicolon" do
      subject(:result) { find("tmp", "-name", "fred", "-okdir", "echo", "{}", "+", ";", env:, stdin: answers) }

      let(:answers) { "y\n" }

      it "passes + to the command as an ordinary argument" do
        expect(result).to print_lines("./fred +")
      end
    end

    context "with a confirmed command reading standard input" do
      subject(:result) { find("tmp", "-name", "fred", "-okdir", "cat", ";", env:, stdin: answers) }

      let(:answers) { "y\nmore input\n" }

      it "does not hand the rest of the input to the command" do
        expect(result).to print_nothing
      end

      it "runs the command with its standard input redirected from /dev/null" do
        pending "findutils 4.11.0 closes the standard input of the command instead"
        expect(result).to have_attributes(stderr: "< cat ... tmp/fred > ? ")
      end
    end

    context "with the descriptor leak check enabled" do
      let(:env) { super().merge("GNU_FINDUTILS_FD_LEAK_CHECK" => "1") }
      let(:answers) { "y\n" }

      it "runs the confirmed command without leaking a descriptor into it" do
        pending "findutils 4.11.0 leaves /dev/null open on a high descriptor after closing standard input"
        expect(result).to print_lines("./fred")
      end
    end
  end

  context "with a file name containing an escape character" do
    subject(:result) { find("tmp", "-name", "a*", "-okdir", "echo", "{}", ";", env:, stdin: "n\n") }

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/a#{27.chr}b")
    end

    it "prompts with the name as-is" do
      expect(result).to have_attributes(stderr: "< echo ... tmp/a#{27.chr}b > ? ")
    end
  end

  context "with several files" do
    subject(:result) { find("a", "b", "-okdir", "echo", "{}", ";", env:, stdin: "n\ny\n") }

    before { sandbox.mkdir("a", "b") }

    it "asks once per file, reading one answer each time" do
      expect(result).to print_lines("./b")
    end

    it "writes the prompts one after the other without line breaks" do
      expect(result).to have_attributes(stderr: "< echo ... a > ? < echo ... b > ? ")
    end
  end

  context "with + after the braces" do
    subject(:result) { find("tmp", "-okdir", "echo", "{}", "+", env:) }

    it_behaves_like "a rejected command line", "find: missing argument to `-okdir'"
  end
end
