package find

import (
	"os"
	"sync"

	"golang.org/x/sys/unix"
)

// sharedFile is one FILE* from sharefile.c: every alias of the same
// (dev, ino) shares this stdio stream. It is a sink (output.go).
type sharedFile struct {
	mu     sync.Mutex
	name   string // the first name it was opened under (error messages)
	key    devIno
	seq    int // opening order: the order record.go locks shared files in
	f      *os.File
	c      *cfile
	closed bool // fclose'd by closeSharedFiles: exit() no longer flushes it
}

func (sf *sharedFile) lock()                { sf.mu.Lock() }
func (sf *sharedFile) unlock()              { sf.mu.Unlock() }
func (sf *sharedFile) write(b []byte) error { return sf.c.write(b) }
func (sf *sharedFile) flush() error         { return sf.c.flushBuf() }

type devIno struct{ dev, ino uint64 }

// shareRegistry is struct sharefile: the files opened for -fprint,
// -fprint0, -fprintf (and -fls), keyed by (dev, ino). order mirrors the
// gnulib hash table's layout, because sharefile_destroy fcloses in its
// bucket order and the first failure is fatal.
type shareRegistry struct {
	mu    sync.Mutex
	files map[devIno]*sharedFile
	order hashOrder
}

// openOutputFile is parser.c open_output_file. The exact strings
// "/dev/stdout" and "/dev/stderr" are the real streams, never reopened.
func (ps *parser) openOutputFile(path string) *Stream {
	r := ps.r
	switch path {
	case "/dev/stderr":
		return r.out.stderrStream
	case "/dev/stdout":
		return r.out.stdoutStream
	}
	sf, err := r.shared.fopen(path)
	if err != nil {
		r.fatalNontargetFileError(err, path)
	}
	return &Stream{name: path, kind: streamFile, tty: isatty(int(sf.f.Fd())), sink: sf}
}

// fopen is sharefile.c sharefile_fopen with mode "w": open (O_CLOEXEC, as
// set_cloexec_flag does), then share an earlier stream on the same file.
// A second fopen of the same file has truncated it again; nothing was
// written yet, as every output file opens while parsing.
func (reg *shareRegistry) fopen(path string) (*sharedFile, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return nil, unwrapPathError(err)
	}
	var st unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &st); err != nil {
		f.Close()
		return nil, err
	}
	key := devIno{st.Dev, st.Ino}
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if reg.files == nil {
		reg.files = map[devIno]*sharedFile{}
	}
	if sf, ok := reg.files[key]; ok {
		f.Close()
		return sf, nil
	}
	sf := &sharedFile{name: path, key: key, seq: len(reg.files), f: f, c: newCfile(f)}
	reg.files[key] = sf
	reg.order.insert(sf)
	return sf, nil
}

func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}

// fatalNontargetFileError is util.c fatal_nontarget_file_error.
func (r *Run) fatalNontargetFileError(err error, name string) {
	r.fatalf(err, "%s", r.quote(name))
}

// closeSharedFiles is sharefile_destroy: fclose every file; a failure is
// fatal, reported under the first-opened name. Files not closed yet when
// that happens are still flushed by exit() (flushFilesQuietly).
func (r *Run) closeSharedFiles() {
	reg := &r.shared
	reg.mu.Lock()
	files := reg.order.all()
	reg.mu.Unlock()
	for _, sf := range files {
		sf.lock()
		// entry_free: fclose fails only if flushing what is buffered now, or
		// close(2), fails; an earlier failure that emptied the buffer does not.
		err := sf.c.flushBuf()
		if cerr := sf.f.Close(); err == nil {
			err = unwrapPathError(cerr)
		}
		sf.closed = true
		sf.unlock()
		if err != nil {
			r.fatalNontargetFileError(err, sf.name)
		}
	}
	reg.mu.Lock()
	reg.order, reg.files = hashOrder{}, nil
	reg.mu.Unlock()
}

// hashOrder is the bucket layout of a gnulib hash.c table made by
// hash_initialize(11, NULL, …) and grown by hash_insert_if_absent, with
// sharefile.c's hasher (dev ^ ino) % n_buckets. Each bucket lists its
// chain from the head.
type hashOrder struct {
	buckets [][]*sharedFile
	used    int // n_buckets_used
}

const (
	hashGrowthThreshold float32 = 0.8
	hashGrowthFactor    float32 = 1.414
)

func (h *hashOrder) insert(sf *sharedFile) {
	if h.buckets == nil {
		h.buckets = make([][]*sharedFile, hashBucketSize(11))
	}
	if float32(h.used) > hashGrowthThreshold*float32(len(h.buckets)) {
		candidate := float32(len(h.buckets)) * hashGrowthFactor * hashGrowthThreshold
		if n := hashBucketSize(uint64(candidate)); n != len(h.buckets) {
			h.rehash(n)
		}
	}
	h.add(sf)
}

// add puts sf at the head of an empty bucket, else right after the head.
func (h *hashOrder) add(sf *sharedFile) {
	i := (sf.key.dev ^ sf.key.ino) % uint64(len(h.buckets))
	b := h.buckets[i]
	if len(b) == 0 {
		h.buckets[i] = []*sharedFile{sf}
		h.used++
		return
	}
	h.buckets[i] = append(b[:1], append([]*sharedFile{sf}, b[1:]...)...)
}

// rehash is transfer_entries: each old bucket's overflow entries move
// first, then its head.
func (h *hashOrder) rehash(n int) {
	old := h.buckets
	h.buckets, h.used = make([][]*sharedFile, n), 0
	for _, b := range old {
		if len(b) == 0 {
			continue
		}
		for _, sf := range b[1:] {
			h.add(sf)
		}
		h.add(b[0])
	}
}

// all is hash_free's order.
func (h *hashOrder) all() []*sharedFile {
	var out []*sharedFile
	for _, b := range h.buckets {
		out = append(out, b...)
	}
	return out
}

// hashBucketSize is hash.c compute_bucket_size for the default tuning.
func hashBucketSize(candidate uint64) int {
	return int(nextPrime(uint64(float32(candidate) / hashGrowthThreshold)))
}

// nextPrime is gnulib next_prime.
func nextPrime(c uint64) uint64 {
	c = max(c, 10) | 1
	for {
		prime := true
		for d := uint64(3); d*d <= c; d += 2 {
			if c%d == 0 {
				prime = false
				break
			}
		}
		if prime {
			return c
		}
		c += 2
	}
}
