module MountTable
  module_function

  def file_system_type(path)
    device = File.stat(path).then { "#{it.dev_major}:#{it.dev_minor}" }
    fields = File.foreach("/proc/self/mountinfo").map(&:split).rfind { it[2] == device }
    fields[fields.index("-") + 1]
  end
end
