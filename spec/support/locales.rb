module Locales
  module_function

  def installed?(name) = installed.include?(normalize(name))

  def installed
    @installed ||= begin
      IO.popen(["locale", "-a"], err: File::NULL, &:readlines).map { normalize(it.chomp) }
    rescue SystemCallError
      []
    end
  end

  def normalize(name) = name.downcase.delete("-")
end
