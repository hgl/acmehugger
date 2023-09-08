package nginx

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"sync"
)

type Tree struct {
	name          string
	conf          *Config
	includedConfs map[string]*Config
	confdir       string
	mu            sync.Mutex
}

func (tr *Tree) Change(fn func()) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	fn()
}

func (tr *Tree) Config() *Config {
	return tr.conf
}

type Config struct {
	path        string
	Children    []Directive
	parent      *IncludeDirective
	parentBlock *BlockDirective
	text        string
	lexer       *lexer
	tr          *Tree
	out         io.Writer
}

func (conf *Config) Path() string {
	return conf.path
}
func (conf *Config) Parent() *IncludeDirective {
	return conf.parent
}
func (conf *Config) Tree() *Tree {
	return conf.tr
}

type Directive interface {
	Name() string
	Args() []string
	Accept(Visitor) error
	Parent() any
	ParentBlock() *BlockDirective
	Location() string
	ReplaceWith(...Directive)
	Delete()
	Config() *Config
	Tree() *Tree
	position() pos
	setParent(any)
	setParentBlock(*BlockDirective)
}

type SimpleDirective struct {
	pos
	name        string
	args        []string
	raw         []string
	conf        *Config
	parent      any
	parentBlock *BlockDirective
}

func (d *SimpleDirective) Name() string {
	return d.name
}
func (d *SimpleDirective) Args() []string {
	return d.args
}
func (d *SimpleDirective) SetArg(i int, s string) {
	d.args[i] = s
	d.raw[i+1] = escape(s)
}
func (d *SimpleDirective) BoolArg() (on bool, err error) {
	if len(d.args) == 1 {
		switch d.args[0] {
		case "on":
			on = true
			return
		case "off":
			on = false
			return
		}
	}
	err = fmt.Errorf("%s must be either on or off in %s", d.name, loc(d))
	return
}
func (d *SimpleDirective) OneArg() (arg string, err error) {
	if len(d.args) == 1 {
		arg = d.args[0]
		return
	}
	err = fmt.Errorf("%s requires one value in %s", d.name, loc(d))
	return
}
func (d *SimpleDirective) TwoArgs() (arg1 string, arg2 string, err error) {
	if len(d.args) == 2 {
		arg1 = d.args[0]
		arg2 = d.args[1]
		return
	}
	err = fmt.Errorf("%s requires two values in %s", d.name, loc(d))
	return
}
func (d *SimpleDirective) OnePlusArgs() (args []string, err error) {
	if len(d.args) == 0 {
		err = fmt.Errorf("%s requires at least one value in %s", d.name, loc(d))
	}
	return d.args, err
}
func (d *SimpleDirective) IntArg() (int, error) {
	s, err := d.OneArg()
	if err != nil {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number: %w in %s", d.name, err, loc(d))
	}
	return n, nil
}
func (d *SimpleDirective) Parent() any {
	return d.parent
}
func (d *SimpleDirective) ParentBlock() *BlockDirective {
	return d.parentBlock
}
func (d *SimpleDirective) ReplaceWith(dires ...Directive) {
	replaceDire(d.parent, d, dires...)
}
func (d *SimpleDirective) Delete() {
	deleteDire(d.parent, d)
}
func (d *SimpleDirective) Location() string {
	return loc(d)
}
func (d *SimpleDirective) Config() *Config {
	return d.conf
}
func (d *SimpleDirective) Tree() *Tree {
	return d.conf.tr
}
func (d *SimpleDirective) setParent(parent any) {
	d.parent = parent
}
func (d *SimpleDirective) setParentBlock(b *BlockDirective) {
	d.parentBlock = b
}

type BlockDirective struct {
	pos
	name        string
	args        []string
	raw         []string
	Children    []Directive
	conf        *Config
	parent      any
	parentBlock *BlockDirective
}

func (d *BlockDirective) Name() string {
	return d.name
}
func (d *BlockDirective) Args() []string {
	return d.args
}
func (d *BlockDirective) Parent() any {
	return d.parent
}
func (d *BlockDirective) ParentBlock() *BlockDirective {
	return d.parentBlock
}
func (d *BlockDirective) ReplaceWith(dires ...Directive) {
	replaceDire(d.parent, d, dires...)
}
func (d *BlockDirective) Delete() {
	deleteDire(d.parent, d)
}
func (d *BlockDirective) Location() string {
	return loc(d)
}
func (d *BlockDirective) Config() *Config {
	return d.conf
}
func (d *BlockDirective) Tree() *Tree {
	return d.conf.tr
}
func (d *BlockDirective) setParent(parent any) {
	d.parent = parent
}
func (d *BlockDirective) setParentBlock(b *BlockDirective) {
	d.parentBlock = b
}

type IncludeDirective struct {
	pos
	Target      string
	Includes    []*Config
	conf        *Config
	parent      any
	parentBlock *BlockDirective
}

func (d *IncludeDirective) Name() string {
	return "include"
}
func (d *IncludeDirective) Args() []string {
	return []string{d.Target}
}
func (d *IncludeDirective) Parent() any {
	return d.parent
}
func (d *IncludeDirective) ParentBlock() *BlockDirective {
	return d.parentBlock
}
func (d *IncludeDirective) ReplaceWith(dires ...Directive) {
	replaceDire(d.parent, d, dires...)
}
func (d *IncludeDirective) Delete() {
	deleteDire(d.parent, d)
}
func (d *IncludeDirective) Location() string {
	return loc(d)
}
func (d *IncludeDirective) Config() *Config {
	return d.conf
}
func (d *IncludeDirective) Tree() *Tree {
	return d.conf.tr
}
func (d *IncludeDirective) setParent(parent any) {
	panic("include directive's parent should only be set during parsing")
}
func (d *IncludeDirective) setParentBlock(b *BlockDirective) {
	panic("include directive's parent block should only be set during parsing")
}

type DeferredDirective struct {
	Directive
}

func (d *DeferredDirective) Undefer() {
	replaceDire(d.Parent(), d, d.Directive)
}

func newDeferredDirective(dire Directive) *DeferredDirective {
	switch d := dire.(type) {
	case *SimpleDirective:
		d.name = d.args[0]
		d.args = d.args[1:]
		d.raw = d.raw[1:]
	case *BlockDirective:
		d.name = d.args[0]
		d.args = d.args[1:]
		d.raw = d.raw[1:]
	default:
		panic("directive cannot be deferred")
	}
	return &DeferredDirective{dire}
}

func NewDirective(name string, args []string, block ...Directive) Directive {
	raw := make([]string, len(args)+1)
	raw[0] = escape(name)
	for i := 0; i < len(args); i++ {
		raw[i+1] = escape(args[i])
	}
	if len(block) == 0 {
		return &SimpleDirective{
			pos:  -1,
			name: name,
			args: args,
			raw:  raw,
		}
	}
	return &BlockDirective{
		pos:      -1,
		name:     name,
		args:     args,
		raw:      raw,
		Children: block,
	}
}

func NewBlockDirective(name string, args []string, block ...Directive) *BlockDirective {
	raw := make([]string, len(args)+1)
	raw[0] = escape(name)
	for i := 0; i < len(args); i++ {
		raw[i+1] = escape(args[i])
	}
	return &BlockDirective{
		pos:      -1,
		name:     name,
		args:     args,
		raw:      raw,
		Children: block,
	}
}

func loc(d Directive) string {
	filename := d.Config().path
	text := d.Config().text
	loc := d.position().loc(text)
	return fmt.Sprintf("%s:%s", filename, loc)
}

func replaceDire(parent any, target Directive, replacement ...Directive) {
	var children *[]Directive
	switch p := parent.(type) {
	case *BlockDirective:
		children = &p.Children
	case *Config:
		children = &p.Children
	}
	i := slices.Index(*children, target)
	if i != -1 {
		*children = slices.Replace(*children, i, i+1, replacement...)
	}
}

func deleteDire(parent any, target Directive) {
	var children *[]Directive
	switch p := parent.(type) {
	case *BlockDirective:
		children = &p.Children
	case *Config:
		children = &p.Children
	}
	i := slices.Index(*children, target)
	if i != -1 {
		*children = slices.Delete(*children, i, i+1)
	}
}
