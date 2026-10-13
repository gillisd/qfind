RSpec.describe "find -size" do
  def create_file(path, size)
    sandbox.touch(path)
    sandbox.truncate(path, size)
  end

  shared_examples "a size suffix" do |suffix, unit|
    before do
      { "empty" => 0, "one-byte" => 1, "one-unit" => unit, "over-one-unit" => unit + 1 }.each do |name, size|
        create_file(name, size)
      end
    end

    context "with a count of one unit" do
      subject(:result) { find(".", "-type", "f", "-size", "1#{suffix}") }

      it_behaves_like "a successful search", "./one-byte", "./one-unit"
    end

    context "with more than one unit" do
      subject(:result) { find(".", "-type", "f", "-size", "+1#{suffix}") }

      it_behaves_like "a successful search", "./over-one-unit"
    end

    context "with less than one unit" do
      subject(:result) { find(".", "-type", "f", "-size", "-1#{suffix}") }

      it_behaves_like "a successful search", "./empty"
    end
  end

  context "with files around one block in size", upstream: "find.posix/sizes.exp" do
    subject(:result) do
      find("tmp", "-mindepth", "1",
           "(", "-size", "-1", "-printf", "A size -1: %s %p\n", ")", ",",
           "(", "-size", "1", "-printf", "B size  1: %s %p\n", ")", ",",
           "(", "-size", "+1", "-printf", "C size +1: %s %p\n", ")", ",",
           "(", "-size", "-2", "-printf", "G size -2: %s %p\n", ")", ",",
           "(", "-size", "2", "-printf", "H size  2: %s %p\n", ")", ",",
           "(", "-size", "+2", "-printf", "I size +2: %s %p\n", ")")
    end

    before do
      sandbox.mkdir("tmp")
      [0, 511, 512, 513, 1024].each { create_file("tmp/#{it}-bytes", it) }
    end

    it_behaves_like "a successful search",
                    "A size -1: 0 tmp/0-bytes",
                    "B size  1: 511 tmp/511-bytes",
                    "B size  1: 512 tmp/512-bytes",
                    "C size +1: 513 tmp/513-bytes",
                    "C size +1: 1024 tmp/1024-bytes",
                    "H size  2: 513 tmp/513-bytes",
                    "H size  2: 1024 tmp/1024-bytes",
                    "G size -2: 0 tmp/0-bytes",
                    "G size -2: 511 tmp/511-bytes",
                    "G size -2: 512 tmp/512-bytes"
  end

  context "without a suffix" do
    it_behaves_like "a size suffix", "", 512
  end

  context "with the b suffix" do
    it_behaves_like "a size suffix", "b", 512
  end

  context "with the c suffix" do
    it_behaves_like "a size suffix", "c", 1
  end

  context "with the w suffix" do
    it_behaves_like "a size suffix", "w", 2
  end

  context "with the k suffix" do
    it_behaves_like "a size suffix", "k", 1024
  end

  context "with the M suffix" do
    it_behaves_like "a size suffix", "M", 1_048_576
  end

  context "with the G suffix" do
    it_behaves_like "a size suffix", "G", 1_073_741_824
  end

  context "with files up to one byte short of a mebibyte" do
    before do
      { "empty" => 0, "one-byte" => 1, "almost-a-mebibyte" => 1_048_575 }.each { |name, size| create_file(name, size) }
    end

    context "with less than one mebibyte" do
      subject(:result) { find(".", "-type", "f", "-size", "-1M") }

      it_behaves_like "a successful search", "./empty"
    end

    context "with less than the same amount in bytes" do
      subject(:result) { find(".", "-type", "f", "-size", "-1048576c") }

      it_behaves_like "a successful search", "./empty", "./one-byte", "./almost-a-mebibyte"
    end
  end

  context "with -size before -type", upstream: "find.posix/sizetype.exp" do
    subject(:result) { find("tmp", "-size", "0c", "-type", "f") }

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/empty")
    end

    it_behaves_like "a successful search", "tmp/empty"
  end

  context "with -size after -type", upstream: "find.posix/typesize.exp" do
    subject(:result) { find("tmp", "-type", "f", "-size", "0c") }

    before do
      sandbox.mkdir("tmp")
      sandbox.touch("tmp/empty")
    end

    it_behaves_like "a successful search", "tmp/empty"
  end

  context "with a symbolic link" do
    before do
      sandbox.write("target", "x" * 100)
      sandbox.symlink("target", "link")
    end

    context "with the size of the link itself" do
      subject(:result) { find("link", "-size", "6c") }

      it_behaves_like "a successful search", "link"
    end

    context "with the size of its target and -L" do
      subject(:result) { find("-L", "link", "-size", "100c") }

      it_behaves_like "a successful search", "link"
    end
  end

  context "with an invalid size", upstream: "find.posix/size-invalid.exp" do
    before { sandbox.touch("tmp") }

    {
      "w" => "find: Invalid argument `w' to -size",
      "b" => "find: Invalid argument `b' to -size",
      "c" => "find: Invalid argument `c' to -size",
      "k" => "find: Invalid argument `k' to -size",
      "m" => "find: invalid -size type `m'",
      "G" => "find: Invalid argument `G' to -size",
      "" => "find: invalid null argument to -size",
      "+-1" => "find: Invalid argument `+-1' to -size",
      "+-1G" => "find: Invalid argument `+-1G' to -size",
      "1.2k" => "find: Invalid argument `1.2k' to -size",
    }.each do |size, diagnostic|
      context "with the argument '#{size}'" do
        subject(:result) { find("tmp", "-size", size) }

        it_behaves_like "a rejected command line", diagnostic
      end
    end
  end

  context "with an unknown suffix" do
    subject(:result) { find("tmp", "-size", "1K") }

    before { sandbox.touch("tmp") }

    it_behaves_like "a rejected command line", "find: invalid -size type `K'"
  end

  context "with a count too large to represent" do
    subject(:result) { find("tmp", "-size", "18446744073709551616") }

    before { sandbox.touch("tmp") }

    it_behaves_like "a rejected command line", "find: Invalid argument `18446744073709551616' to -size"
  end

  context "without a size", upstream: "find.posix/size-missing.exp" do
    subject(:result) { find("tmp", "-size") }

    before { sandbox.touch("tmp") }

    it_behaves_like "a rejected command line", "find: missing argument to `-size'"
  end
end
