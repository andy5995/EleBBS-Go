package sshserv

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestHostKeysCreatedAndReloaded(t *testing.T) {
	dir := t.TempDir()
	first, err := loadHostKeys(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{ssh.KeyAlgoED25519, ssh.KeyAlgoRSA, ssh.InsecureKeyAlgoDSA}
	if len(first) != len(want) {
		t.Fatalf("keys %v", first)
	}
	for i, k := range first {
		if k.PublicKey().Type() != want[i] {
			t.Fatalf("key %d is %s, want %s", i, k.PublicKey().Type(), want[i])
		}
	}
	for _, n := range []string{"eleserv_hostkey", "eleserv_hostkey_rsa", "eleserv_hostkey_dsa"} {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			t.Fatal(err)
		}
	}
	again, err := loadHostKeys(dir)
	if err != nil {
		t.Fatal(err)
	}
	for i := range first {
		if a, b := ssh.FingerprintSHA256(first[i].PublicKey()), ssh.FingerprintSHA256(again[i].PublicKey()); a != b {
			t.Fatalf("key %d changed: %s -> %s", i, a, b)
		}
	}
}

// handshake connects a client limited to cc's algorithms and returns the
// host key type it was shown.
func handshake(t *testing.T, sc *ssh.ServerConfig, cc ssh.ClientConfig) (string, error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		srv, err := ln.Accept()
		if err != nil {
			return
		}
		_ = srv.SetDeadline(time.Now().Add(30 * time.Second))
		conn, chans, reqs, err := ssh.NewServerConn(srv, sc)
		if err != nil {
			srv.Close()
			return
		}
		go ssh.DiscardRequests(reqs)
		go func() {
			for c := range chans {
				_ = c.Reject(ssh.Prohibited, "test")
			}
		}()
		_ = conn.Wait()
	}()
	cli, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_ = cli.SetDeadline(time.Now().Add(30 * time.Second))
	var got string
	cc.User = "test"
	cc.HostKeyCallback = func(_ string, _ net.Addr, k ssh.PublicKey) error {
		got = k.Type()
		return nil
	}
	c, _, _, err := ssh.NewClientConn(cli, "bbs", &cc)
	if err != nil {
		cli.Close()
		return "", err
	}
	c.Close()
	return got, nil
}

func testServer(t *testing.T) *ssh.ServerConfig {
	t.Helper()
	signers, err := loadHostKeys(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sc := &ssh.ServerConfig{Config: legacyAlgorithms(), NoClientAuth: true}
	for _, s := range signers {
		sc.AddHostKey(s)
	}
	return sc
}

func TestHostKeyAlgorithms(t *testing.T) {
	sc := testServer(t)
	for algo, want := range map[string]string{
		ssh.KeyAlgoRSA:         ssh.KeyAlgoRSA,
		ssh.KeyAlgoRSASHA256:   ssh.KeyAlgoRSA,
		ssh.KeyAlgoRSASHA512:   ssh.KeyAlgoRSA,
		ssh.KeyAlgoED25519:     ssh.KeyAlgoED25519,
		ssh.InsecureKeyAlgoDSA: ssh.InsecureKeyAlgoDSA,
	} {
		got, err := handshake(t, sc, ssh.ClientConfig{HostKeyAlgorithms: []string{algo}})
		if err != nil {
			t.Fatalf("%s: %v", algo, err)
		}
		if got != want {
			t.Fatalf("%s: host key %s", algo, got)
		}
	}
}

func TestOldClientAlgorithms(t *testing.T) {
	sc := testServer(t)
	for _, c := range []struct{ kex, cipher, hostKey string }{
		{ssh.InsecureKeyExchangeDH1SHA1, ssh.InsecureCipherAES128CBC, ssh.KeyAlgoRSA},
		{ssh.InsecureKeyExchangeDH14SHA1, ssh.InsecureCipherTripleDESCBC, ssh.InsecureKeyAlgoDSA},
		{ssh.InsecureKeyExchangeDHGEXSHA1, ssh.CipherAES128CTR, ssh.KeyAlgoRSA},
	} {
		cc := ssh.ClientConfig{
			Config: ssh.Config{
				KeyExchanges: []string{c.kex},
				Ciphers:      []string{c.cipher},
				MACs:         []string{ssh.HMACSHA1},
			},
			HostKeyAlgorithms: []string{c.hostKey},
		}
		if _, err := handshake(t, sc, cc); err != nil {
			t.Fatalf("%+v: %v", c, err)
		}
	}
}
