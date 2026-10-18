package main

// The reboot record in a TPM NV index. See watch.go for why.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/google/go-tpm/legacy/tpm2"
	"github.com/google/go-tpm/tpmutil"
)

const nvSize = 128

type record struct {
	Target  string `json:"target"`
	Version string `json:"version"`
	Tries   int    `json:"tries"`
}

type nvStore struct {
	rw    io.ReadWriteCloser
	index tpmutil.Handle
}

func openNV(index uint32) (*nvStore, error) {
	rw, err := tpm2.OpenTPM("/dev/tpmrm0")
	if err != nil {
		return nil, err
	}
	s := &nvStore{rw: rw, index: tpmutil.Handle(index)}
	if _, err := tpm2.NVReadPublic(rw, s.index); err != nil {
		// Not defined yet: an owner-hierarchy index, owner auth (empty on a
		// fresh fTPM), no dictionary-attack lockout.
		attrs := tpm2.AttrOwnerWrite | tpm2.AttrOwnerRead | tpm2.AttrNoDA
		if err := tpm2.NVDefineSpace(rw, tpm2.HandleOwner, s.index, "", "", nil, attrs, nvSize); err != nil {
			rw.Close()
			return nil, fmt.Errorf("define NV index %#x: %v", index, err)
		}
	}
	return s, nil
}

func (s *nvStore) close() { s.rw.Close() }

// read returns the zero record when the index was never written.
func (s *nvStore) read() (record, error) {
	var r record
	pub, err := tpm2.NVReadPublic(s.rw, s.index)
	if err != nil {
		return r, err
	}
	if pub.Attributes&tpm2.AttrWritten == 0 {
		return r, nil // defined, never written: reading it is an error
	}
	b, err := tpm2.NVReadEx(s.rw, s.index, tpm2.HandleOwner, "", 0)
	if err != nil {
		return r, err
	}
	b = bytes.TrimRight(b, "\x00")
	if len(b) == 0 {
		return r, nil
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return record{}, nil // unreadable: start over
	}
	return r, nil
}

func (s *nvStore) write(r record) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if len(b) > nvSize {
		return fmt.Errorf("record is %d bytes, the index holds %d", len(b), nvSize)
	}
	buf := make([]byte, nvSize)
	copy(buf, b)
	return tpm2.NVWrite(s.rw, tpm2.HandleOwner, s.index, "", buf, 0)
}
