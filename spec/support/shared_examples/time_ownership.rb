require "etc"

RSpec.shared_context "with files aged by one timestamp" do |timestamp, ages|
  before do
    ages.each do |name, seconds|
      sandbox.touch(name)
      times = { atime: Time.now - (30 * 86_400), mtime: Time.now - (30 * 86_400) }
      sandbox.utime(name, **times, timestamp => Time.now - seconds)
    end
  end
end

RSpec.shared_context "with a file whose other timestamps are recent" do |timestamp|
  before do
    sandbox.touch("stale")
    times = { atime: Time.now, mtime: Time.now }
    sandbox.utime("stale", **times, timestamp => Time.now - (30 * 86_400))
  end
end

RSpec.shared_examples "a minute-based timestamp test" do |test, timestamp|
  context "with files aged from seconds ago to seconds ahead" do
    include_context "with files aged by one timestamp", timestamp,
                    "45s-ago" => 45, "105s-ago" => 105, "90m-ago" => 5_400, "45s-ahead" => -45

    [
      ["-2", "less than two minutes ago or in the future", ["45s-ago", "105s-ago", "45s-ahead"]],
      ["2", "between one and two minutes ago", ["105s-ago"]],
      ["1", "less than one minute ago", ["45s-ago"]],
      ["0", "within the coming minute", ["45s-ahead"]],
      ["+1", "more than one minute ago", ["105s-ago", "90m-ago"]],
      ["+0", "at any time in the past", ["45s-ago", "105s-ago", "90m-ago"]],
      ["-0", "at any time in the future", ["45s-ahead"]],
      ["+60", "more than an hour ago", ["90m-ago"]],
      ["-1.5", "less than ninety seconds ago or in the future", ["45s-ago", "45s-ahead"]],
    ].each do |count, meaning, matches|
      context "with #{count}, meaning #{meaning}" do
        subject(:result) { find("45s-ago", "105s-ago", "90m-ago", "45s-ahead", test, count) }

        it_behaves_like "a successful search", *matches
      end
    end
  end

  context "with a file whose other timestamps are recent" do
    subject(:result) { find("stale", test, "-60") }

    include_context "with a file whose other timestamps are recent", timestamp

    it_behaves_like "a successful search"
  end
end

RSpec.shared_examples "a day-based timestamp test" do |test, timestamp|
  context "with files aged from hours ago to days ahead" do
    include_context "with files aged by one timestamp", timestamp,
                    "6h-ago" => 6 * 3_600, "36h-ago" => 36 * 3_600, "60h-ago" => 60 * 3_600, "36h-ahead" => -36 * 3_600

    [
      ["0", "less than one day ago", ["6h-ago"]],
      ["1", "between one and two days ago", ["36h-ago"]],
      ["2", "between two and three days ago", ["60h-ago"]],
      ["-1", "less than one day ago or in the future", ["6h-ago", "36h-ahead"]],
      ["-2", "less than two days ago or in the future", ["6h-ago", "36h-ago", "36h-ahead"]],
      ["+0", "at least one whole day ago", ["36h-ago", "60h-ago"]],
      ["+1", "at least two whole days ago", ["60h-ago"]],
      ["-0.5", "less than half a day ago or in the future", ["6h-ago", "36h-ahead"]],
    ].each do |count, meaning, matches|
      context "with #{count}, meaning #{meaning}" do
        subject(:result) { find("6h-ago", "36h-ago", "60h-ago", "36h-ahead", test, count) }

        it_behaves_like "a successful search", *matches
      end
    end
  end

  context "with a file whose other timestamps are recent" do
    subject(:result) { find("stale", test, "-1") }

    include_context "with a file whose other timestamps are recent", timestamp

    it_behaves_like "a successful search"
  end
end

RSpec.shared_context "with three files created in succession" do
  before do
    sandbox.touch("file1")
    create_changed_after("file1", "file2")
    create_changed_after("file2", "file3")
  end

  def create_changed_after(earlier, later)
    sandbox.touch(later)
    until sandbox.join(later).stat.ctime > sandbox.join(earlier).stat.ctime
      sandbox.join(later).delete
      sandbox.touch(later)
    end
  end
end

RSpec.shared_examples "a test comparing against a reference file" do |test|
  context "with a symbolic link to an older file as reference" do
    before do
      sandbox.touch("ancient", "between")
      sandbox.utime("ancient", atime: Time.now - (40 * 86_400), mtime: Time.now - (40 * 86_400))
      sandbox.utime("between", atime: Time.now - (15 * 86_400), mtime: Time.now - (15 * 86_400))
      sandbox.symlink("ancient", "link")
    end

    context "without -H or -L" do
      subject(:result) { find("between", test, "link") }

      it_behaves_like "a successful search"
    end

    context "with -H" do
      subject(:result) { find("-H", "between", test, "link") }

      it_behaves_like "a successful search", "between"
    end

    context "with -L" do
      subject(:result) { find("-L", "between", test, "link") }

      it_behaves_like "a successful search", "between"
    end

    context "with -follow before the test" do
      subject(:result) { find("between", "-follow", test, "link") }

      it_behaves_like "a successful search", "between"
    end

    context "with -follow after the test" do
      subject(:result) { find("between", test, "link", "-follow") }

      it_behaves_like "a successful search"
    end
  end

  context "with a dangling symbolic link as reference and -L" do
    subject(:result) { find("-L", "between", test, "dangling") }

    before do
      sandbox.touch("between")
      sandbox.utime("between", atime: Time.now - (15 * 86_400), mtime: Time.now - (15 * 86_400))
      sandbox.symlink("nowhere", "dangling")
    end

    it_behaves_like "a successful search"
  end

  context "with a reference file modified while the search runs" do
    subject(:result) { find("first", "second", test, "reference", "-print", "-exec", "touch", "reference", ";") }

    before do
      sandbox.touch("first", "second", "reference")
      sandbox.utime("first", atime: Time.now - (5 * 86_400), mtime: Time.now - (5 * 86_400))
      sandbox.utime("second", atime: Time.now - (5 * 86_400), mtime: Time.now - (5 * 86_400))
      sandbox.utime("reference", atime: Time.now - (10 * 86_400), mtime: Time.now - (10 * 86_400))
    end

    it_behaves_like "a successful search", "first", "second"
  end

  context "with a reference file that does not exist" do
    subject(:result) { find(".", test, "missing") }

    it_behaves_like "a rejected command line", "find: 'missing': No such file or directory"
  end

  context "with a reference file in an unreadable directory", :unprivileged do
    subject(:result) { find(".", test, "locked/reference") }

    before do
      sandbox.mkdir("locked")
      sandbox.touch("locked/reference")
      sandbox.chmod(0o000, "locked")
    end

    it_behaves_like "a rejected command line", "find: 'locked/reference': Permission denied"
  end
end

RSpec.shared_examples "a test matching an owner by name or id" do |test, kind|
  before { sandbox.touch("mine") }

  context "with the name of the owner" do
    subject(:result) { find("mine", test, owner_name) }

    it_behaves_like "a successful search", "mine"
  end

  context "with the numeric id of the owner" do
    subject(:result) { find("mine", test, owner_id.to_s) }

    it_behaves_like "a successful search", "mine"
  end

  context "with a plus sign before the numeric id of the owner" do
    subject(:result) { find("mine", test, "+#{owner_id}") }

    it_behaves_like "a successful search", "mine"
  end

  context "with a leading zero before the numeric id of the owner" do
    subject(:result) { find("mine", test, "0#{owner_id}") }

    it_behaves_like "a successful search", "mine"
  end

  context "with the numeric id of a different owner" do
    subject(:result) { find("mine", test, (owner_id + 1).to_s) }

    it_behaves_like "a successful search"
  end

  context "with the name of the owner of the system files", :unprivileged do
    subject(:result) { find("mine", test, "root") }

    it_behaves_like "a successful search"
  end

  context "with a name unknown to the system" do
    subject(:result) { find("mine", test, "no-such-owner") }

    it_behaves_like "a rejected command line", "find: invalid #{kind} argument to #{test}: 'no-such-owner'"
  end

  context "with the largest id the system can represent", upstream: "tests/find/user-group-max.sh" do
    subject(:result) { find(test, "4294967295") }

    it_behaves_like "a successful search"
  end

  context "with an id one beyond the largest the system can represent", upstream: "tests/find/user-group-max.sh" do
    subject(:result) { find(test, "4294967296", "-name", "enoent") }

    it_behaves_like "a rejected command line", "find: invalid #{kind} argument to #{test}: '4294967296'"
  end
end

RSpec.shared_examples "a test comparing numeric owner ids" do |test|
  before { sandbox.touch("mine") }

  context "with the id of the owner" do
    subject(:result) { find("mine", test, owner_id.to_s) }

    it_behaves_like "a successful search", "mine"
  end

  context "with a minus sign before the next higher id" do
    subject(:result) { find("mine", test, "-#{owner_id + 1}") }

    it_behaves_like "a successful search", "mine"
  end

  context "with a minus sign before the id of the owner" do
    subject(:result) { find("mine", test, "-#{owner_id}") }

    it_behaves_like "a successful search"
  end

  context "with a plus sign before the id of the owner" do
    subject(:result) { find("mine", test, "+#{owner_id}") }

    it_behaves_like "a successful search"
  end

  context "with a plus sign before the next lower id", :unprivileged do
    subject(:result) { find("mine", test, "+#{owner_id - 1}") }

    it_behaves_like "a successful search", "mine"
  end

  context "with the id of a different owner" do
    subject(:result) { find("mine", test, (owner_id + 1).to_s) }

    it_behaves_like "a successful search"
  end

  context "with a number beyond the range of ids" do
    subject(:result) { find("mine", test, "18446744073709551615") }

    it_behaves_like "a successful search"
  end

  context "with a name instead of a number" do
    subject(:result) { find("mine", test, "root") }

    it_behaves_like "a rejected command line", "find: non-numeric argument to #{test}: 'root'"
  end
end

RSpec.shared_context "with a file owned by a user id unknown to the system" do
  before do
    sandbox.touch("orphan")
    File.chown(4_000_000_000, nil, sandbox.join("orphan"))
  end
end

RSpec.shared_context "with a file owned by a group id unknown to the system" do
  before do
    sandbox.touch("orphan")
    File.chown(nil, 4_000_000_000, sandbox.join("orphan"))
  end
end
