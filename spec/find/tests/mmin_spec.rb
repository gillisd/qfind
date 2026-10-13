RSpec.describe "find -mmin" do
  it_behaves_like "a minute-based timestamp test", "-mmin", :mtime
end
