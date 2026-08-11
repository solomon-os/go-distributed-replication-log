package log

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	write = []byte("hello world")
	width = uint64(len(write)) + lenWidth
)

func TestStoreAppendRead(t *testing.T) {
	t.Log("creating temp file")
	f, err := os.CreateTemp("", "store_append_test")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	t.Logf("temp file: %s", f.Name())

	t.Log("creating store")
	s, err := newStore(f)
	require.NoError(t, err)

	t.Log("testing append")
	testAppend(t, s)
	t.Log("append done")

	t.Log("testing read")
	testRead(t, s)
	t.Log("read done")

	t.Log("testing readAt")
	testReadAt(t, s)
	t.Log("readAt done")

	t.Log("re-creating store from same file")
	s, err = newStore(f)
	require.NoError(t, err)
	testRead(t, s)
	t.Log("second read done")
}

func testAppend(t *testing.T, s *store) {
	t.Helper()
	for i := uint64(1); i < 4; i++ {
		t.Logf("append iteration %d", i)
		n, pos, err := s.Append(write)
		require.NoError(t, err)
		require.Equal(t, pos+n, width*i)
	}
}

func testRead(t *testing.T, s *store) {
	t.Helper()
	var pos uint64
	for i := uint64(1); i < 4; i++ {
		t.Logf("read iteration %d at pos %d", i, pos)
		read, err := s.Read(pos)
		require.NoError(t, err)
		require.Equal(t, write, read)
		pos += width
	}
}

func testReadAt(t *testing.T, s *store) {
	t.Helper()
	for i, off := uint64(1), int64(0); i < 4; i++ {
		t.Logf("readAt iteration %d at off %d", i, off)
		b := make([]byte, lenWidth)
		t.Log("readAt: reading length header")
		n, err := s.ReadAt(b, int64(off))
		require.NoError(t, err)
		require.Equal(t, lenWidth, n)

		off += int64(n)
		size := enc.Uint64(b)
		t.Logf("readAt: payload size %d", size)
		b = make([]byte, size)
		t.Log("readAt: reading payload")
		n, err = s.ReadAt(b, off)
		require.NoError(t, err)
		require.Equal(t, write, b)
		require.Equal(t, int(size), n)
		off += int64(n)
	}
}

func TestStoreClose(t *testing.T) {
	t.Log("TestStoreClose: creating temp file")
	f, err := os.CreateTemp("", "store_close_test")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	t.Logf("TestStoreClose: temp file: %s", f.Name())

	t.Log("TestStoreClose: creating store")
	s, err := newStore(f)
	require.NoError(t, err)

	t.Log("TestStoreClose: appending")
	_, _, err = s.Append(write)
	require.NoError(t, err)

	t.Log("TestStoreClose: opening file for size check")
	f, beforeSize, err := openFile(f.Name())
	require.NoError(t, err)

	t.Log("TestStoreClose: closing store")
	err = s.Close()
	require.NoError(t, err)

	t.Log("TestStoreClose: re-opening file for size check")
	_, afterSize, err := openFile(f.Name())
	require.NoError(t, err)
	require.True(t, afterSize > beforeSize)
	t.Log("TestStoreClose: done")
}

func openFile(name string) (file *os.File, size int64, err error) {
	f, err := os.OpenFile(
		name,
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
		0644,
	)
	if err != nil {
		return nil, 0, err
	}
	fi, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}
	return f, fi.Size(), nil
}
