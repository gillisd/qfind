RSpec.shared_context "with a link to a file and a broken link" do
  before do
    sandbox.mkdir("tmp")
    sandbox.touch("passwd")
    sandbox.symlink(sandbox.join("passwd").to_s, "tmp/LINK")
    sandbox.symlink("/NOSUCHFILE", "tmp/BROKEN")
  end
end
