RSpec.describe "find -amin" do
  it_behaves_like "a minute-based timestamp test", "-amin", :atime
end
