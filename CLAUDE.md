# BLACKBOX TESTING FOR FINDUTILS

## Task

Your task is to convert all black box tests for gnu findutils v4.11.0 into idiomatic rspec. **This means first and foremost using the /rspec skill**. Additionally:

1. These are blackbox tests. This means no stubs or mocks. EVERY spec should actually shell out to find/gfind
2. 1 expectation per example. No exceptions
3. Absolutely no comments allowed in the specs. Any comments in specs need to be organized into their own specs
4. Constantly reference the rspec source to ensure idiomatic use. It can be found in references/rspec

Use rspec features like shared_examples, shared_context etc to keep things DRY and behavior based, if needed


## Setup

Run `rake` to build. `find` should come out in `dist/find`, and reference source should appear in `references`
