require "open3"
require "rake/clean"
require "rspec/core/rake_task"
require "rubocop/rake_task"

RSpec::Core::RakeTask.new
RuboCop::RakeTask.new

FINDUTILS_VERSION = "v4.11.0".freeze

def run(*cmd, stdin_data: nil)
  warn "RUN #{cmd.join(" ")}"
  r, status = Open3.capture2e(*cmd, stdin_data:)
  raise r unless status.success?

  r.strip
end

def untar(file)
  run "xz -dc | tar -xv", stdin_data: File.binread(file)
end

class Reference
  attr_reader :repo

  def initialize(repo)
    @repo = repo
  end

  def dir
    Pathname("references")
  end

  def path
    dir / Pathname(repo).basename
  end
end

module ReferenceUtils
  module_function

  def dir; end

  def fetch(reference)
    run "gh", "repo", "clone", reference.repo.to_s, reference.path.to_s, "--", "--depth=1"
  end
end

module UpstreamUtils
  module_function

  def latest
    run <<~BASH
      curl -sL https://ftp.wayne.edu/gnu/findutils/ \
        | grep -oE 'findutils-[0-9.]+.tar.xz' \
        | sort -uV \
        | tail -1
    BASH
  end

  def fetch(archive)
    run <<~BASH
      curl -o #{archive.basename} -sL https://ftp.wayne.edu/gnu/findutils/#{archive.basename}
    BASH
  end
end

class UpstreamArchive
  attr_reader :version

  def self.from_basename(basename)
    basename
      .to_s
      .reverse
      .split(/\.(?=[0-9])/, 2)
      .last
      .reverse
      .split("-") => [name, version]
    new "v#{version}"
  end

  def initialize(version)
    @version = version
  end

  def extracted_name
    "findutils-#{stripped_version}"
  end

  def basename
    "#{extracted_name}.tar.xz"
  end

  def binname
    Pathname(extracted_name) / "find/find"
  end

  def dist_dir
    Pathname("dist")
  end

  def dist_binname
    dist_dir / "find"
  end

  private

  def stripped_version
    version.gsub(/^v/, "")
  end
end

namespace :upstream do
  archive = UpstreamArchive.new FINDUTILS_VERSION

  file archive.basename do
    UpstreamUtils.fetch archive
  end

  directory archive.extracted_name => [archive.basename] do
    untar archive.basename
  end

  directory archive.dist_dir do
    mkdir_p "dist"
  end

  sources = FileList[archive.extracted_name, "#{archive.extracted_name}/find/*", "#{archive.extracted_name}/find/**/*"]
  sources.exclude archive.binname

  file archive.binname => sources do
    chdir archive.extracted_name do
      run "./configure"
      run "make"
    end
  end

  file archive.dist_binname => [archive.dist_dir, archive.binname] do
    ln archive.binname, archive.dist_binname
  end

  CLEAN << archive.basename
  CLOBBER.include FileList[archive.extracted_name, archive.dist_dir]

  desc "Print latest upstream version"
  task :latest_version do
    archive = UpstreamArchive.from_basename(UpstreamUtils.latest.then { File.basename it })
    puts archive.version
  end

  task build: archive.dist_binname
end

namespace :references do
  repos = ["rspec/rspec"]
  references = repos.map { Reference.new it }
  dirs = FileList["references"]
  references.each do |reference|
    directory reference.dir do
      mkdir_p reference.dir
    end

    directory reference.path => [reference.dir] do
      ReferenceUtils.fetch reference
    end

    dirs << reference.path
  end

  CLOBBER.include dirs

  desc "Setup references"
  task setup: dirs
end

desc "setup the project"
multitask setup: ["upstream:build", "references:setup"]

task default: [:setup, :spec, :rubocop]
