module FindHelpers
  def find(*arguments, **options)
    gnu_find.run(*optimisation_flags, *arguments, chdir: sandbox.root, **options)
  end

  def find_on_terminal(*arguments, **options)
    gnu_find.run_on_terminal(*optimisation_flags, *arguments, chdir: sandbox.root, **options)
  end

  def find_with_terminal_output(*arguments, **options)
    gnu_find.run_with_terminal_output(*optimisation_flags, *arguments, chdir: sandbox.root, **options)
  end

  def find_with_stdin_from(path, *arguments, **options)
    find_redirected(*arguments, in: sandbox.join(path).to_s, **options)
  end

  def find_with_stdout_to(path, *arguments, **options)
    find_redirected(*arguments, out: sandbox.join(path).to_s, **options)
  end

  private

  def gnu_find = GnuFind.new

  def optimisation_flags = Array(optimisation_level).map { "-O#{it}" }

  def find_redirected(*arguments, **options)
    gnu_find.run_redirected(*optimisation_flags, *arguments, chdir: sandbox.root, **options)
  end
end
