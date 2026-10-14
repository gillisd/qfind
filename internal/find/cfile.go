package find

import (
	"bytes"
	"errors"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// cfile emulates a glibc stdio FILE open for writing (libio/fileops.c).
// It is not safe for concurrent use; the sink that owns it holds a lock.
type cfile struct {
	f       *os.File
	buf     []byte
	size    int   // _IO_file_doallocate: BUFSIZ, or st_blksize when smaller
	lineBuf bool  // a terminal: _IO_LINE_BUF
	err     error // first failed write(2): ferror()
}

// newCfile sizes the buffer as _IO_file_doallocate does when the first
// write allocates it.
func newCfile(f *os.File) *cfile {
	c := &cfile{f: f, size: 8192}
	var st unix.Stat_t
	if unix.Fstat(int(f.Fd()), &st) == nil {
		if st.Mode&unix.S_IFMT == unix.S_IFCHR && isatty(int(f.Fd())) {
			c.lineBuf = true
		}
		if st.Blksize > 0 && st.Blksize < 8192 {
			c.size = int(st.Blksize)
		}
	}
	c.buf = make([]byte, 0, c.size)
	return c
}

// write is _IO_new_file_xsputn, the output path of fwrite, fputs and
// fprintf. It returns an error when this call's write(2) failed: exactly
// when the C call returns EOF or a short count.
func (c *cfile) write(p string) error {
	toDo := len(p)
	space := c.size - len(c.buf)
	count, mustFlush := space, false
	if c.lineBuf && space >= toDo {
		if i := strings.LastIndexByte(p, '\n'); i >= 0 {
			count, mustFlush = i+1, true
		}
	}
	count = min(count, toDo)
	c.buf = append(c.buf, p[:count]...)
	p, toDo = p[count:], toDo-count
	if toDo == 0 && !mustFlush {
		return nil
	}
	if err := c.flushBuf(); err != nil {
		return err // the rest of this call's data is dropped, as in glibc
	}
	doWrite := toDo
	if c.size >= 128 {
		doWrite -= toDo % c.size // write whole blocks directly
	}
	if doWrite > 0 {
		if err := c.sysWrite([]byte(p[:doWrite])); err != nil {
			return err
		}
		p = p[doWrite:]
	}
	// _IO_default_xsputn: the remainder fits in the emptied buffer. On a
	// line-buffered stream it goes through _IO_new_file_overflow one byte
	// at a time, which writes the buffer out at every newline.
	c.buf = append(c.buf, p...)
	if c.lineBuf {
		if i := bytes.LastIndexByte(c.buf, '\n'); i >= 0 {
			err := c.sysWrite(c.buf[:i+1])
			if err != nil {
				c.buf = c.buf[:0]
				return err
			}
			c.buf = append(c.buf[:0], c.buf[i+1:]...)
		}
	}
	return nil
}

// writePair is write(s + suffix), one C call, without concatenating
// when the two fit in the buffer's free space and nothing is flushed.
func (c *cfile) writePair(s, suffix string) error {
	if suffix == "" {
		return c.write(s)
	}
	if !c.lineBuf && len(s)+len(suffix) <= c.size-len(c.buf) {
		c.buf = append(append(c.buf, s...), suffix...)
		return nil
	}
	return c.write(s + suffix)
}

// flushBuf is _IO_do_write of the buffer (fflush, _IO_OVERFLOW): the
// buffer is emptied whether or not the write succeeds.
func (c *cfile) flushBuf() error {
	if len(c.buf) == 0 {
		return nil
	}
	err := c.sysWrite(c.buf)
	c.buf = c.buf[:0]
	return err
}

// sysWrite is _IO_new_file_write: write everything or fail, setting the
// error flag. os.File.Write loops over short writes and retries EINTR; a
// broken pipe on fd 1 or 2 kills the process with SIGPIPE, as in C.
func (c *cfile) sysWrite(p []byte) error {
	_, err := c.f.Write(p)
	if err == nil {
		return nil
	}
	if c.err == nil {
		c.err = err
	}
	var pe *os.PathError
	if errors.As(err, &pe) {
		err = pe.Err
	}
	return err
}
