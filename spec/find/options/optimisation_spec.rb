RSpec.describe "find -O" do
  before do
    sandbox.mkdir("dir")
    sandbox.touch("dir/file")
  end

  [0, 1, 2, 3, 4, 65_535].each do |level|
    context "with level #{level}" do
      subject(:result) { find("-O#{level}", "dir", "-name", "file") }

      it { is_expected.to be_successful }
      it { is_expected.to print_lines("dir/file") }
    end

    context "with level #{level} and --version" do
      subject(:features) { find("-O#{level}", "--version").lines.last }

      it { is_expected.to include("CBO(level=#{level})") }
    end
  end

  context "without the option and with --version" do
    subject(:features) { find("--version").lines.last }

    it { is_expected.to include("CBO(level=1)") }
  end

  context "when given twice" do
    subject(:features) { find("-O3", "-O0", "--version").lines.last }

    it { is_expected.to include("CBO(level=0)") }
  end

  {
    "without a level" => ["-O", "find: The -O option must be immediately followed by a decimal integer"],
    "with a level that is not a number" => ["-Ofoo", "find: Please specify a decimal number immediately after -O"],
    "with a negative level" => ["-O-1", "find: Please specify a decimal number immediately after -O"],
    "with characters trailing the level" => ["-O1x", "find: Invalid optimisation level 1x"],
    "with a level above 65535" => [
      "-O65536",
      "find: Optimisation level 65536 is too high.  If you want to find files very quickly, consider using GNU locate.",
    ],
    "with a level out of the numeric range" => [
      "-O99999999999999999999",
      "find: Invalid optimisation level 99999999999999999999: Numerical result out of range",
    ],
  }.each do |description, (option, diagnostic)|
    context description do
      subject(:result) { find(option, "dir") }

      it_behaves_like "a rejected command line", diagnostic
    end
  end

  context "with the level in a separate argument" do
    subject(:result) { find("-O", "1", "dir") }

    it_behaves_like "a rejected command line", "find: The -O option must be immediately followed by a decimal integer"
  end

  context "with the option after a starting point" do
    subject(:result) { find("dir", "-O3") }

    it_behaves_like "a rejected command line", "find: unknown predicate `-O3'"
  end

  context "with -D opt and tests of differing cost" do
    subject(:result) { find("-O#{level}", "-D", "opt", "dir", "-size", "0", "-type", "f", "-name", "file") }

    [0, 1].each do |optimisation|
      context "with level #{optimisation}" do
        let(:level) { optimisation }

        it "moves the test on the name to the front" do
          expect(result).to diagnose("-O#{optimisation}: promoting cheap predicate -name file into name_list")
        end

        it "leaves the other tests in place" do
          expect(result).not_to diagnose(a_string_including("categorising predicate"))
        end
      end
    end

    context "with level 2" do
      let(:level) { 2 }

      it "moves the test on the type ahead of the tests needing the inode" do
        expect(result).to diagnose("-O2: categorising predicate -type f by cost (Type)")
      end

      it "leaves the tests needing the inode in place" do
        expect(result).not_to diagnose(a_string_including("categorising predicate -size 0"))
      end
    end

    context "with level 3" do
      let(:level) { 3 }

      it "orders every test by cost" do
        expect(result).to diagnose("-O3: categorising predicate -size 0 by cost (StatInfo)")
      end
    end
  end

  context "with -D opt and a test for a file system type that is not mounted" do
    subject(:result) { find("-O#{level}", "-D", "opt", "dir", "-fstype", "nosuchfs") }

    let(:substitution) { "-fstype nosuchfs can never succeed, substituting -false" }

    [0, 1].each do |optimisation|
      context "with level #{optimisation}" do
        let(:level) { optimisation }

        it "keeps the test" do
          expect(result).not_to diagnose(substitution)
        end
      end
    end

    [2, 3].each do |optimisation|
      context "with level #{optimisation}" do
        let(:level) { optimisation }

        it "replaces the test with -false" do
          expect(result).to diagnose(substitution)
        end
      end
    end
  end

  context "with a test needing the inode before -false and an entry that cannot be examined", :unprivileged do
    subject(:result) { find("-O#{level}", "r", "-size", "0", "-false") }

    before do
      sandbox.mkdir("r")
      sandbox.touch("r/f")
      sandbox.chmod(0o444, "r")
    end

    context "with level 0" do
      let(:level) { 0 }

      it "keeps the given order and so fails to examine the entry" do
        expect(result).to diagnose("find: 'r/f': Permission denied")
      end
    end

    [1, 2, 3].each do |optimisation|
      context "with level #{optimisation}" do
        let(:level) { optimisation }

        it "evaluates -false first and so never examines the entry" do
          expect(result).to diagnose_nothing
        end
      end
    end
  end

  context "with -readable before -empty and an unreadable directory", :unprivileged do
    subject(:result) { find("-O#{level}", "d", "-readable", "-empty") }

    let(:denial) { "find: 'd/locked': Permission denied" }

    before do
      sandbox.mkdir("d/locked", "d/open")
      sandbox.chmod(0o000, "d/locked")
    end

    [0, 1].each do |optimisation|
      context "with level #{optimisation}" do
        let(:level) { optimisation }

        it "keeps the given order and so reports the directory once" do
          expect(result).to diagnose_exactly(denial)
        end
      end
    end

    [2, 3].each do |optimisation|
      context "with level #{optimisation}" do
        let(:level) { optimisation }

        it "evaluates -empty first and so reports the directory twice" do
          expect(result).to diagnose_exactly(denial, denial)
        end
      end
    end
  end
end
