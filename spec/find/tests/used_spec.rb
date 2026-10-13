RSpec.describe "find -used" do
  context "with files accessed up to forty days after their status changed", upstream: "tests/find/used.sh" do
    before do
      sandbox.touch("t00", "t10", "t20", "t30", "t40")
      [10, 20, 30, 40].each { sandbox.utime("t#{it}", atime: Time.now + (it * 86_400), mtime: Time.now) }
    end

    {
      "-45" => ["./t00", "./t10", "./t20", "./t30", "./t40"],
      "-35" => ["./t00", "./t10", "./t20", "./t30"],
      "-25" => ["./t00", "./t10", "./t20"],
      "-15" => ["./t00", "./t10"],
      "-5" => ["./t00"],
      "0" => [],
      "5" => [],
      "15" => [],
      "25" => [],
      "35" => [],
      "45" => [],
      "+0" => ["./t10", "./t20", "./t30", "./t40"],
      "+5" => ["./t10", "./t20", "./t30", "./t40"],
      "+15" => ["./t20", "./t30", "./t40"],
      "+25" => ["./t30", "./t40"],
      "+35" => ["./t40"],
      "+45" => [],
    }.each do |days, matches|
      context "with #{days}" do
        subject(:result) { find(".", "-type", "f", "-name", "t*", "-used", days) }

        it_behaves_like "a successful search", *matches
      end
    end
  end

  context "with a file accessed when its status changed" do
    subject(:result) { find("untouched", "-used", "1") }

    before { sandbox.touch("untouched") }

    it_behaves_like "a successful search", "untouched"
  end

  context "with a file accessed ten and a half days after its status changed" do
    before do
      sandbox.touch("accessed")
      sandbox.utime("accessed", atime: Time.now + (10.5 * 86_400), mtime: Time.now)
    end

    context "with 11, counting the started day" do
      subject(:result) { find("accessed", "-used", "11") }

      it_behaves_like "a successful search", "accessed"
    end

    context "with 10, the number of whole days elapsed" do
      subject(:result) { find("accessed", "-used", "10") }

      it_behaves_like "a successful search"
    end
  end

  context "with a file accessed before its status last changed" do
    subject(:result) { find("accessed-earlier", "-used", "-100") }

    before do
      sandbox.touch("accessed-earlier")
      sandbox.utime("accessed-earlier", atime: Time.now - (10 * 86_400), mtime: Time.now)
    end

    it_behaves_like "a successful search"
  end

  context "with an argument that is not a number", upstream: "find.gnu/used-invarg.exp" do
    subject(:result) { find(".", "-used", "ZZ") }

    it_behaves_like "a rejected command line", "find: Invalid argument ZZ to -used"
  end

  context "without an argument", upstream: "find.gnu/used-missing.exp" do
    subject(:result) { find(".", "-used") }

    it_behaves_like "a rejected command line", "find: missing argument to `-used'"
  end
end
