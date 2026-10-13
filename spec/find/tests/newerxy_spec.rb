RSpec.describe "find -newerXY" do
  context "with three files created in succession", upstream: "tests/find/newer.sh" do
    include_context "with three files created in succession"

    ["-neweraa", "-newerac", "-neweram", "-newerca", "-newercc", "-newercm", "-newerma", "-newermc", "-newermm"]
      .each do |test|
        context "with #{test} and the second file as reference" do
          subject(:result) { find(".", test, "file2", "-name", "file*") }

          it_behaves_like "a successful search", "./file3"
        end
      end

    ["-newerat", "-newerct", "-newermt"].each do |test|
      context "with #{test} and the modification time of the second file" do
        subject(:result) { find(".", test, modification_time, "-name", "file*") }

        let(:modification_time) { sandbox.join("file2").mtime.strftime("%Y-%m-%d %H:%M:%S.%N %z") }

        it_behaves_like "a successful search", "./file3"
      end
    end
  end

  context "with a reference accessed twenty days ago, modified ten days ago and changed just now" do
    before do
      sandbox.touch("15-days-ago", "5-days-ago", "tomorrow", "reference")
      sandbox.utime("15-days-ago", atime: Time.now - (30 * 86_400), mtime: Time.now - (15 * 86_400))
      sandbox.utime("5-days-ago", atime: Time.now - (30 * 86_400), mtime: Time.now - (5 * 86_400))
      sandbox.utime("tomorrow", atime: Time.now - (30 * 86_400), mtime: Time.now + 86_400)
      sandbox.utime("reference", atime: Time.now - (20 * 86_400), mtime: Time.now - (10 * 86_400))
    end

    {
      "-newerma" => ["15-days-ago", "5-days-ago", "tomorrow"],
      "-newermm" => ["5-days-ago", "tomorrow"],
      "-newermc" => ["tomorrow"],
    }.each do |test, matches|
      context "with #{test}" do
        subject(:result) { find("15-days-ago", "5-days-ago", "tomorrow", test, "reference") }

        it_behaves_like "a successful search", *matches
      end
    end
  end

  context "with files accessed and modified on either side of a time" do
    before do
      sandbox.touch("accessed-in-2020", "modified-in-2020")
      sandbox.utime("accessed-in-2020", atime: Time.utc(2020, 6, 1), mtime: Time.utc(2019, 6, 1))
      sandbox.utime("modified-in-2020", atime: Time.utc(2019, 6, 1), mtime: Time.utc(2020, 6, 1))
    end

    {
      "-newerat" => ["accessed-in-2020"],
      "-newermt" => ["modified-in-2020"],
      "-newerct" => ["accessed-in-2020", "modified-in-2020"],
    }.each do |test, matches|
      context "with #{test}" do
        subject(:result) { find("accessed-in-2020", "modified-in-2020", test, "2020-01-01 00:00:00") }

        it_behaves_like "a successful search", *matches
      end
    end
  end

  context "with files modified around the start of 2020" do
    before do
      sandbox.touch("last-day-of-2019", "early-2020")
      sandbox.utime("last-day-of-2019", atime: Time.utc(2019, 12, 31, 12), mtime: Time.utc(2019, 12, 31, 12))
      sandbox.utime("early-2020", atime: Time.utc(2020, 1, 1, 3), mtime: Time.utc(2020, 1, 1, 3))
    end

    {
      "a date and time" => "2020-01-01 00:00:00",
      "a date alone" => "2020-01-01",
      "an ISO 8601 timestamp" => "2020-01-01T00:00:00Z",
      "seconds since the epoch" => "@1577836800",
    }.each do |form, time|
      context "with #{form}" do
        subject(:result) { find("last-day-of-2019", "early-2020", "-newermt", time) }

        it_behaves_like "a successful search", "early-2020"
      end
    end

    context "with a time zone five hours behind UTC in the environment" do
      subject(:result) do
        find("last-day-of-2019", "early-2020", "-newermt", "2020-01-01 00:00:00", env: { "TZ" => "EST5" })
      end

      it_behaves_like "a successful search"
    end
  end

  context "with files modified two and four days ago" do
    subject(:result) { find("2-days-ago", "4-days-ago", "-newermt", "3 days ago") }

    before do
      sandbox.touch("2-days-ago", "4-days-ago")
      sandbox.utime("2-days-ago", atime: Time.now - (2 * 86_400), mtime: Time.now - (2 * 86_400))
      sandbox.utime("4-days-ago", atime: Time.now - (4 * 86_400), mtime: Time.now - (4 * 86_400))
    end

    it_behaves_like "a successful search", "2-days-ago"
  end

  context "with files modified exactly at and one nanosecond after the time" do
    subject(:result) { find("exactly", "just-after", "-newermt", "@1600000000.123456789") }

    before do
      sandbox.touch("exactly", "just-after")
      sandbox.utime("exactly", atime: Time.at(1_600_000_000, 123_456_789, :nsec),
                               mtime: Time.at(1_600_000_000, 123_456_789, :nsec))
      sandbox.utime("just-after", atime: Time.at(1_600_000_000, 123_456_790, :nsec),
                                  mtime: Time.at(1_600_000_000, 123_456_790, :nsec))
    end

    it_behaves_like "a successful search", "just-after"
  end

  context "with a time that cannot be parsed" do
    subject(:result) { find(".", "-newermt", "not a date") }

    it_behaves_like "a rejected command line",
                    "find: I cannot figure out how to interpret 'not a date' as a date or time"
  end

  context "without a reference file", upstream: "find.gnu/sv-bug-24169.exp" do
    subject(:result) { find("-neweraa") }

    it_behaves_like "a rejected command line", "find: The '-neweraa' test needs to be followed by an argument"
  end

  context "without a time" do
    subject(:result) { find(".", "-newermt") }

    it_behaves_like "a rejected command line", "find: The '-newermt' test needs to be followed by an argument"
  end

  ["-newerta", "-newertc", "-newertm", "-newertt"].each do |test|
    context "with #{test}, taking the time of the file being considered from the argument" do
      subject(:result) { find(".", test, "2020-01-01") }

      it_behaves_like "a rejected command line", "find: invalid predicate `#{test}'"
    end
  end

  ["-newerxm", "-newermx", "-newerMM"].each do |test|
    context "with #{test}, naming an unknown timestamp" do
      subject(:result) { find(".", test, "reference") }

      before { sandbox.touch("reference") }

      it_behaves_like "a rejected command line", "find: invalid predicate `#{test}'"
    end
  end

  ["-newerm", "-newermmm"].each do |test|
    context "with #{test}, naming the wrong number of timestamps" do
      subject(:result) { find(".", test, "reference") }

      before { sandbox.touch("reference") }

      it_behaves_like "a rejected command line", "find: unknown predicate `#{test}'"
    end
  end

  ["-newerBa", "-newerBB", "-newerBc", "-newerBm", "-newerBt", "-neweraB", "-newercB", "-newermB", "-newertB"]
    .each do |test|
      context "with #{test}, using the birth time this system cannot provide" do
        subject(:result) { find(".", test, "reference") }

        before { sandbox.touch("reference") }

        it_behaves_like "a rejected command line",
                        "find: This system does not provide a way to find the birth time of a file."

        it { is_expected.to diagnose("find: invalid predicate `#{test}'") }
      end
    end

  it_behaves_like "a test comparing against a reference file", "-newerma"
end
