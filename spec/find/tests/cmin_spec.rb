RSpec.describe "find -cmin" do
  context "with a file changed just now but accessed and modified a month ago" do
    before do
      sandbox.touch("changed")
      sandbox.utime("changed", atime: Time.now - (30 * 86_400), mtime: Time.now - (30 * 86_400))
    end

    [
      ["1", "less than one minute ago", ["changed"]],
      ["-1", "less than one minute ago or in the future", ["changed"]],
      ["+0", "at any time in the past", ["changed"]],
      ["0", "within the coming minute", []],
      ["-0", "at any time in the future", []],
      ["+1", "more than one minute ago", []],
      ["2", "between one and two minutes ago", []],
    ].each do |count, meaning, matches|
      context "with #{count}, meaning #{meaning}" do
        subject(:result) { find("changed", "-cmin", count) }

        it_behaves_like "a successful search", *matches
      end
    end
  end
end
