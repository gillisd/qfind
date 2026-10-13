require "etc"

Dir[File.join(__dir__, "support", "**", "*.rb")].each { require it }

RSpec.configure do |config|
  config.expect_with :rspec do |expectations|
    expectations.include_chain_clauses_in_custom_matcher_descriptions = true
  end

  config.mock_with :nothing

  config.shared_context_metadata_behavior = :apply_to_host_groups
  config.filter_run_when_matching :focus
  config.example_status_persistence_file_path = "spec/examples.txt"
  config.disable_monkey_patching!
  config.warnings = true
  config.default_formatter = "doc" if config.files_to_run.one?
  config.order = :random
  Kernel.srand config.seed

  config.include FindHelpers
  config.extend OptimisationLevels
  config.include_context "with a sandbox"

  config.define_derived_metadata(:expensive) do |metadata|
    metadata[:skip] ||= "expensive: set RUN_EXPENSIVE_TESTS=yes to run" unless ENV["RUN_EXPENSIVE_TESTS"] == "yes"
  end

  config.define_derived_metadata(:root) do |metadata|
    metadata[:skip] ||= "must be run as root" unless Process.uid.zero?
  end

  config.define_derived_metadata(:unprivileged) do |metadata|
    metadata[:skip] ||= "must be run as non-root" if Process.uid.zero?
  end

  config.define_derived_metadata(:locale) do |metadata|
    metadata[:skip] ||= "requires the #{metadata[:locale]} locale" unless Locales.installed?(metadata[:locale])
  end
end
