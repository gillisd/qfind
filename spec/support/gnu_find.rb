require "io/console"
require "open3"
require "pty"
require "tmpdir"

class GnuFind
  DEFAULT_EXECUTABLE = File.expand_path("../../dist/find", __dir__)

  OPTIMISATION_LEVELS = [0, 1, 2, 3].freeze

  ENVIRONMENT = {
    "GNU_FINDUTILS_FD_LEAK_CHECK" => "1",
    "LANG" => nil,
    "LANGUAGE" => nil,
    "LC_ALL" => "C",
    "POSIXLY_CORRECT" => nil,
    "TZ" => "UTC",
  }.freeze

  def initialize(executable = ENV.fetch("FIND", DEFAULT_EXECUTABLE))
    @executable = executable
  end

  def run(*arguments, chdir:, env: {}, stdin: "", **options)
    stdout, stderr, status = Open3.capture3(
      *invocation(arguments, env),
      chdir: chdir.to_s, stdin_data: stdin, binmode: true, **options,
    )
    FindResult.new(stdout:, stderr:, status:)
  end

  def run_on_terminal(*arguments, **options)
    PTY.open { |_controller, terminal| run_redirected(*arguments, in: terminal, **options) }
  end

  def run_with_terminal_output(*arguments, **options)
    PTY.open do |controller, terminal|
      terminal.raw!
      output = Thread.new { drain(controller) }
      result = run_redirected(*arguments, out: terminal, **options)
      terminal.close
      result.with(stdout: output.value)
    end
  end

  def run_redirected(*arguments, chdir:, env: {}, **redirects)
    Dir.mktmpdir("find-capture-") do |capture|
      streams = { in: File::NULL, out: File.join(capture, "stdout"), err: File.join(capture, "stderr") }
      pid = Process.spawn(*invocation(arguments, env), chdir: chdir.to_s, **streams, **redirects)
      status = Process.wait2(pid).last
      FindResult.new(stdout: captured(streams[:out]), stderr: captured(streams[:err]), status:)
    end
  end

  private

  def invocation(arguments, env) = [ENVIRONMENT.merge(env), [@executable, "find"], *arguments]

  def captured(path) = File.file?(path) ? File.binread(path) : ""

  def drain(controller)
    output = "".b
    begin
      loop { output << controller.readpartial(4096) }
    rescue EOFError, Errno::EIO
      output
    end
  end
end
