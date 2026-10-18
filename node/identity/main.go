// node-identity gives a stateless node a stable SSH host key that cannot
// leave its TPM.
//
//	node-identity agent [-dir /run/node-identity] [-ephemeral]
//	node-identity pubkey
//
// The key is an ECC P-256 signing key, created as a primary key in the TPM
// owner hierarchy from a fixed template and kept at persistent handle
// 0x81008001. A primary key is derived from the hierarchy seed, so the same
// TPM always gives the same key, and the private half never leaves the TPM.
// The OS holds no key material and no state.
//
// agent serves that key over the ssh-agent protocol on <dir>/agent.sock and
// writes its public half to <dir>/ssh_host_ecdsa_key.pub. sshd uses both
// (HostKey plus HostKeyAgent). The public file appears only once the socket
// is listening, so init waits for it.
//
// With no TPM, or when the key cannot be created or loaded, agent prints one
// FATAL line on stderr and in the kernel log and exits 1. Init then does not
// start sshd. -ephemeral (node_identity=ephemeral on the kernel command line)
// uses a key in RAM instead, with a loud WARNING. It is for tests and dev only.
//
// TPM commands are go-tpm's (legacy/tpm2). SSH encoding and the agent
// protocol are golang.org/x/crypto/ssh. Nothing here marshals by hand.
package main

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/go-tpm/legacy/tpm2"
	"github.com/google/go-tpm/tpmutil"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const (
	tpmPath = "/dev/tpmrm0"
	// Owner-hierarchy persistent range, clear of the SRK (0x81000001) and
	// the EK range (0x8101xxxx).
	keyHandle = tpmutil.Handle(0x81008001)
)

var template = tpm2.Public{
	Type:    tpm2.AlgECC,
	NameAlg: tpm2.AlgSHA256,
	Attributes: tpm2.FlagSign | tpm2.FlagFixedTPM | tpm2.FlagFixedParent |
		tpm2.FlagSensitiveDataOrigin | tpm2.FlagUserWithAuth,
	ECCParameters: &tpm2.ECCParams{
		Sign:    &tpm2.SigScheme{Alg: tpm2.AlgECDSA, Hash: tpm2.AlgSHA256},
		CurveID: tpm2.CurveNISTP256,
	},
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	fs := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	dir := fs.String("dir", "/run/node-identity", "socket and public key directory")
	ephemeral := fs.Bool("ephemeral", false, "key in RAM, for tests and dev only")
	fs.Parse(os.Args[2:])
	switch os.Args[1] {
	case "agent":
		runAgent(*dir, *ephemeral)
	case "pubkey":
		s, err := tpmSigner()
		if err != nil {
			fatal(err)
		}
		pub, err := ssh.NewPublicKey(s.Public())
		if err != nil {
			fatal(err)
		}
		der, err := x509.MarshalPKIXPublicKey(s.Public())
		if err != nil {
			fatal(err)
		}
		os.Stdout.Write(ssh.MarshalAuthorizedKey(pub))
		pem.Encode(os.Stdout, &pem.Block{Type: "PUBLIC KEY", Bytes: der})
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: node-identity agent [-dir d] [-ephemeral] | node-identity pubkey")
	os.Exit(2)
}

func runAgent(dir string, ephemeral bool) {
	var signer crypto.Signer
	source := "tpm"
	if ephemeral {
		k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			fatal(err)
		}
		signer, source = k, "ephemeral"
		logk(4, "identity: WARNING node_identity=ephemeral: the SSH host key is in RAM and changes every boot. "+
			"For tests and dev only. Remove node_identity=ephemeral to use the TPM.")
	} else {
		s, err := tpmSigner()
		if err != nil {
			fatal(err)
		}
		signer = s
	}
	sshSigner, err := ssh.NewSignerFromSigner(signer)
	if err != nil {
		fatal(err)
	}

	must(os.MkdirAll(dir, 0o700))
	sock := filepath.Join(dir, "agent.sock")
	os.Remove(sock)
	l, err := net.Listen("unix", sock)
	must(err)
	must(os.Chmod(sock, 0o600))

	pub := ssh.MarshalAuthorizedKey(sshSigner.PublicKey())
	writeAtomic(filepath.Join(dir, "source"), []byte(source+"\n"))
	writeAtomic(filepath.Join(dir, "ssh_host_ecdsa_key.pub"), pub)
	logk(6, fmt.Sprintf("identity: host key from %s: %s", describe(source), ssh.FingerprintSHA256(sshSigner.PublicKey())))

	a := &hostAgent{signer: sshSigner}
	for {
		c, err := l.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer c.Close()
			agent.ServeAgent(a, c)
		}()
	}
}

func describe(source string) string {
	if source == "tpm" {
		return fmt.Sprintf("TPM handle %#x", uint32(keyHandle))
	}
	return "RAM (ephemeral)"
}

// tpmSigner opens the TPM and loads the host key, creating it on first use.
func tpmSigner() (*tpmKey, error) {
	if _, err := os.Stat(tpmPath); err != nil {
		hint := "enable fTPM in BIOS (Security > AMD fTPM)"
		if _, err := os.Stat("/dev/tpm0"); err == nil {
			hint = "/dev/tpm0 exists but has no resource manager: the TPM may be 1.2, or the kernel lacks TPM 2.0 support"
		}
		return nil, fmt.Errorf("no TPM 2.0 (%s missing): %s", tpmPath, hint)
	}
	rw, err := tpm2.OpenTPM(tpmPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %v: check that no other process holds the TPM", tpmPath, err)
	}
	pub, err := loadOrCreate(rw)
	if err != nil {
		rw.Close()
		return nil, err
	}
	return &tpmKey{rw: rw, pub: pub}, nil
}

func loadOrCreate(rw io.ReadWriter) (*ecdsa.PublicKey, error) {
	p, _, _, readErr := tpm2.ReadPublic(rw, keyHandle)
	if readErr == nil {
		if p.Type != tpm2.AlgECC || p.ECCParameters == nil || p.ECCParameters.CurveID != tpm2.CurveNISTP256 ||
			p.Attributes&tpm2.FlagSign == 0 || p.Attributes&tpm2.FlagFixedTPM == 0 {
			return nil, fmt.Errorf("TPM handle %#x holds another object, not this node's P-256 signing key: "+
				"evict it (tpm2_evictcontrol -c %#x) or clear the TPM", uint32(keyHandle), uint32(keyHandle))
		}
		k, err := p.Key()
		if err != nil {
			return nil, fmt.Errorf("TPM handle %#x: %v", uint32(keyHandle), err)
		}
		return k.(*ecdsa.PublicKey), nil
	}
	h, _, err := tpm2.CreatePrimary(rw, tpm2.HandleOwner, tpm2.PCRSelection{}, "", "", template)
	if err != nil {
		return nil, fmt.Errorf("TPM key at %#x could not be loaded (%v) or created (%v): "+
			"the owner hierarchy may have a password, or the fTPM is in failure mode; clear the TPM in BIOS",
			uint32(keyHandle), readErr, err)
	}
	defer tpm2.FlushContext(rw, h)
	if err := tpm2.EvictControl(rw, "", tpm2.HandleOwner, h, keyHandle); err != nil {
		return nil, fmt.Errorf("TPM key created but not persisted at %#x: %v: TPM NV may be full", uint32(keyHandle), err)
	}
	logk(6, fmt.Sprintf("identity: created the host key at TPM handle %#x", uint32(keyHandle)))
	p, _, _, err = tpm2.ReadPublic(rw, keyHandle)
	if err != nil {
		return nil, fmt.Errorf("TPM handle %#x after create: %v", uint32(keyHandle), err)
	}
	k, err := p.Key()
	if err != nil {
		return nil, err
	}
	return k.(*ecdsa.PublicKey), nil
}

// tpmKey is a crypto.Signer whose private key stays in the TPM.
type tpmKey struct {
	mu  sync.Mutex
	rw  io.ReadWriteCloser
	pub *ecdsa.PublicKey
}

func (k *tpmKey) Public() crypto.PublicKey { return k.pub }

func (k *tpmKey) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	if opts.HashFunc() != crypto.SHA256 {
		return nil, fmt.Errorf("P-256 host key signs SHA-256 digests only, not %v", opts.HashFunc())
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	sig, err := tpm2.Sign(k.rw, keyHandle, "", digest, nil,
		&tpm2.SigScheme{Alg: tpm2.AlgECDSA, Hash: tpm2.AlgSHA256})
	if err != nil {
		return nil, err
	}
	if sig.ECC == nil {
		return nil, errors.New("TPM returned no ECDSA signature")
	}
	return asn1.Marshal(struct{ R, S *big.Int }{sig.ECC.R, sig.ECC.S})
}

// hostAgent serves one key and refuses every change.
type hostAgent struct{ signer ssh.Signer }

var errReadOnly = errors.New("node-identity: read-only agent")

func (a *hostAgent) List() ([]*agent.Key, error) {
	p := a.signer.PublicKey()
	return []*agent.Key{{Format: p.Type(), Blob: p.Marshal(), Comment: "node-identity"}}, nil
}

func (a *hostAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	return a.SignWithFlags(key, data, 0)
}

func (a *hostAgent) SignWithFlags(key ssh.PublicKey, data []byte, _ agent.SignatureFlags) (*ssh.Signature, error) {
	if !bytes.Equal(key.Marshal(), a.signer.PublicKey().Marshal()) {
		return nil, errors.New("node-identity: unknown key")
	}
	return a.signer.Sign(rand.Reader, data)
}

func (a *hostAgent) Signers() ([]ssh.Signer, error) { return []ssh.Signer{a.signer}, nil }
func (a *hostAgent) Add(agent.AddedKey) error       { return errReadOnly }
func (a *hostAgent) Remove(ssh.PublicKey) error     { return errReadOnly }
func (a *hostAgent) RemoveAll() error               { return errReadOnly }
func (a *hostAgent) Lock([]byte) error              { return errReadOnly }
func (a *hostAgent) Unlock([]byte) error            { return errReadOnly }
func (a *hostAgent) Extension(string, []byte) ([]byte, error) {
	return nil, agent.ErrExtensionUnsupported
}

func writeAtomic(path string, b []byte) {
	tmp := path + ".tmp"
	must(os.WriteFile(tmp, b, 0o600)) // sshd refuses a group- or world-readable HostKey path
	must(os.Rename(tmp, path))
}

// logk writes one line to the kernel log at a syslog level: dmesg, and the
// console for level 2 and below (loglevel=3). The node-agent dashboard shows
// FATAL and WARNING lines. Stderr (the console) gets every line only with
// node_verbose=1, the classic scrolling console.
func logk(level int, msg string) {
	if os.Getenv("node_verbose") == "1" {
		fmt.Fprintln(os.Stderr, msg)
	}
	if f, err := os.OpenFile("/dev/kmsg", os.O_WRONLY, 0); err == nil {
		fmt.Fprintf(f, "<%d>%s\n", level, msg)
		f.Close()
	}
}

func fatal(err error) {
	logk(2, "identity: FATAL "+err.Error()+". sshd not started")
	os.Exit(1)
}

func must(err error) {
	if err != nil {
		fatal(err)
	}
}
