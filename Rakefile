require 'open3'
require 'rake/clean'
require 'rspec/core/rake_task'

RSpec::Core::RakeTask.new

FINDUTILS_VERSION = "v4.11.0"

def run(*cmd)
  warn "RUN #{cmd.join(" ")}"
  r, status = Open3.capture2e *cmd
  raise r unless status.success?
  r.strip
end

def untar(file)
  run "tar", "-xzvf", file
end

module UpstreamUtils
  extend self

  def latest
    run <<~BASH
      curl -sL https://ftp.wayne.edu/gnu/findutils/ \
        | grep -oE 'findutils-[0-9.]+\.tar\.xz' \
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

  private

  def stripped_version
    version.gsub(/^v/, "")
  end
end

namespace :upstream do
  desc "Print latest upstream version"
  task :latest_version do
    archive = UpstreamArchive.from_basename UpstreamUtils.latest.then { File.basename it }
    puts archive.version
  end
end

archive = UpstreamArchive.new FINDUTILS_VERSION

file archive.basename do
  UpstreamUtils.fetch archive
end

directory archive.extracted_name => [archive.basename] do
  untar archive.basename
end


CLEAN << archive.basename
CLOBBER << archive.extracted_name

task :default => [archive.extracted_name]
