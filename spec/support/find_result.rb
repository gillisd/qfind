FindResult = Data.define(:stdout, :stderr, :status) do
  def exit_status = status.exitstatus

  def successful? = status.success?

  def lines = split_lines(stdout)

  def records = stdout.split(0.chr)

  def diagnostics = split_lines(stderr)

  private

  def split_lines(text) = text.each_line.map { it.delete_suffix("\n") }
end
