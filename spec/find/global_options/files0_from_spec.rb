RSpec.describe "find -files0-from", upstream: "tests/find/files0-from.sh" do
  context "without a file argument" do
    subject(:result) { find("-files0-from") }

    it_behaves_like "a rejected command line", "find: missing argument to `-files0-from'"
  end

  context "with starting points also given on the command line" do
    subject(:result) { find("OFFENDING", "-files0-from", "FILE") }

    it_behaves_like "a rejected command line", "find: extra operand 'OFFENDING'"

    it { is_expected.to diagnose("find: file operands cannot be combined with -files0-from") }
  end

  context "with a file listing two starting points" do
    subject(:result) { find("-files0-from", "in", "-print") }

    before do
      sandbox.touch("a", "b")
      sandbox.write("in", "a#{0.chr}b#{0.chr}")
    end

    it_behaves_like "a successful search", "a", "b"
  end

  context "with starting points that look like tests or actions" do
    subject(:result) { find("-files0-from", "in", "-printf", "%p\n") }

    before do
      sandbox.touch("-print", "-mtime", "-size")
      sandbox.write("in", ["-print", "-mtime", "-size"].join(0.chr))
    end

    it_behaves_like "a successful search", "-print", "-mtime", "-size"
  end

  context "with a directory as starting point" do
    subject(:result) { find("-files0-from", "-", stdin: "d1") }

    before do
      sandbox.mkdir("d1/d2/d3")
      sandbox.touch("d1/d2/d3/file")
    end

    it_behaves_like "a successful search", "d1", "d1/d2", "d1/d2/d3", "d1/d2/d3/file"
  end

  context "with -maxdepth 0" do
    subject(:result) { find("-files0-from", "-", "-maxdepth", "0", stdin: listing.join(0.chr)) }

    let(:listing) { ["d1", "d1/d2", "d1/d2/d3", "d1/d2/d3/file"] }

    before do
      sandbox.mkdir("d1/d2/d3")
      sandbox.touch("d1/d2/d3/file")
    end

    it_behaves_like "a successful search", "d1", "d1/d2", "d1/d2/d3", "d1/d2/d3/file"
  end

  context "with an empty file" do
    subject(:result) { find("-files0-from", File::NULL) }

    it_behaves_like "a successful search"
  end

  context "with empty standard input" do
    subject(:result) { find("-files0-from", "-") }

    it_behaves_like "a successful search"
  end

  context "with a zero-length file name in a file" do
    subject(:result) { find("-files0-from", "file", "-print") }

    before do
      sandbox.touch("a", "b")
      sandbox.write("file", "a#{0.chr}#{0.chr}b#{0.chr}")
    end

    it_behaves_like "a failed search", "a", "b"

    it { is_expected.to diagnose("find: 'file':2: invalid zero-length file name") }
  end

  context "with a zero-length file name on standard input" do
    subject(:result) { find("-files0-from", "-", "-print", stdin: "a#{0.chr}#{0.chr}b#{0.chr}") }

    before { sandbox.touch("a", "b") }

    it_behaves_like "a failed search", "a", "b"

    it { is_expected.to diagnose("find: '(standard input)':2: invalid zero-length file name") }
  end

  context "with a starting point that does not exist" do
    subject(:result) { find("-files0-from", "-", "-print", stdin: ["a", "ENOENT", "b"].join(0.chr)) }

    before { sandbox.touch("a", "b") }

    it_behaves_like "a failed search", "a", "b"

    it { is_expected.to diagnose("find: 'ENOENT': No such file or directory") }
  end

  context "when given several times" do
    subject(:result) { find("-files0-from", "m1", "-files0-from", "m2", "-files0-from", "m3") }

    before do
      ["m1", "m2", "m3"].each { sandbox.write(it, "#{it}#{0.chr}") }
    end

    it_behaves_like "a successful search", "m3"
  end

  context "with a large number of entries" do
    subject(:result) { find("-files0-from", "exp") }

    before { sandbox.write("exp", "exp#{0.chr}" * 100_000) }

    it "visits every entry" do
      expect(result.lines).to eq(["exp"] * 100_000)
    end
  end

  context "with a file that does not exist" do
    subject(:result) { find("-files0-from", "ENOENT") }

    it_behaves_like "a rejected command line", "find: cannot open 'ENOENT' for reading: No such file or directory"
  end

  context "with a file that cannot be read", :unprivileged do
    subject(:result) { find("-files0-from", "secret") }

    before do
      sandbox.touch("secret")
      sandbox.chmod(0o000, "secret")
    end

    it_behaves_like "a rejected command line", "find: cannot open 'secret' for reading: Permission denied"
  end

  context "with a directory instead of a file" do
    subject(:result) { find("-files0-from", "/") }

    it_behaves_like "a rejected command line", "find: '/': read error: Is a directory"
  end

  context "with a closing parenthesis as the first expression argument" do
    subject(:result) { find("-files0-from", "-", ")", "-print") }

    it_behaves_like "a rejected command line",
                    "find: invalid expression: expected expression before closing parentheses ')'."
  end

  context "with a closing parenthesis as the whole expression" do
    subject(:result) { find("-files0-from", "-", ")") }

    it_behaves_like "a rejected command line",
                    "find: invalid expression: expected expression before closing parentheses ')'."
  end

  ["-ok", "-okdir"].each do |action|
    context "with standard input as the file and #{action}" do
      subject(:result) { find("-files0-from", "-", action, "echo", "{}", ";") }

      it_behaves_like "a rejected command line",
                      "find: when -ok or -okdir is in use, the option -files0-from cannot also read from " \
                      "standard input"
    end
  end

  {
    ["in", "in", "-ok"] => "in",
    ["inlink", "in", "-ok"] => "inlink",
    ["in", "inlink", "-ok"] => "in",
    ["inlink", "in", "-okdir"] => "inlink",
    ["in", "inlink", "-okdir"] => "in",
    ["/dev/stdin", "in", "-ok"] => "/dev/stdin",
    ["/dev/stdin", "in", "-okdir"] => "/dev/stdin",
  }.each do |(file, input, action), reported|
    context "with #{file} as the file, standard input read from #{input} and #{action}" do
      subject(:result) { find_with_stdin_from(input, "-files0-from", file, action, "echo", "{}", ";") }

      before do
        sandbox.write("in", ".")
        sandbox.symlink("in", "inlink")
      end

      it_behaves_like "a rejected command line",
                      "find: when -ok or -okdir is in use, the option -files0-from cannot also read from " \
                      "standard input, but the file ('#{reported}') used for -files0-from appears to refer " \
                      "to the same file as the standard input"
    end
  end
end
