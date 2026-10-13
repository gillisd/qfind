RSpec.describe "find -printf time directives" do
  conversions = {
    "@" => "seconds since the epoch",
    "H" => "hour (00..23)",
    "I" => "hour (01..12)",
    "k" => "hour ( 0..23)",
    "l" => "hour ( 1..12)",
    "M" => "minute",
    "p" => "AM or PM indicator",
    "r" => "12-hour time",
    "S" => "second",
    "T" => "24-hour time",
    "+" => "date and time joined by a plus sign",
    "X" => "time representation",
    "Z" => "time zone",
    "a" => "abbreviated weekday name",
    "A" => "full weekday name",
    "b" => "abbreviated month name",
    "B" => "full month name",
    "c" => "date and time representation",
    "d" => "day of the month",
    "D" => "date as mm/dd/yy",
    "F" => "date as yyyy-mm-dd",
    "h" => "abbreviated month name",
    "j" => "day of the year",
    "m" => "month",
    "U" => "week of the year starting on Sunday",
    "w" => "day of the week",
    "W" => "week of the year starting on Monday",
    "x" => "date representation",
    "y" => "last two digits of the year",
    "Y" => "year",
  }

  def timestamp_of(path, timestamp) = File.lstat(sandbox.join(path)).public_send(timestamp).getutc

  def fraction_of(time) = format(".%09d0", time.nsec)

  def formatted(time, conversion)
    case conversion
    when "@" then "#{time.to_i}#{fraction_of(time)}"
    when "+" then "#{time.strftime("%F+%T")}#{fraction_of(time)}"
    when "S", "T", "X" then "#{time.strftime("%#{conversion}")}#{fraction_of(time)}"
    else time.strftime("%#{conversion}")
    end
  end

  def ctime_style(time) = "#{time.strftime("%a %b %e %T")}#{fraction_of(time)} #{time.strftime("%Y")}"

  shared_examples "a directive formatting a timestamp" do |directive, timestamp|
    conversions.each do |conversion, meaning|
      context "with %#{directive}#{conversion}, the #{meaning}" do
        subject(:result) { find("file", "-printf", "%#{directive}#{conversion}") }

        it { is_expected.to print_exactly(formatted(timestamp_of("file", timestamp), conversion)) }
      end
    end
  end

  let(:accessed) { Time.utc(1999, 12, 31, 9, 4, 5.987654321r) }
  let(:modified) { Time.utc(1990, 6, 2, 15, 4, 5.123456789r) }

  before do
    sandbox.touch("file")
    sandbox.utime("file", atime: accessed, mtime: modified)
  end

  {
    "a" => [:atime, "the last access time"],
    "c" => [:ctime, "the last status change time"],
    "t" => [:mtime, "the last modification time"],
  }.each do |directive, (timestamp, meaning)|
    context "with %#{directive}" do
      subject(:result) { find("file", "-printf", "%#{directive}") }

      it "prints #{meaning} in the format of ctime(3) with a fractional part" do
        expect(result).to print_exactly(ctime_style(timestamp_of("file", timestamp)))
      end
    end
  end

  context "with the last access time" do
    it_behaves_like "a directive formatting a timestamp", "A", :atime
  end

  context "with the last status change time" do
    it_behaves_like "a directive formatting a timestamp", "C", :ctime
  end

  context "with the last modification time" do
    it_behaves_like "a directive formatting a timestamp", "T", :mtime
  end

  context "with further strftime(3) conversions" do
    {
      "e" => "day of the month padded with a space",
      "C" => "century",
      "G" => "ISO 8601 week-based year",
      "V" => "ISO 8601 week number",
      "u" => "day of the week counting Monday as 1",
      "R" => "hour and minute",
      "s" => "seconds since the epoch without a fractional part",
      "z" => "numeric time zone",
      "%" => "literal percent sign",
    }.each do |conversion, meaning|
      context "with %T#{conversion}, the #{meaning}" do
        subject(:result) { find("file", "-printf", "%T#{conversion}") }

        it { is_expected.to print_exactly(timestamp_of("file", :mtime).strftime("%#{conversion}")) }
      end
    end
  end

  context "with the birth time, which stat(2) does not report on GNU/Linux" do
    conversions.except("@").each do |conversion, meaning|
      context "with %B#{conversion}, the #{meaning}" do
        subject(:result) { find("file", "-printf", "%B#{conversion}") }

        it { is_expected.to print_nothing }
      end
    end

    context "with %B@" do
      subject(:result) { find("file", "-printf", "%B@") }

      it "prints a placeholder of minus one second and minus one nanosecond instead of nothing" do
        expect(result).to print_exactly("-1.-000000010")
      end
    end

    context "with text around %BY" do
      subject(:result) { find("file", "-printf", "[%BY] %p\n") }

      it "still prints the rest of the format" do
        expect(result).to print_exactly("[] file\n")
      end
    end

    context "with a width on %BY" do
      subject(:result) { find("file", "-printf", "[%5BY]") }

      it "pads the empty field to the width" do
        expect(result).to print_exactly("[     ]")
      end
    end
  end

  context "with TZ set to a zone nine hours ahead of UTC" do
    context "with %T+" do
      subject(:result) { find("file", "-printf", "%T+", env: { "TZ" => "JST-9" }) }

      it "prints the date and time in that zone" do
        expect(result).to print_exactly(formatted(timestamp_of("file", :mtime).getlocal("+09:00"), "+"))
      end
    end

    context "with %t" do
      subject(:result) { find("file", "-printf", "%t", env: { "TZ" => "JST-9" }) }

      it "prints the date and time in that zone" do
        expect(result).to print_exactly(ctime_style(timestamp_of("file", :mtime).getlocal("+09:00")))
      end
    end

    context "with %TZ" do
      subject(:result) { find("file", "-printf", "%TZ", env: { "TZ" => "JST-9" }) }

      it "prints the abbreviation of that zone" do
        expect(result).to print_exactly("JST")
      end
    end

    context "with %T@" do
      subject(:result) { find("file", "-printf", "%T@", env: { "TZ" => "JST-9" }) }

      it "prints the same seconds since the epoch as in UTC" do
        expect(result).to print_exactly(formatted(timestamp_of("file", :mtime), "@"))
      end
    end
  end

  context "with a modification time half a second before the epoch" do
    before { sandbox.utime("file", atime: accessed, mtime: Time.utc(1969, 12, 31, 23, 59, 59.5r)) }

    context "with %T@" do
      subject(:result) { find("file", "-printf", "%T@") }

      it "prints the whole seconds rounded down followed by the positive fraction" do
        expect(result).to print_exactly("-1.5000000000")
      end
    end

    context "with %T+" do
      subject(:result) { find("file", "-printf", "%T+") }

      it "prints the date and time before the epoch" do
        expect(result).to print_exactly("1969-12-31+23:59:59.5000000000")
      end
    end
  end

  context "with a width on %TH" do
    subject(:result) { find("file", "-printf", "%5TH") }

    it "right-aligns the hour" do
      expect(result).to print_exactly(formatted(timestamp_of("file", :mtime), "H").rjust(5))
    end
  end

  context "with the - flag and a width on %TH" do
    subject(:result) { find("file", "-printf", "%-5TH") }

    it "left-aligns the hour" do
      expect(result).to print_exactly(formatted(timestamp_of("file", :mtime), "H").ljust(5))
    end
  end

  context "with the + and 0 flags and a width on %TH" do
    subject(:result) { find("file", "-printf", "%+05TH") }

    it "ignores the flags and pads the hour with spaces" do
      expect(result).to print_exactly(formatted(timestamp_of("file", :mtime), "H").rjust(5))
    end
  end

  context "with a precision on %T@" do
    subject(:result) { find("file", "-printf", "%.4T@") }

    it "truncates the seconds since the epoch like a string" do
      expect(result).to print_exactly(formatted(timestamp_of("file", :mtime), "@")[0, 4])
    end
  end

  context "with a precision on %t" do
    subject(:result) { find("file", "-printf", "%.3t") }

    it "keeps only the weekday" do
      expect(result).to print_exactly(ctime_style(timestamp_of("file", :mtime))[0, 3])
    end
  end

  ["A", "B", "C", "T"].each do |directive|
    context "with %#{directive} at the end of the format" do
      subject(:result) { find("file", "-printf", "x%#{directive}") }

      it_behaves_like "a successful search that warns",
                      "find: warning: format directive `%#{directive}' should be followed by another character",
                      "x%#{directive}"
    end
  end
end
