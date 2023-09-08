package nginx

import (
	"log/slog"
	"slices"
	"time"

	"github.com/hgl/acmehugger/acme"
	"github.com/hgl/acmehugger/internal/clock"
	"github.com/hgl/acmehugger/internal/set"
	"github.com/hgl/acmehugger/internal/stack"
)

func (tr *Tree) PrepareACME() (*ACMEProcessor, error) {
	extractor := &acmeExtractor{
		NoopVisitor: NoopVisitor{},
	}
	err := tr.Accept(extractor)
	if err != nil {
		return nil, err
	}
	if extractor.hasHTTP01 {
		if extractor.httpBlock == nil {
			blk := NewBlockDirective("http", []string{})
			tr.conf.Children = append(tr.conf.Children, blk)
			extractor.httpBlock = blk
		}
		if len(extractor.httpServerBlocks) == 0 {
			blk := NewBlockDirective("server", []string{})
			extractor.httpBlock.Children = append(extractor.httpBlock.Children, blk)
			extractor.httpServerBlocks = []*serverBlock{&serverBlock{
				http: true,
				dire: blk,
			}}
		}
		for _, s := range extractor.httpServerBlocks {
			s.dire.Children = append(s.dire.Children,
				NewDirective("location", []string{`/.well-known/acme-challenge/`},
					NewDirective("root", []string{acme.ChallengeDir}),
				),
			)
		}
	}
	for _, s := range extractor.httpsServerBlocks {
		paths, err := s.acct.CertPaths(s.domains[0])
		if err != nil {
			return nil, err
		}
		exist, err := paths.Exist()
		if err != nil {
			return nil, err
		}
		if !exist {
			if !s.http {
				deferred := &DeferredDirective{s.dire}
				s.dire.ReplaceWith(deferred)
				s.deferredBlk = deferred
			}
			continue
		}
		s.replaceDeferDirectives()
		s.ensureSSLDirectives(paths)
	}
	slog.Debug("server blocks collected for acme issuing",
		"hasHTTP01", extractor.hasHTTP01,
		"httpServersLen", len(extractor.httpServerBlocks),
		"httpsServersLen", len(extractor.httpsServerBlocks),
	)
	return &ACMEProcessor{
		tr:        tr,
		extractor: extractor,
		stopped:   make(chan struct{}),
	}, nil
}

type ACMEProcessor struct {
	tr        *Tree
	extractor *acmeExtractor
	stopped   chan struct{}
}

type ACMEChangeInfo struct {
	Block       *BlockDirective
	TreeChanged bool
	Server      string
	Email       string
	Domains     []string
}

func (p *ACMEProcessor) Process() <-chan *ACMEChangeInfo {
	ch := make(chan *ACMEChangeInfo)
	for _, s := range p.extractor.httpsServerBlocks {
		go p.processServerBlock(s, ch)
	}
	for _, a := range p.extractor.acmeBlocks {
		go p.processACMEBlock(a, ch)
	}
	return ch
}

func (p *ACMEProcessor) processServerBlock(s *serverBlock, ch chan<- *ACMEChangeInfo) {
	var issuer *acme.Issuer
	var err error
	for {
		issuer, err = acme.GetIssuer(s.acct)
		if err != nil {
			slog.Error("failed to prepare issuing, retry in an hour", "error", err)
			t := clock.NewTimer(time.Hour)
			select {
			case <-p.stopped:
				t.Stop()
				return
			case <-t.C():
				continue
			}
		}
		break
	}

	firstRun := true
	for {
		info, err := issuer.Issue(s.domains, s.issueOpts)
		if err != nil {
			slog.Error("failed to issue, retry in an hour", "error", err)
			t := clock.NewTimer(time.Hour)
			select {
			case <-p.stopped:
				t.Stop()
				return
			case <-t.C():
				continue
			}
		}
		hacct := issuer.HandlerAccount()
		if firstRun {
			firstRun = false
			if info.Changed {
				p.tr.Change(func() {
					s.replaceDeferDirectives()
					s.ensureSSLDirectives(info.CertPaths)
				})
				go func() {
					ch <- &ACMEChangeInfo{
						Block:       s.dire,
						TreeChanged: true,
						Server:      hacct.Server,
						Email:       hacct.Email,
						Domains:     s.domains,
					}
				}()
			}
		} else if info.Changed {
			go func() {
				ch <- &ACMEChangeInfo{
					Block:       s.dire,
					TreeChanged: false,
					Server:      hacct.Server,
					Email:       hacct.Email,
					Domains:     s.domains,
				}
			}()
		}

		select {
		case <-p.stopped:
			info.RenewTimer.Stop()
			return
		case <-info.RenewTimer.C():
			continue
		}
	}
}

func (p *ACMEProcessor) processACMEBlock(a *acmeBlock, ch chan<- *ACMEChangeInfo) {
	var issuer *acme.Issuer
	var err error
	for {
		issuer, err = acme.GetIssuer(a.acct)
		if err != nil {
			slog.Error("failed to prepare issuing, retry in an hour", "error", err)
			t := clock.NewTimer(time.Hour)
			select {
			case <-p.stopped:
				t.Stop()
				return
			case <-t.C():
				continue
			}
		}
		break
	}
	for {
		info, err := issuer.Issue(a.domains, a.issueOpts)
		if err != nil {
			slog.Error("failed to issue, retry in an hour", "error", err)
			t := clock.NewTimer(time.Hour)
			select {
			case <-p.stopped:
				t.Stop()
				return
			case <-t.C():
				continue
			}
		}

		if info.Changed {
			hacct := issuer.HandlerAccount()
			go func() {
				ch <- &ACMEChangeInfo{
					Block:       a.dire,
					TreeChanged: false,
					Server:      hacct.Server,
					Email:       hacct.Email,
					Domains:     a.domains,
				}
			}()
		}

		select {
		case <-p.stopped:
			info.RenewTimer.Stop()
			return
		case <-info.RenewTimer.C():
			continue
		}
	}
}

func (p *ACMEProcessor) Stop() {
	close(p.stopped)
}

type serverBlock struct {
	http                   bool
	https                  bool
	domains                []string
	domainsFromACMEDomains bool
	acct                   *acme.Account
	issueOpts              *acme.IssueOptions
	deferredBlk            *DeferredDirective
	dire                   *BlockDirective
	sslCertificate         *SimpleDirective
	sslCertificateKey      *SimpleDirective
	sslTrustedCertificate  *SimpleDirective
}

func (s *serverBlock) ensureSSLDirectives(paths *acme.CertPaths) {
	if s.sslCertificate == nil {
		s.sslCertificate = NewDirective("ssl_certificate", []string{paths.FullChain}).(*SimpleDirective)
		s.dire.Children = append(s.dire.Children, s.sslCertificate)
	} else {
		s.sslCertificate.SetArg(0, paths.FullChain)
	}
	if s.sslCertificateKey == nil {
		s.sslCertificateKey = NewDirective("ssl_certificate_key", []string{paths.Key}).(*SimpleDirective)
		s.dire.Children = append(s.dire.Children, s.sslCertificateKey)
	} else {
		s.sslCertificateKey.SetArg(0, paths.Key)
	}
	if s.sslTrustedCertificate == nil {
		s.sslTrustedCertificate = NewDirective("ssl_trusted_certificate", []string{paths.Chain}).(*SimpleDirective)
		s.dire.Children = append(s.dire.Children, s.sslTrustedCertificate)
	} else {
		s.sslTrustedCertificate.SetArg(0, paths.Chain)
	}
}

func (s *serverBlock) replaceDeferDirectives() {
	if s.deferredBlk != nil {
		s.deferredBlk.Undefer()
	}
	for i, dire := range s.dire.Children {
		d, ok := dire.(*DeferredDirective)
		if !ok {
			continue
		}
		s.dire.Children[i] = d.Directive
	}
}

type acmeBlock struct {
	domains   []string
	acct      *acme.Account
	issueOpts *acme.IssueOptions
	dire      *BlockDirective
}

type acmeExtractor struct {
	NoopVisitor
	tr                *Tree
	blockDepth        int
	visitedDires      set.Set[string]
	acctStack         stack.Stack[*acme.Account]
	issueOptsStack    stack.Stack[*acme.IssueOptions]
	serverBlock       *serverBlock
	httpServerBlocks  []*serverBlock
	httpsServerBlocks []*serverBlock
	hasHTTP01         bool
	httpBlock         *BlockDirective
	acmeBlock         *acmeBlock
	acmeBlocks        []*acmeBlock
}

func (f *acmeExtractor) VisitTreeBegin(tr *Tree) error {
	f.tr = tr
	f.visitedDires = make(set.Set[string])
	var acct acme.Account
	f.acctStack = stack.Stack[*acme.Account]{&acct}
	var opts acme.IssueOptions
	f.issueOptsStack = stack.Stack[*acme.IssueOptions]{&opts}
	return nil
}

func (f *acmeExtractor) VisitBlockBegin(d *BlockDirective) error {
	f.blockDepth++
	if f.blockDepth >= 3 {
		return SkipLevel
	}
	switch d.Name() {
	case "http", "server", "acme":
		acct := f.acctStack.MustPeek().Clone()
		f.acctStack.Push(acct)
		issueOpts := f.issueOptsStack.MustPeek().Clone()
		f.issueOptsStack.Push(issueOpts)
	default:
		return SkipLevel
	}
	switch d.Name() {
	case "server":
		f.serverBlock = &serverBlock{}
	case "acme":
		f.acmeBlock = &acmeBlock{}
	}
	return nil
}

func (f *acmeExtractor) VisitBlockEnd(d *BlockDirective) error {
	f.blockDepth--
	f.visitedDires.Clear()
	switch d.Name() {
	case "http":
		f.acctStack.MustPop()
		f.issueOptsStack.MustPop()
		f.httpBlock = d
	case "server":
		acct := f.acctStack.MustPop()
		issueOpts := f.issueOptsStack.MustPop()
		if f.serverBlock.http {
			f.httpServerBlocks = append(f.httpServerBlocks, f.serverBlock)
		}
		if f.serverBlock.https && len(f.serverBlock.domains) != 0 {
			f.serverBlock.acct = acct
			f.serverBlock.issueOpts = issueOpts
			f.serverBlock.dire = d
			f.httpsServerBlocks = append(f.httpsServerBlocks, f.serverBlock)
			if issueOpts.Challenge == acme.ChallengeHTTP {
				f.hasHTTP01 = true
			}
		}
		f.serverBlock = nil
	case "acme":
		acct := f.acctStack.MustPop()
		issueOpts := f.issueOptsStack.MustPop()
		f.acmeBlock.acct = acct
		f.acmeBlock.issueOpts = issueOpts
		f.acmeBlock.dire = d
		f.acmeBlocks = append(f.acmeBlocks, f.acmeBlock)
		f.acmeBlock = nil
		if issueOpts.Challenge == acme.ChallengeHTTP {
			f.hasHTTP01 = true
		}
		d.Delete()
	}
	return nil
}

func (p *acmeExtractor) VisitDirective(dire Directive) error {
	d, ok := dire.(*SimpleDirective)
	if !ok {
		return nil
	}

	// TODO: check uniq
	switch d.Name() {
	case "listen":
		if p.serverBlock == nil {
			return nil
		}
		https := slices.Contains(d.args[1:], "ssl")
		if https {
			p.serverBlock.https = true
		} else {
			p.serverBlock.http = true
		}
		return nil
	case "server_name":
		if p.serverBlock == nil {
			return nil
		}
		if p.serverBlock.domainsFromACMEDomains {
			return nil
		}
		_, err := d.OnePlusArgs()
		if err != nil {
			return err
		}
		domains := make([]string, 0, len(d.Args()))
		for _, domain := range d.Args() {
			if domain == "" {
				continue
			}
			if domain[0] == '~' {
				return nil
			}
			domains = append(domains, domain)
		}
		p.serverBlock.domains = domains
		return nil
	case "acme_email":
		email, err := d.OneArg()
		if err != nil {
			return err
		}
		p.acctStack.MustPeek().Email = email
		d.Delete()
		return nil
	case "acme_server":
		server, err := d.OneArg()
		if err != nil {
			return err
		}
		p.acctStack.MustPeek().Server = server
		d.Delete()
		return nil
	case "acme_staging":
		on, err := d.BoolArg()
		if err != nil {
			return err
		}
		p.acctStack.MustPeek().Staging = on
		d.Delete()
		return nil
	case "acme_challenge":
		s, err := d.OneArg()
		if err != nil {
			return err
		}
		t, err := acme.ParseChallengeType(s)
		if err != nil {
			// TODO: wrap with loc info
			return err
		}
		p.issueOptsStack.MustPeek().Challenge = t
		d.Delete()
		return nil
	case "acme_days":
		days, err := d.IntArg()
		if err != nil {
			return err
		}
		p.issueOptsStack.MustPeek().Days = &days
		d.Delete()
		return nil
	case "acme_key":
		s, err := d.OneArg()
		if err != nil {
			return err
		}
		t, err := acme.ParseKeyType(s)
		if err != nil {
			// TODO: wrap with loc info
			return err
		}
		p.acctStack.MustPeek().KeyType = t
		p.issueOptsStack.MustPeek().KeyType = t
		d.Delete()
		return nil
	case "acme_dns":
		name, err := d.OneArg()
		if err != nil {
			return err
		}
		p.issueOptsStack.MustPeek().DNS.Name = name
		d.Delete()
		return nil
	case "acme_dns_option":
		k, v, err := d.TwoArgs()
		if err != nil {
			return err
		}
		opts := p.issueOptsStack.MustPeek()
		o := &opts.DNS.Options
		if *o == nil {
			*o = make(map[string]string)
		}
		(*o)[k] = v
		d.Delete()
		return nil
	case "acme_domain":
		if p.acmeBlock == nil && p.serverBlock == nil {
			return nil
		}
		domains, err := d.OnePlusArgs()
		if err != nil {
			return err
		}
		if p.acmeBlock != nil {
			p.acmeBlock.domains = domains
		} else if p.serverBlock != nil {
			p.serverBlock.domains = domains
			p.serverBlock.domainsFromACMEDomains = true
		}
		d.Delete()
		return nil
	case "acme_defer":
		_, err := d.OnePlusArgs()
		if err != nil {
			return err
		}
		dd := newDeferredDirective(d)
		if dd.Name() == "listen" {
			https := slices.Contains(dd.Args()[1:], "ssl")
			if https {
				p.serverBlock.https = true
			} else {
				p.serverBlock.http = true
			}
		}
		d.ReplaceWith(dd)
		return nil
	case "ssl_certificate":
		p.serverBlock.sslCertificate = d
		return nil
	case "ssl_certificate_key":
		p.serverBlock.sslCertificateKey = d
		return nil
	case "ssl_trusted_certificate":
		p.serverBlock.sslTrustedCertificate = d
		return nil
	default:
		return nil
	}
}
