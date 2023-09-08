package nginx

import (
	"errors"
	"slices"
)

type Visitor interface {
	VisitTreeBegin(*Tree) error
	VisitTreeEnd(*Tree) error
	VisitConfigBegin(*Config) error
	VisitConfigEnd(*Config) error
	VisitBlockBegin(*BlockDirective) error
	VisitBlockEnd(*BlockDirective) error
	VisitDirective(Directive) error
}

//lint:ignore ST1012 follows fs.SkipAll
var SkipLevel = errors.New("skip this level")

//lint:ignore ST1012 follows fs.SkipAll
var SkipAll = errors.New("skip and exit")

type NoopVisitor struct{}

func (NoopVisitor) VisitTreeBegin(*Tree) error {
	return nil
}
func (NoopVisitor) VisitTreeEnd(*Tree) error {
	return nil
}
func (NoopVisitor) VisitConfigBegin(*Config) error {
	return nil
}
func (NoopVisitor) VisitConfigEnd(*Config) error {
	return nil
}
func (NoopVisitor) VisitBlockBegin(*BlockDirective) error {
	return nil
}
func (NoopVisitor) VisitBlockEnd(*BlockDirective) error {
	return nil
}
func (NoopVisitor) VisitDirective(Directive) error {
	return nil
}

func (tr *Tree) Accept(visitor Visitor) (err error) {
	defer func() {
		// TODO: disallow SkipAll at the tree level?
		e := visitor.VisitTreeEnd(tr)
		switch e {
		case SkipLevel, SkipAll:
			e = nil
		}
		if err == nil {
			err = e
		}

		v := recover()
		switch v {
		case nil, SkipAll:
		default:
			panic(v)
		}
	}()
	err = visitor.VisitTreeBegin(tr)
	switch err {
	case nil:
	case SkipLevel, SkipAll:
		return nil
	default:
		return err
	}
	return tr.conf.Accept(visitor)
}

func (conf *Config) Accept(visitor Visitor) (err error) {
	defer func() {
		e := visitor.VisitConfigEnd(conf)
		switch e {
		case SkipLevel:
			e = nil
		case SkipAll:
			panic(SkipAll)
		}
		if err == nil {
			err = e
		}
	}()
	err = visitor.VisitConfigBegin(conf)
	switch err {
	case nil:
	case SkipLevel:
		return nil
	case SkipAll:
		panic(SkipAll)
	default:
		return err
	}
	return visitChildren(visitor, conf.Children)
}

func (d *SimpleDirective) Accept(visitor Visitor) error {
	return nil
}

func (d *IncludeDirective) Accept(visitor Visitor) error {
	for _, conf := range d.Includes {
		err := conf.Accept(visitor)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *BlockDirective) Accept(visitor Visitor) (err error) {
	defer func() {
		e := visitor.VisitBlockEnd(d)
		switch e {
		case SkipLevel:
			e = nil
		case SkipAll:
			panic(SkipAll)
		}
		if err == nil {
			err = e
		}
	}()
	err = visitor.VisitBlockBegin(d)
	switch err {
	case nil:
	case SkipLevel:
		return nil
	case SkipAll:
		panic(SkipAll)
	default:
		return
	}
	return visitChildren(visitor, d.Children)
}

func visitChildren(visitor Visitor, children []Directive) error {
	// clone is needed because visiting a directive might change the children
	// list (e.g., the visited directive is removed)
	for _, d := range slices.Clone(children) {
		err := visitor.VisitDirective(d)
		switch err {
		case nil:
		case SkipLevel:
			return nil
		case SkipAll:
			panic(SkipAll)
		default:
			return err
		}
		err = d.Accept(visitor)
		if err != nil {
			return err
		}
	}
	return nil
}
