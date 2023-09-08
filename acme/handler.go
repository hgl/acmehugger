package acme

import (
	"crypto"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns"
	"github.com/go-acme/lego/v4/providers/http/webroot"
	"github.com/go-acme/lego/v4/registration"
	"github.com/hgl/acmehugger"
	"github.com/hgl/acmehugger/internal/util"
)

var userAgent = fmt.Sprintf("acmehugger/%s lego", acmehugger.Version)

type HandlerAccount struct {
	Server string            `json:"-"`
	Email  string            `json:"email"`
	URL    string            `json:"url"`
	Key    crypto.PrivateKey `json:"-"`
}

func loadHandlerAccount(acct *Account) (*HandlerAccount, string, error) {
	dir := acct.Dir()
	certDir := filepath.Join(dir, "certificates")
	err := os.MkdirAll(certDir, 0755)
	if err != nil {
		return nil, "", err
	}
	keyPath := filepath.Join(dir, "account.key")
	key, created, err := LoadOrCreateKey(acct.KeyType, keyPath)
	if err != nil {
		return nil, "", err
	}

	server := acct.ResolveServer()
	acctPath := filepath.Join(dir, "account.json")
	if created {
		hacct := &HandlerAccount{
			Server: server,
			Email:  acct.Email,
			Key:    key,
		}
		slog.Debug("new key created, creating acme account", "account", hacct)
		err = DefaultHandler().CreateAccount(hacct)
		if err != nil {
			return nil, "", err
		}
		err = util.WriteJSON(acctPath, hacct, 0644)
		return hacct, certDir, err
	}
	var hacct *HandlerAccount
	err = util.ReadJSON(acctPath, &hacct)
	if errors.Is(err, fs.ErrNotExist) {
		hacct = &HandlerAccount{
			Server: server,
			Key:    key,
		}
		slog.Debug("key exists, but account json not found, recovering acme account", "account", hacct)
		err = DefaultHandler().RecoverAccount(hacct)
		if err != nil {
			return nil, "", err
		}
		hacct.Email = acct.Email
		err = util.WriteJSON(acctPath, hacct, 0644)
		return hacct, certDir, err
	}
	if err != nil {
		return nil, "", err
	}
	hacct.Server = server
	hacct.Key = key
	if acct.Email != hacct.Email {
		hacct.Email = acct.Email
		slog.Debug("acme email changed, updating account", "account", hacct)
		err = DefaultHandler().UpdateAccount(hacct)
		if err != nil {
			return nil, "", err
		}
	}
	return hacct, certDir, nil
}

type Handler interface {
	CreateAccount(*HandlerAccount) error
	UpdateAccount(*HandlerAccount) error
	RecoverAccount(*HandlerAccount) error
	Issue(a *HandlerAccount, domains []string, opts *IssueOptions) (*Cert, error)
}

type handler struct{}

var defaultHandler Handler
var defaultHandlerMu sync.RWMutex

func init() {
	defaultHandlerMu.Lock()
	defaultHandler = handler{}
	defaultHandlerMu.Unlock()
}

func DefaultHandler() Handler {
	defaultHandlerMu.RLock()
	defer defaultHandlerMu.RUnlock()
	return defaultHandler
}

func SetDefaultHandler(h Handler) {
	defaultHandlerMu.Lock()
	defaultHandler = h
	defaultHandlerMu.Unlock()
}

func (handler) CreateAccount(acct *HandlerAccount) error {
	cfg := lego.NewConfig(&legoAccount{
		email: acct.Email,
		key:   acct.Key,
	})
	cfg.CADirURL = acct.Server
	cfg.UserAgent = userAgent
	client, err := lego.NewClient(cfg)
	if err != nil {
		return err
	}
	opts := registration.RegisterOptions{TermsOfServiceAgreed: true}
	res, err := client.Registration.Register(opts)
	if err != nil {
		return err
	}
	acct.URL = res.URI
	return nil
}

func (handler) UpdateAccount(acct *HandlerAccount) error {
	cfg := lego.NewConfig(&legoAccount{
		email: acct.Email,
		key:   acct.Key,
		url:   acct.URL,
	})
	cfg.CADirURL = acct.Server
	cfg.UserAgent = userAgent
	client, err := lego.NewClient(cfg)
	if err != nil {
		return err
	}
	opts := registration.RegisterOptions{TermsOfServiceAgreed: true}
	_, err = client.Registration.UpdateRegistration(opts)
	return err
}

func (handler) RecoverAccount(acct *HandlerAccount) error {
	cfg := lego.NewConfig(&legoAccount{
		key: acct.Key,
	})
	cfg.CADirURL = acct.Server
	cfg.UserAgent = userAgent
	client, err := lego.NewClient(cfg)
	if err != nil {
		return err
	}
	res, err := client.Registration.ResolveAccountByKey()
	if err != nil {
		return err
	}
	acct.URL = res.URI
	return nil
}

func (handler) Issue(acct *HandlerAccount, domains []string, opts *IssueOptions) (*Cert, error) {
	cfg := lego.NewConfig(&legoAccount{
		key: acct.Key,
		url: acct.URL,
	})
	cfg.CADirURL = acct.Server
	cfg.Certificate.KeyType = legoKeyType(opts.KeyType)
	cfg.UserAgent = userAgent
	client, err := lego.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	switch opts.Challenge {
	case ChallengeHTTP:
		p, err := webroot.NewHTTPProvider(ChallengeDir)
		if err != nil {
			return nil, err
		}
		err = client.Challenge.SetHTTP01Provider(p)
		if err != nil {
			return nil, err
		}
		slog.Debug("HTTP01 issuance", "domains", domains, "issueOpts", opts, "account", acct)
	case ChallengeDNS:
		if opts.DNS.Options != nil {
			for k, v := range opts.DNS.Options {
				k = strings.ToUpper(k)
				slog.Debug("setting env for DNS01 issuance", "key", k)
				err := os.Setenv(k, v)
				if err != nil {
					return nil, err
				}
			}
		}
		provider, err := dns.NewDNSChallengeProviderByName(opts.DNS.Name)
		if err != nil {
			return nil, err
		}
		err = client.Challenge.SetDNS01Provider(provider)
		if err != nil {
			return nil, err
		}
		slog.Debug("DNS01 issuance", "domains", domains, "issueOpts", opts, "account", acct)
	}
	res, err := client.Certificate.Obtain(certificate.ObtainRequest{
		Domains: domains,
		Bundle:  true,
	})
	if err != nil {
		return nil, err
	}
	return &Cert{
		Key:       res.PrivateKey,
		FullChain: res.Certificate,
		Chain:     res.IssuerCertificate,
		URL:       res.CertURL,
	}, nil
}

type legoAccount struct {
	email string
	key   crypto.PrivateKey
	url   string
}

func (acct *legoAccount) GetEmail() string {
	return acct.email
}
func (acct *legoAccount) GetPrivateKey() crypto.PrivateKey {
	return acct.key
}
func (acct *legoAccount) GetRegistration() *registration.Resource {
	if acct.url == "" {
		return nil
	}
	return &registration.Resource{URI: acct.url}
}

func legoKeyType(kt KeyType) certcrypto.KeyType {
	switch kt {
	case KeyEC256:
		return certcrypto.EC256
	case KeyEC384:
		return certcrypto.EC384
	case KeyRSA2048:
		return certcrypto.RSA2048
	case KeyRSA3072:
		return certcrypto.RSA3072
	case KeyRSA4096:
		return certcrypto.RSA4096
	case KeyRSA8192:
		return certcrypto.RSA8192
	default:
		panic("unknown KeyType")
	}
}
