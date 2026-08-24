package log

import (
	"io"
	"os"

	"github.com/tysonmote/gommap"
)

var (
	offWidth uint64 = 4
	posWidth uint64 = 8
	entWidth        = offWidth + posWidth
)

type index struct {
	file *os.File
	mmap gommap.MMap
	size uint64
}

func newIndex(f *os.File, c Config) (*index, error) {
	idx := &index{
		file: f,
	}
	fi, err := os.Stat(f.Name())
	if err != nil {
		return nil, err
	}
	preSize := uint64(fi.Size())

	if err = os.Truncate(
		f.Name(), int64(c.Segment.MaxIndexBytes),
	); err != nil {
		return nil, err
	}

	if idx.mmap, err = gommap.Map(
		idx.file.Fd(),
		gommap.PROT_READ|gommap.PROT_WRITE,
		gommap.MAP_SHARED,
	); err != nil {
		return nil, err
	}

	// A file that was already at capacity before this truncate can't be
	// trusted at face value: it may be the padded remainder of an index
	// that was never cleanly closed (e.g. the process was killed), in
	// which case the on-disk size no longer reflects how many entries
	// were actually written. Recover the real size by walking entries
	// instead of trusting the raw file size in that case.
	if preSize < c.Segment.MaxIndexBytes {
		idx.size = preSize
	} else {
		idx.size = idx.recoverSize()
	}

	return idx, nil
}

// recoverSize walks the memory-mapped entries from the start and returns
// the size in bytes of the longest valid prefix, where "valid" means each
// entry's relative offset is exactly one more than the last. Entries past
// an unclean shutdown are indistinguishable from zero-valued padding, so
// this stops at the first entry that breaks the expected sequence.
func (i *index) recoverSize() uint64 {
	var size uint64
	for expected := uint32(0); size+entWidth <= uint64(len(i.mmap)); expected++ {
		off := enc.Uint32(i.mmap[size : size+offWidth])
		if off != expected {
			break
		}
		size += entWidth
	}
	return size
}

func (i *index) Name() string {
	return i.file.Name()
}

func (i *index) Read(in int64) (out uint32, pos uint64, err error) {
	if i.size == 0 {
		return 0, 0, io.EOF
	}

	if in == -1 {
		out = uint32((i.size / entWidth) - 1)
	} else {
		out = uint32(in)
	}
	pos = uint64(out) * entWidth
	if i.size < pos+entWidth {
		return 0, 0, io.EOF
	}
	out = enc.Uint32(i.mmap[pos : pos+offWidth])
	pos = enc.Uint64(i.mmap[pos+offWidth : pos+entWidth])
	return out, pos, nil
}

func (i *index) Write(off uint32, pos uint64) error {
	if uint64(len(i.mmap)) < i.size+entWidth {
		return io.EOF
	}
	enc.PutUint32(i.mmap[i.size:i.size+offWidth], off)
	enc.PutUint64(i.mmap[i.size+offWidth:i.size+entWidth], pos)
	i.size += entWidth
	return nil
}

func (idx *index) Close() error {
	if err := idx.mmap.Sync(gommap.MS_SYNC); err != nil {
		return err
	}
	if err := idx.file.Sync(); err != nil {
		return err
	}
	if err := idx.file.Truncate(int64(idx.size)); err != nil {
		return err
	}
	return idx.file.Close()
}
