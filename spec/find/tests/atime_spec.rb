RSpec.describe "find -atime" do
  it_behaves_like "a day-based timestamp test", "-atime", :atime
end
