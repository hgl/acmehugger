package acme

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hgl/acmehugger"
	"github.com/hgl/acmehugger/internal/clock"
	"github.com/hgl/acmehugger/internal/clock/clocktest"
	"github.com/hgl/acmehugger/internal/util"
)

func TestIssuer(t *testing.T) {
	origClock := clock.Default()
	defer func() {
		clock.SetDefault(origClock)
	}()
	clock.SetDefault(clocktest.NewClock(time.Time{}))

	dir := t.TempDir()
	acmehugger.StateDir = dir
	AccountsDir = dir + "/acme/accounts"
	CertsDir = t.TempDir()
	handler := &handlerMock{
		T:       t,
		AcctURL: "foo",
	}
	SetDefaultHandler(handler)
	handler.ExpectedCreateAccountCalls.Store(1)
	issuer, err := GetIssuer(&Account{
		Email:  "foo@example.com",
		Server: "https://example.com/dir",
	})
	if err != nil {
		t.Fatal(err)
	}
	handler.checkCalls()
	acctDir := filepath.Join(AccountsDir, "example.com_dir")
	data, err := os.ReadFile(filepath.Join(acctDir, "account.key"))
	if err != nil {
		t.Fatal(err)
	}
	key, err := x509.ParseECPrivateKey(data)
	if err != nil {
		t.Fatal(err)
	}
	if key.Params().BitSize != 256 {
		t.Errorf("expected ecdsa key size 256, got %d", key.Params().BitSize)
	}

	var hacct *HandlerAccount
	err = util.ReadJSON(filepath.Join(acctDir, "account.json"), &hacct)
	if err != nil {
		t.Fatal(err)
	}
	if want := "foo@example.com"; hacct.Email != want {
		t.Errorf("account email = %s; want %s", hacct.Email, want)
	}
	if want := "foo"; hacct.URL != want {
		t.Errorf("account url = %s; want %s", hacct.URL, want)
	}

	issuer2, err := GetIssuer(&Account{
		Email:  "foo@example.com",
		Server: "https://example.com/dir",
	})
	if err != nil {
		t.Fatal(err)
	}
	if issuer2 != issuer {
		t.Fatalf("different issuer returned for the same account")
	}
	handler.checkCalls()

	domains := []string{"a.com", "b.com"}
	opts := &IssueOptions{}
	crt := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotAfter:     clock.Now().Add(time.Duration(DefaultDays+1) * 24 * time.Hour),
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
	handler.Cert = &Cert{
		Key:       []byte{1},
		FullChain: crtData,
		Chain:     []byte{2},
		URL:       "example.com",
	}
	handler.ExpectedIssueCalls.Store(1)
	info, err := issuer.Issue(domains, opts)
	if err != nil {
		t.Fatal(err)
	}
	handler.checkCalls()
	if !info.Changed {
		t.Fatalf("cert not issued")
	}
	crtDir := filepath.Join(acctDir, "certificates")
	paths := &CertPaths{
		Key:           filepath.Join(crtDir, "a.com.key"),
		KeyLive:       filepath.Join(CertsDir, "a.com.key"),
		FullChain:     filepath.Join(crtDir, "a.com.fullchain.crt"),
		FullChainLive: filepath.Join(CertsDir, "a.com.fullchain.crt"),
		Chain:         filepath.Join(crtDir, "a.com.chain.crt"),
		ChainLive:     filepath.Join(CertsDir, "a.com.chain.crt"),
		Info:          filepath.Join(crtDir, "a.com.json"),
	}
	if *info.CertPaths != *paths {
		t.Errorf("cert paths = %#v, want %#v", info.CertPaths, paths)
	}

	data, err = os.ReadFile(paths.Key)
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{1}; !slices.Equal(data, want) {
		t.Errorf("key = %#v, want %#v", data, want)
	}
	link, err := os.Readlink(paths.KeyLive)
	if err != nil {
		t.Fatal(err)
	}
	if link != paths.Key {
		t.Errorf("live key link = %#v, want %#v", link, paths.Key)
	}
	data, err = os.ReadFile(paths.FullChain)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(data, crtData) {
		t.Errorf("key = %#v, want %#v", data, crtData)
	}
	link, err = os.Readlink(paths.FullChainLive)
	if err != nil {
		t.Fatal(err)
	}
	if link != paths.FullChain {
		t.Errorf("live full chain link = %#v, want %#v", link, paths.FullChain)
	}
	data, err = os.ReadFile(paths.Chain)
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{2}; !slices.Equal(data, want) {
		t.Errorf("key = %#v, want %#v", data, want)
	}
	link, err = os.Readlink(paths.ChainLive)
	if err != nil {
		t.Fatal(err)
	}
	if link != paths.Chain {
		t.Errorf("live chain link = %#v, want %#v", link, paths.Chain)
	}
	info, err = issuer.Issue(domains, opts)
	if err != nil {
		t.Fatal(err)
	}
	handler.checkCalls()
	if info.Changed {
		t.Fatalf("cert should not be issued")
	}
	if *info.CertPaths != *paths {
		t.Errorf("cert paths = %#v, want %#v", info.CertPaths, paths)
	}
}

type handlerMock struct {
	T                           *testing.T
	AcctURL                     string
	Cert                        *Cert
	Domains                     []string
	ExpectedCreateAccountCalls  atomic.Int32
	ExpectedUpdateAccountCalls  atomic.Int32
	ExpectedRecoverAccountCalls atomic.Int32
	ExpectedIssueCalls          atomic.Int32
}

func (h *handlerMock) CreateAccount(acct *HandlerAccount) error {
	h.ExpectedCreateAccountCalls.Add(-1)
	if h.ExpectedCreateAccountCalls.Load() < 0 {
		h.T.Fatal("calling CreateAccount unexpectedly")
	}
	acct.URL = h.AcctURL
	return nil
}

func (h *handlerMock) UpdateAccount(acct *HandlerAccount) error {
	h.ExpectedUpdateAccountCalls.Add(-1)
	if h.ExpectedUpdateAccountCalls.Load() < 0 {
		h.T.Fatal("calling UpdateAccount unexpectedly")
	}
	return nil
}

func (h *handlerMock) RecoverAccount(acct *HandlerAccount) error {
	h.ExpectedRecoverAccountCalls.Add(-1)
	if h.ExpectedRecoverAccountCalls.Load() < 0 {
		h.T.Fatal("calling RecoverAccount unexpectedly")
	}
	acct.URL = h.AcctURL
	return nil
}

func (h *handlerMock) Issue(acct *HandlerAccount, domains []string, opts *IssueOptions) (*Cert, error) {
	h.ExpectedIssueCalls.Add(-1)
	if h.ExpectedIssueCalls.Load() < 0 {
		h.T.Fatal("calling Issue unexpectedly")
	}
	if h.Domains != nil && !slices.Equal(domains, h.Domains) {
		h.T.Fatalf("issue domains: got %#v, want %#v", domains, h.Domains)
	}
	return h.Cert, nil
}
func (h *handlerMock) checkCalls() {
	if h.ExpectedCreateAccountCalls.Load() != 0 {
		h.T.Fatal("missed calling CreateAccount")
	}
	if h.ExpectedUpdateAccountCalls.Load() != 0 {
		h.T.Fatal("missed calling UpdateAccount")
	}
	if h.ExpectedRecoverAccountCalls.Load() < 0 {
		h.T.Fatal("missed calling RecoverAccount")
	}
	if h.ExpectedIssueCalls.Load() < 0 {
		h.T.Fatal("missed calling Issue")
	}
}
