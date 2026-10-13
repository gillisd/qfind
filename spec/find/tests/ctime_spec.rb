RSpec.describe "find -ctime" do
  context "with a file changed just now but accessed and modified a month ago" do
    before do
      sandbox.touch("changed")
      sandbox.utime("changed", atime: Time.now - (30 * 86_400), mtime: Time.now - (30 * 86_400))
    end

    [
      ["0", "less than one day ago", ["changed"]],
      ["-1", "less than one day ago or in the future", ["changed"]],
      ["+0", "at least one whole day ago", []],
      ["1", "between one and two days ago", []],
      ["+7", "at least eight whole days ago", []],
      ["+-1", "at any time in the past", ["changed"]],
    ].each do |count, meaning, matches|
      context "with #{count}, meaning #{meaning}" do
        subject(:result) { find("changed", "-ctime", count) }

        it_behaves_like "a successful search", *matches
      end
    end
  end
end
