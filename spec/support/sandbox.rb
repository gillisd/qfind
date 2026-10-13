require "fileutils"
require "socket"
require "tmpdir"

class Sandbox
  attr_reader :root

  def initialize
    @root = Pathname(Dir.mktmpdir("find-spec-")).realpath
  end

  def join(path) = root.join(path)

  def mkdir(*paths) = paths.each { join(it).mkpath }

  def touch(*paths) = paths.each { FileUtils.touch(join(it)) }

  def write(path, content) = join(path).binwrite(content)

  def symlink(target, path) = File.symlink(target, join(path))

  def link(existing, path) = File.link(join(existing), join(path))

  def chmod(mode, *paths) = File.chmod(mode, *paths.map { join(it) })

  def utime(path, atime:, mtime:) = File.utime(atime, mtime, join(path))

  def lutime(path, atime:, mtime:) = File.lutime(atime, mtime, join(path))

  def truncate(path, size) = File.truncate(join(path), size)

  def mkfifo(path) = File.mkfifo(join(path))

  def socket(path) = UNIXServer.new(join(path).to_s).close

  def exist?(path) = join(path).symlink? || join(path).exist?

  def read(path) = join(path).binread

  def remove
    make_traversable(root)
    FileUtils.rm_rf(root)
  end

  private

  def make_traversable(directory)
    directory.chmod(0o700)
    directory.children.each { make_traversable(it) if it.directory? && !it.symlink? }
  end
end
