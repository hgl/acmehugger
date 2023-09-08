package acme

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-acme/lego/v4/lego"
	"github.com/hgl/acmehugger/internal/clock"
	"github.com/hgl/acmehugger/internal/set"
	"github.com/hgl/acmehugger/internal/util"
	"golang.org/x/net/idna"
)

const DefaultDays = 30

type Issuer struct {
	hacct   *HandlerAccount
	certDir string
	mu      sync.Mutex
}

var issuers = make(map[string]*Issuer)

type Account struct {
	Email   string
	Server  string
	Staging bool
	KeyType KeyType
}

func (acct *Account) Clone() *Account {
	na := *acct
	return &na
}

func (acct *Account) ResolveServer() string {
	if acct.Server != "" {
		return acct.Server
	}
	if acct.Staging {
		return lego.LEDirectoryStaging
	}
	return lego.LEDirectoryProduction
}

func (acct *Account) Dir() string {
	server := acct.ResolveServer()
	dir := strings.TrimPrefix(server, "https://")
	dir = strings.NewReplacer(":", "_", "/", "_").Replace(dir)
	return filepath.Join(AccountsDir, dir)
}

func (acct *Account) CertPaths(domain string) (*CertPaths, error) {
	dir := acct.Dir()
	dir = filepath.Join(dir, "certificates")
	return newCertPaths(dir, domain)
}

func GetIssuer(acct *Account) (*Issuer, error) {
	server := acct.ResolveServer()
	issuer := issuers[server]
	if issuer != nil {
		return issuer, nil
	}

	hacct, certDir, err := loadHandlerAccount(acct)
	if err != nil {
		return nil, err
	}

	issuer = &Issuer{
		hacct:   hacct,
		certDir: certDir,
	}
	issuers[acct.Server] = issuer
	return issuer, nil
}

type CertPaths struct {
	Key           string
	KeyLive       string
	FullChain     string
	FullChainLive string
	Chain         string
	ChainLive     string
	Info          string
}

func newCertPaths(certDir string, domain string) (*CertPaths, error) {
	name, err := idna.ToASCII(strings.NewReplacer("*", "_").Replace(domain))
	if err != nil {
		return nil, err
	}
	return &CertPaths{
		Key:           filepath.Join(certDir, name+".key"),
		KeyLive:       filepath.Join(CertsDir, name+".key"),
		FullChain:     filepath.Join(certDir, name+".fullchain.crt"),
		FullChainLive: filepath.Join(CertsDir, name+".fullchain.crt"),
		Chain:         filepath.Join(certDir, name+".chain.crt"),
		ChainLive:     filepath.Join(CertsDir, name+".chain.crt"),
		Info:          filepath.Join(certDir, name+".json"),
	}, nil
}

func (paths *CertPaths) Exist() (bool, error) {
	exist, err := util.FileExist(paths.FullChain)
	if err != nil {
		return false, err
	}
	if !exist {
		return false, nil
	}
	exist, err = util.FileExist(paths.Key)
	if err != nil {
		return false, err
	}
	if !exist {
		return false, nil
	}
	exist, err = util.FileExist(paths.Chain)
	if err != nil {
		return false, err
	}
	if !exist {
		return false, nil
	}
	return true, nil
}

type IssueOptions struct {
	KeyType   KeyType
	Days      *int
	Challenge ChallengeType
	DNS       DNS
}

func (opts *IssueOptions) Clone() *IssueOptions {
	nopts := *opts
	if opts.Days != nil {
		days := *opts.Days
		nopts.Days = &days
	}
	if opts.DNS.Options != nil {
		m := make(map[string]string, len(opts.DNS.Options))
		for k, v := range opts.DNS.Options {
			m[k] = v
		}
		opts.DNS.Options = m
	}
	return &nopts
}

type ChallengeType int

const (
	ChallengeHTTP ChallengeType = iota
	ChallengeDNS
)

func ParseChallengeType(s string) (ChallengeType, error) {
	switch s {
	case "http":
		return ChallengeHTTP, nil
	case "dns":
		return ChallengeDNS, nil
	default:
		return -1, fmt.Errorf("invalid ChallengeType: %s", s)
	}
}

type DNS struct {
	Name    string
	Options map[string]string
}

type Cert struct {
	Key       []byte
	FullChain []byte
	Chain     []byte
	URL       string
}

type IssueInfo struct {
	RenewTimer clock.Timer
	Changed    bool
	CertPaths  *CertPaths
}

func (issuer *Issuer) Issue(domains []string, opts *IssueOptions) (*IssueInfo, error) {
	days := DefaultDays
	if opts.Days != nil {
		days = *opts.Days
	}
	daysDur := time.Duration(days) * 24 * time.Hour

	mainDomain := domains[0]
	paths, err := newCertPaths(issuer.certDir, mainDomain)
	if err != nil {
		return nil, err
	}

	info := &IssueInfo{CertPaths: paths}
	x509crt, err := util.ReadCert(paths.FullChain)
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	} else if err != nil {
		return nil, err
	} else if set.EqualSet(x509crt.DNSNames, domains) {
		left := clock.Until(x509crt.NotAfter.Add(-daysDur))
		if left > 0 {
			info.RenewTimer = clock.NewTimer(left)
			slog.Info("has't reached renew time, renewal skipped", "time left", left,
				"domains", domains)
			return info, nil
		}
		slog.Info("renewing", "domain", mainDomain)
	} else {
		slog.Info("issuing", "domain", mainDomain)
	}

	issuer.mu.Lock()
	defer issuer.mu.Unlock()

	crt, err := DefaultHandler().Issue(issuer.hacct, domains, opts)
	if err != nil {
		return nil, err
	}
	info.Changed = true
	slog.Info("acme certificates issued", "domains", domains)

	block, _ := pem.Decode(crt.FullChain)
	if block == nil {
		return nil, fmt.Errorf("failed to parse issued certificate as pem for domain: %s", domains[0])
	}
	x509crt, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	dur := clock.Until(x509crt.NotAfter.Add(-daysDur))
	info.RenewTimer = clock.NewTimer(dur)

	err = os.WriteFile(paths.Key, crt.Key, 0600)
	if err != nil {
		return nil, err
	}
	err = util.ForceSymlink(paths.Key, paths.KeyLive)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(paths.FullChain, crt.FullChain, 0644)
	if err != nil {
		return nil, err
	}
	err = util.ForceSymlink(paths.FullChain, paths.FullChainLive)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(paths.Chain, crt.Chain, 0644)
	if err != nil {
		return nil, err
	}
	err = util.ForceSymlink(paths.Chain, paths.ChainLive)
	if err != nil {
		return nil, err
	}
	err = util.WriteJSON(paths.Info, map[string]any{
		"certUrl": crt.URL,
	}, 0644)
	return info, err
}

func (issuer *Issuer) HandlerAccount() *HandlerAccount {
	return issuer.hacct
}
