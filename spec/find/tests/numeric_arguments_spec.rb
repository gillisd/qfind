RSpec.describe "find numeric arguments" do
  context "with files having one and two hard links" do
    before do
      sandbox.touch("single", "double")
      sandbox.link("double", "double-link")
    end

    [
      ["+1", "greater than one", ["double", "double-link"]],
      ["-2", "less than two", ["single"]],
      ["2", "exactly two", ["double", "double-link"]],
    ].each do |count, meaning, matches|
      context "with #{count}, meaning #{meaning}" do
        subject(:result) { find("single", "double", "double-link", "-links", count) }

        it_behaves_like "a successful search", *matches
      end
    end
  end

  context "with a file having one hard link" do
    before { sandbox.touch("file") }

    {
      "01" => "a leading zero",
      " 1" => "a leading blank",
      "-+2" => "a plus sign after the comparison sign",
    }.each do |count, form|
      context "with #{form} in the count" do
        subject(:result) { find("file", "-links", count) }

        it_behaves_like "a successful search", "file"
      end
    end
  end

  context "with malformed integers" do
    {
      "1.5" => "a fraction",
      "0x1" => "a hexadecimal number",
      "1 " => "a trailing blank",
      "+" => "a comparison sign alone",
      "--1" => "a negative number after the comparison sign",
      "" => "an empty string",
      "18446744073709551616" => "a number too large to represent",
    }.each do |count, form|
      context "with #{form}" do
        subject(:result) { find(".", "-links", count) }

        it_behaves_like "a rejected command line", "find: non-numeric argument to -links: '#{count}'"
      end
    end
  end

  context "with tests taking an integer", upstream: "tests/find/opt-numeric-arg.sh" do
    ["-inum", "-links", "-uid", "-gid"].each do |test|
      context "without an argument to #{test}" do
        subject(:result) { find(test) }

        it_behaves_like "a rejected command line", "find: missing argument to `#{test}'"
      end

      context "with a word as the argument to #{test}" do
        subject(:result) { find(test, "foo") }

        it_behaves_like "a rejected command line", "find: non-numeric argument to #{test}: 'foo'"
      end
    end
  end

  context "with files modified 45 seconds and 20 minutes ago" do
    before do
      sandbox.touch("recent", "older")
      sandbox.utime("recent", atime: Time.now - 45, mtime: Time.now - 45)
      sandbox.utime("older", atime: Time.now - 1_200, mtime: Time.now - 1_200)
    end

    [
      ["-1e1", "an exponent", ["recent"]],
      ["-0x10", "a hexadecimal number", ["recent"]],
      ["+-1", "a negative number after the comparison sign", ["recent", "older"]],
      [" 1", "a leading blank", ["recent"]],
    ].each do |count, form, matches|
      context "with #{form} in the count of minutes" do
        subject(:result) { find("recent", "older", "-mmin", count) }

        it_behaves_like "a successful search", *matches
      end
    end
  end

  context "with malformed counts of time units" do
    {
      "5x" => "trailing characters",
      "+" => "a comparison sign alone",
      "1e999" => "a number too large to represent",
      "" => "an empty string",
    }.each do |count, form|
      context "with #{form}" do
        subject(:result) { find(".", "-mmin", count) }

        it_behaves_like "a rejected command line", "find: invalid argument `#{count}' to `-mmin'"
      end
    end

    context "with a comparison sign before NaN" do
      subject(:result) { find(".", "-mmin", "-nan") }

      it_behaves_like "a rejected command line", "find: invalid not-a-number argument: `nan'"
    end
  end

  context "with tests counting time units" do
    ["-amin", "-atime", "-cmin", "-ctime", "-mmin", "-mtime"].each do |test|
      context "without a count for #{test}" do
        subject(:result) { find(".", test) }

        it_behaves_like "a rejected command line", "find: missing argument to `#{test}'"
      end

      context "with a word as the count for #{test}" do
        subject(:result) { find(".", test, "recently") }

        it_behaves_like "a rejected command line", "find: invalid argument `recently' to `#{test}'"
      end
    end
  end

  context "with NaN as the count", upstream: "tests/find/arg-nan.sh" do
    ["-used", "-amin", "-cmin", "-mmin", "-atime", "-ctime", "-mtime"].each do |test|
      context "with #{test}" do
        subject(:result) { find(test, "NaN") }

        it_behaves_like "a rejected command line", "find: invalid not-a-number argument: `NaN'"
      end
    end
  end
end
