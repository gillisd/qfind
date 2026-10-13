module LineListMatching
  def self.included(matcher)
    matcher.chain(:in_order) { @in_order = true }
  end

  def lines_matcher
    @lines_matcher ||= @in_order ? match(expected_as_array) : match_array(expected_as_array)
  end

  def listed_lines = RSpec::Matchers::EnglishPhrasing.list(expected_as_array)
end

RSpec::Matchers.define :have_exit_status do |expected|
  match { |result| result.exit_status == expected }
end

RSpec::Matchers.define :print_lines do |*expected|
  include LineListMatching

  match { |result| lines_matcher.matches?(result.lines) }

  description do
    listed = expected.empty? ? "nothing" : "lines#{listed_lines}"
    "print #{listed}#{chained_method_clause_sentences}"
  end

  failure_message { "expected find to #{description}\n#{lines_matcher.failure_message}" }
end

RSpec::Matchers.define :print_nothing do
  match { |result| result.stdout.empty? }

  failure_message { |result| "expected find to print nothing, but it printed #{result.stdout.inspect}" }
end

RSpec::Matchers.define :print_exactly do |expected|
  match { |result| values_match?(expected, @actual = result.stdout) }

  diffable
end

RSpec::Matchers.define :diagnose do |expected|
  match { |result| result.diagnostics.any? { values_match?(expected, it) } }

  failure_message do |result|
    "expected find to diagnose #{description_of(expected)}, but its stderr was #{result.stderr.inspect}"
  end

  failure_message_when_negated do |result|
    "expected find not to diagnose #{description_of(expected)}, but its stderr was #{result.stderr.inspect}"
  end
end

RSpec::Matchers.define :diagnose_exactly do
  include LineListMatching

  match { |result| lines_matcher.matches?(result.diagnostics) }

  description { "diagnose exactly#{listed_lines}#{chained_method_clause_sentences}" }

  failure_message { "expected find to #{description}\n#{lines_matcher.failure_message}" }
end

RSpec::Matchers.define :diagnose_nothing do
  match { |result| result.stderr.empty? }

  failure_message { |result| "expected find to diagnose nothing, but its stderr was #{result.stderr.inspect}" }
end
