RSpec.describe "find -ignore_readdir_race and -noignore_readdir_race" do
  context "with a subdirectory created and removed over and over meanwhile", upstream: "tests/find/readdir_race.sh" do
    subject(:results) { Array.new(1000) { find("testdir", "-ignore_readdir_race", "-ls") } }

    let(:racer) do
      vanishing = sandbox.join("testdir/foo")
      Thread.new do
        loop do
          vanishing.mkdir
          vanishing.rmdir
        end
      end
    end

    before do
      sandbox.mkdir("testdir")
      racer
    end

    after { racer.kill.join }

    it "always succeeds" do
      expect(results).to all(be_successful)
    end

    it "never diagnoses the vanished subdirectory" do
      expect(results).to all(diagnose_nothing)
    end

    it "lists the starting point every time" do
      expect(results).to all(have_attributes(lines: include(a_string_ending_with(" testdir"))))
    end
  end

  context "with a file removed by -exec before find examines it" do
    subject(:result) { find("dir", *options, "-type", "f", "-exec", "rm", "{}", ";", "-size", "-1k") }

    before do
      sandbox.mkdir("dir")
      sandbox.touch("dir/file")
    end

    context "with -ignore_readdir_race" do
      let(:options) { ["-ignore_readdir_race"] }

      it_behaves_like "a successful search"
    end

    context "without -ignore_readdir_race" do
      let(:options) { [] }

      it_behaves_like "a failed search"

      it { is_expected.to diagnose("find: 'dir/file': No such file or directory") }
    end

    context "with -noignore_readdir_race after -ignore_readdir_race" do
      let(:options) { ["-ignore_readdir_race", "-noignore_readdir_race"] }

      it_behaves_like "a failed search"

      it { is_expected.to diagnose("find: 'dir/file': No such file or directory") }
    end

    context "with -noignore_readdir_race before -ignore_readdir_race" do
      let(:options) { ["-noignore_readdir_race", "-ignore_readdir_race"] }

      it_behaves_like "a successful search"
    end

    context "with -ignore_readdir_race and -execdir later on the command line" do
      subject(:result) do
        find("dir", "-ignore_readdir_race", "-type", "f", "-exec", "rm", "{}", ";", "-size", "-1k", "-o",
             "-execdir", "true", ";")
      end

      it_behaves_like "a failed search"

      it "has its effect switched off by -execdir" do
        expect(result).to diagnose("find: 'dir/file': No such file or directory")
      end
    end
  end

  context "with a file removed by -exec before -delete gets to it" do
    subject(:result) { find("dir", *options, "-type", "f", "-exec", "rm", "{}", ";", "-delete", "-print") }

    before do
      sandbox.mkdir("dir")
      sandbox.touch("dir/file")
    end

    context "with -ignore_readdir_race" do
      let(:options) { ["-ignore_readdir_race"] }

      it_behaves_like "a successful search", "dir/file"
    end

    context "without -ignore_readdir_race" do
      let(:options) { [] }

      it_behaves_like "a failed search"

      it { is_expected.to diagnose("find: cannot delete 'dir/file': No such file or directory") }
    end
  end

  context "with a starting point that does not exist" do
    subject(:result) { find("dir", "missing", "-ignore_readdir_race") }

    before { sandbox.mkdir("dir") }

    it_behaves_like "a failed search", "dir"

    it "still diagnoses the missing starting point" do
      expect(result).to diagnose("find: 'missing': No such file or directory")
    end
  end

  ["-ignore_readdir_race", "-noignore_readdir_race"].each do |option|
    context "with #{option} and nothing vanishing" do
      subject(:result) { find("dir", option) }

      before do
        sandbox.mkdir("dir/sub")
        sandbox.touch("dir/file")
      end

      it_behaves_like "a successful search", "dir", "dir/file", "dir/sub"
    end

    context "with #{option} followed by a stray argument" do
      subject(:result) { find(".", option, "1") }

      it_behaves_like "a rejected command line", "find: paths must precede expression: `1'"
    end
  end
end
