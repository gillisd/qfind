RSpec.describe "find -daystart" do
  let(:start_of_today) { Time.now.utc.then { Time.utc(it.year, it.month, it.day) } }

  context "with a file modified at the very start of today" do
    before do
      sandbox.touch("midnight")
      sandbox.utime("midnight", atime: start_of_today, mtime: start_of_today)
    end

    context "with -mtime 1 after it" do
      subject(:result) { find("midnight", "-daystart", "-mtime", "1") }

      it_behaves_like "a successful search", "midnight"
    end

    context "with -mtime 0 after it" do
      subject(:result) { find("midnight", "-daystart", "-mtime", "0") }

      it_behaves_like "a successful search"
    end

    context "with -atime 1 after it" do
      subject(:result) { find("midnight", "-daystart", "-atime", "1") }

      it_behaves_like "a successful search", "midnight"
    end

    context "without it" do
      subject(:result) { find("midnight", "-mtime", "0") }

      it_behaves_like "a successful search", "midnight"
    end

    context "with -mtime 0 before it" do
      subject(:result) { find("midnight", "-mtime", "0", "-daystart") }

      it_behaves_like "a successful search", "midnight"
    end

    context "when given twice before -mtime 1" do
      subject(:result) { find("midnight", "-daystart", "-daystart", "-mtime", "1") }

      it_behaves_like "a successful search", "midnight"
    end
  end

  context "with a file modified yesterday at noon" do
    before do
      sandbox.touch("yesterday")
      sandbox.utime("yesterday", atime: start_of_today - 43_200, mtime: start_of_today - 43_200)
    end

    context "with -mtime 1 after it" do
      subject(:result) { find("yesterday", "-daystart", "-mtime", "1") }

      it_behaves_like "a successful search", "yesterday"
    end

    context "with -mtime +0 after it" do
      subject(:result) { find("yesterday", "-daystart", "-mtime", "+0") }

      it_behaves_like "a successful search", "yesterday"
    end

    context "with -mmin -1440 after it, counting minutes back from the end of today" do
      subject(:result) { find("yesterday", "-daystart", "-mmin", "-1440") }

      it_behaves_like "a successful search"
    end
  end

  context "with a file created just now" do
    before { sandbox.touch("fresh") }

    {
      "-mtime" => "0",
      "-atime" => "0",
      "-ctime" => "0",
      "-mmin" => "-1440",
      "-amin" => "-1440",
      "-cmin" => "-1440",
    }.each do |test, count|
      context "with #{test} #{count} after it" do
        subject(:result) { find("fresh", "-daystart", test, count) }

        it_behaves_like "a successful search", "fresh"
      end
    end

    context "with -mtime +0 after it" do
      subject(:result) { find("fresh", "-daystart", "-mtime", "+0") }

      it_behaves_like "a successful search"
    end
  end

  context "with a file modified half a minute before the end of today" do
    before do
      sandbox.touch("late")
      sandbox.utime("late", atime: start_of_today + 86_370, mtime: start_of_today + 86_370)
    end

    context "with -mmin 1 after it" do
      subject(:result) { find("late", "-daystart", "-mmin", "1") }

      it_behaves_like "a successful search", "late"
    end

    context "with -amin 1 after it" do
      subject(:result) { find("late", "-daystart", "-amin", "1") }

      it_behaves_like "a successful search", "late"
    end
  end

  context "with a file modified at the start of today in a time zone five hours behind UTC" do
    subject(:result) { find("local-midnight", "-daystart", "-mtime", "1", env: { "TZ" => "EST5" }) }

    let(:start_of_local_today) do
      Time.now.getlocal("-05:00").then { Time.new(it.year, it.month, it.day, 0, 0, 0, "-05:00") }
    end

    before do
      sandbox.touch("local-midnight")
      sandbox.utime("local-midnight", atime: start_of_local_today, mtime: start_of_local_today)
    end

    it_behaves_like "a successful search", "local-midnight"
  end

  context "when standard input is a terminal and it follows a test" do
    subject(:result) { find_on_terminal("fresh", "-mtime", "0", "-daystart") }

    before { sandbox.touch("fresh") }

    it { is_expected.to diagnose_nothing }
  end

  context "with it as the only operand of -o" do
    subject(:result) { find("fresh", "-daystart", "-o", "-false") }

    before { sandbox.touch("fresh") }

    it_behaves_like "a rejected command line",
                    "find: invalid expression; you have used a binary operator '-o' with nothing before it."
  end

  context "with it as the operand of !" do
    subject(:result) { find("fresh", "!", "-daystart") }

    before { sandbox.touch("fresh") }

    it_behaves_like "a rejected command line", "find: expected an expression after '-daystart'"
  end
end
