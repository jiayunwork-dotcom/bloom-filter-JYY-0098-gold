package store

import (
	"encoding/binary"
	"hash/crc32"
	"os"
	"sync"
)

// WAL (Write-Ahead Log) provides an alternative append strategy with explicit
// flush control and batch writes. It wraps the store file with buffered I/O
// and sequence numbers for ordered replay.

// WALEntry represents a single entry in the write-ahead log.
type WALEntry struct {
	Seq     uint64
	Type    byte
	Payload []byte
}

// WAL manages buffered writes with sequence tracking.
type WAL struct {
	mu      sync.Mutex
	file    *os.File
	path    string
	seq     uint64
	buf     []WALEntry
	flushed uint64
}

// WALConfig holds configuration for the WAL.
type WALConfig struct {
	BufferSize int // number of entries to buffer before auto-flush
}

// DefaultWALConfig returns defaults: buffer 32 entries.
func DefaultWALConfig() WALConfig {
	return WALConfig{BufferSize: 32}
}

// OpenWAL opens or creates a WAL file at the given path.
func OpenWAL(path string, cfg WALConfig) (*WAL, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 32
	}
	w := &WAL{
		file: f,
		path: path,
		buf:  make([]WALEntry, 0, cfg.BufferSize),
	}
	// Replay to find last sequence number
	w.seq = w.replaySeq()
	return w, nil
}

// Append adds an entry to the buffer. Auto-flushes when buffer is full.
func (w *WAL) Append(typ byte, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.seq++
	entry := WALEntry{Seq: w.seq, Type: typ, Payload: payload}
	w.buf = append(w.buf, entry)
	if len(w.buf) >= cap(w.buf) {
		return w.flushLocked()
	}
	return nil
}

// Flush writes all buffered entries to disk.
func (w *WAL) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushLocked()
}

func (w *WAL) flushLocked() error {
	for _, entry := range w.buf {
		data := encodeWALEntry(entry)
		if _, err := w.file.Write(data); err != nil {
			return err
		}
	}
	if err := w.file.Sync(); err != nil {
		return err
	}
	w.flushed = w.seq
	w.buf = w.buf[:0]
	return nil
}

// Seq returns the current sequence number.
func (w *WAL) Seq() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.seq
}

// FlushedSeq returns the last flushed sequence number.
func (w *WAL) FlushedSeq() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushed
}

// Pending returns the number of buffered (unflushed) entries.
func (w *WAL) Pending() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.buf)
}

// Close flushes and closes the WAL file.
func (w *WAL) Close() error {
	if err := w.Flush(); err != nil {
		return err
	}
	return w.file.Close()
}

// ReadAll reads all valid entries from the WAL file from the beginning.
func ReadAll(path string) ([]WALEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []WALEntry
	pos := 0
	for {
		entry, n, ok := decodeWALEntry(data[pos:])
		if !ok {
			break
		}
		entries = append(entries, entry)
		pos += n
	}
	return entries, nil
}

// WAL entry wire format:
//
//	[8] seq (uint64 big-endian)
//	[1] type
//	[4] payload length (uint32 big-endian)
//	[N] payload
//	[4] CRC32 (IEEE, over seq+type+length+payload)
const walEntryHeader = 13 // 8 + 1 + 4
const walEntryCRC = 4

func encodeWALEntry(e WALEntry) []byte {
	total := walEntryHeader + len(e.Payload) + walEntryCRC
	buf := make([]byte, total)
	binary.BigEndian.PutUint64(buf[0:8], e.Seq)
	buf[8] = e.Type
	binary.BigEndian.PutUint32(buf[9:13], uint32(len(e.Payload)))
	copy(buf[13:], e.Payload)
	checksum := crc32.ChecksumIEEE(buf[:walEntryHeader+len(e.Payload)])
	binary.BigEndian.PutUint32(buf[total-walEntryCRC:], checksum)
	return buf
}

func decodeWALEntry(data []byte) (WALEntry, int, bool) {
	if len(data) < walEntryHeader+walEntryCRC {
		return WALEntry{}, 0, false
	}
	seq := binary.BigEndian.Uint64(data[0:8])
	typ := data[8]
	pLen := int(binary.BigEndian.Uint32(data[9:13]))
	total := walEntryHeader + pLen + walEntryCRC
	if len(data) < total {
		return WALEntry{}, 0, false
	}
	// Validate CRC
	stored := binary.BigEndian.Uint32(data[total-walEntryCRC : total])
	computed := crc32.ChecksumIEEE(data[:walEntryHeader+pLen])
	if stored != computed {
		return WALEntry{}, 0, false
	}
	payload := make([]byte, pLen)
	copy(payload, data[13:13+pLen])
	return WALEntry{Seq: seq, Type: typ, Payload: payload}, total, true
}

func (w *WAL) replaySeq() uint64 {
	data, err := os.ReadFile(w.path)
	if err != nil {
		return 0
	}
	var maxSeq uint64
	pos := 0
	for {
		entry, n, ok := decodeWALEntry(data[pos:])
		if !ok {
			break
		}
		if entry.Seq > maxSeq {
			maxSeq = entry.Seq
		}
		pos += n
	}
	return maxSeq
}
