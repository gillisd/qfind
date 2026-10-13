module OptimisationLevels
  def at_every_optimisation_level(&)
    GnuFind::OPTIMISATION_LEVELS.each do |level|
      context "with -O#{level}" do
        let(:optimisation_level) { level }

        class_exec(&)
      end
    end
  end
end
