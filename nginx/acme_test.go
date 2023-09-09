package nginx

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hgl/acmehugger/acme"
	"github.com/hgl/acmehugger/internal/clock"
	"github.com/hgl/acmehugger/internal/clock/clocktest"
	"github.com/hgl/acmehugger/internal/util"
)

func TestACME(t *testing.T) {
	origClock := clock.Default()
	defer func() {
		clock.SetDefault(origClock)
	}()
	fakeClock := clocktest.NewClock(time.Time{})
	clock.SetDefault(fakeClock)

	acme.AccountsDir = t.TempDir()
	acme.CertsDir = t.TempDir()
	acme.ChallengeDir = "/challenge"

	names, err := filepath.Glob("testdata/process/*.in.conf")
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range names {
		base := filepath.Base(src)
		name := strings.TrimSuffix(base, ".in.conf")
		tr, err := Parse(src, filepath.Dir(src))
		if err != nil {
			t.Fatal(err)
		}
		ap, err := tr.PrepareACME()
		if err != nil {
			t.Fatal(err)
		}
		compareTree(t, tr, name+" after PrepareACME", "pre")
		acme.SetDefaultHandler(&handlerStub{
			createAccount: func(acct *acme.HandlerAccount) error {
				return nil
			},
			issue: func(acct *acme.HandlerAccount, domains []string, io *acme.IssueOptions) (*acme.Cert, error) {
				crt := &x509.Certificate{
					SerialNumber: big.NewInt(1),
					NotAfter:     clock.Now().Add(time.Duration(acme.DefaultDays+1) * 24 * time.Hour),
					DNSNames:     domains,
				}
				crtKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				if err != nil {
					t.Fatal(err)
				}
				crtData, err := x509.CreateCertificate(rand.Reader, crt, crt, &crtKey.PublicKey, crtKey)
				if err != nil {
					t.Fatal(err)
				}
				crtData = pem.EncodeToMemory(&pem.Block{Bytes: crtData})
				return &acme.Cert{
					FullChain: crtData,
				}, nil
			},
		})

		changed := ap.Process()
		i := 0
		for range changed {
			i++
			if i >= 3 {
				ap.Stop()
			}
			compareTree(t, tr, name+" after Process", "out")
			fakeClock.Tick(25 * time.Hour)
		}
	}
}

func compareTree(t *testing.T, tr *Tree, name string, target string) {
	outDir := t.TempDir()
	_, err := tr.Dump(outDir)
	if err != nil {
		t.Fatal(err)
	}
	comp := &treeComparer{
		NoopVisitor: NoopVisitor{},
		name:        name,
		outDir:      outDir,
		target:      target,
		t:           t,
	}
	tr.Accept(comp)
}

type treeComparer struct {
	NoopVisitor
	name   string
	outDir string
	target string
	t      *testing.T
}

func (comp *treeComparer) VisitConfigBegin(c *Config) error {
	got, err := util.ReadText(filepath.Join(comp.outDir, c.path))
	if err != nil {
		comp.t.Fatal(err)
	}
	got = strings.ReplaceAll(got, acme.AccountsDir, "")
	p := filepath.Dir(c.path)
	p = filepath.Join(comp.outDir, p)
	got = strings.ReplaceAll(got, p, "")
	p = strings.TrimSuffix(c.path, ".in.conf")
	if p == c.path {
		p = strings.TrimSuffix(c.path, ".inc.conf")
	}
	p = fmt.Sprintf("%s.%s.conf", p, comp.target)
	want, err := util.ReadText(p)
	if err != nil {
		comp.t.Fatal(err)
	}
	if got != want {
		comp.t.Fatalf("%s: \ngot\n%s\nwant\n%s", comp.name, got, want)
	}
	return nil
}

type procStub struct {
	reload func(*Tree) error
}

func (p procStub) Start(tr *Tree, bin string, args []string) error {
	return nil
}

func (p procStub) Reload(tr *Tree) error {
	return p.reload(tr)
}

func (p procStub) Wait() error {
	var c chan struct{}
	<-c
	return nil
}

type handlerStub struct {
	createAccount  func(*acme.HandlerAccount) error
	updateAccount  func(*acme.HandlerAccount) error
	recoverAccount func(*acme.HandlerAccount) error
	issue          func(*acme.HandlerAccount, []string, *acme.IssueOptions) (*acme.Cert, error)
}

func (h handlerStub) CreateAccount(acct *acme.HandlerAccount) error {
	return h.createAccount(acct)
}

func (h handlerStub) UpdateAccount(acct *acme.HandlerAccount) error {
	return h.updateAccount(acct)
}

func (h handlerStub) RecoverAccount(acct *acme.HandlerAccount) error {
	return h.recoverAccount(acct)
}

func (h handlerStub) Issue(acct *acme.HandlerAccount, domains []string, opts *acme.IssueOptions) (*acme.Cert, error) {
	return h.issue(acct, domains, opts)
}
