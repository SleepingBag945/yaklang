package ssa

import (
	"fmt"
	"strings"

	"github.com/yaklang/yaklang/common/log"
	"github.com/yaklang/yaklang/common/utils"
	"github.com/yaklang/yaklang/common/utils/memedit"
	"github.com/yaklang/yaklang/common/utils/omap"
	"golang.org/x/exp/slices"
)

type anInstruction struct {
	fun   Value
	prog  *Program
	block Instruction
	R     memedit.RangeIf
	// scope *Scope

	name        string
	verboseName string // verbose name for output or debug or tag
	id          int64

	isAnnotation bool
	isExtern     bool
	isFromDB     bool
}

func (v *anInstruction) GetSourceCode() string {
	r := v.GetRange()
	if r == nil {
		return ""
	}
	return r.GetText()
}

func (v *anInstruction) GetSourceCodeContext(n int) string {
	r := v.GetRange()
	if r == nil {
		return ""
	}
	return r.GetTextContext(n)
}

func (i *anInstruction) IsUndefined() bool {
	return false
}

func (i *anInstruction) IsParameter() bool {
	return false
}

func (i *anInstruction) IsBlock(name string) bool {
	if i.GetOpcode() == SSAOpcodeBasicBlock {
		return strings.HasPrefix(i.GetName(), name)
	}
	return false
}

func (i *anInstruction) SelfDelete() {
	DeleteInst(i)
}

func (i *anInstruction) IsCFGEnterBlock() ([]Instruction, bool) {
	return nil, false
}

func (i *anInstruction) IsLazy() bool { return false }

func (i *anInstruction) IsFromDB() bool { return i.isFromDB }

func (i *anInstruction) SetIsFromDB(b bool) { i.isFromDB = b }

func (i *anInstruction) Self() Instruction {
	return i
}

func (i *anInstruction) ReplaceValue(Value, Value) {
}

func (i *anInstruction) GetVerboseName() string {
	if i.verboseName != "" {
		return i.verboseName
	}
	if i.name != "" {
		return i.name
	}
	return ""
}

func (i *anInstruction) GetShortVerboseName() string {
	if i.name != "" {
		return i.name
	}
	return "t" + fmt.Sprint(i.GetId())
}

func (i *anInstruction) SetVerboseName(verbose string) {
	i.verboseName = verbose
}

func NewInstruction() anInstruction {
	return anInstruction{
		id: -1,
	}
}

// ssa function and block
func (a *anInstruction) SetFunc(f *Function) {
	a.fun = f
	a.prog = f.GetProgram()
}

func (a *anInstruction) GetFunc() *Function {
	f, ok := ToFunction(a.fun)
	if ok {
		return f
	}
	return nil
}

func (a *anInstruction) GetProgram() *Program {
	return a.prog
}

func (a *anInstruction) GetProgramName() string {
	if a.prog == nil {
		return ""
	}
	return a.prog.Name
}

func (a *anInstruction) SetProgram(prog *Program) {
	a.prog = prog
}

func (a *anInstruction) SetIsAnnotation(b bool) {
	a.isAnnotation = b
}

func (a *anInstruction) IsAnnotation() bool {
	return a.isAnnotation
}

func (a *anInstruction) SetBlock(block *BasicBlock) { a.block = block }
func (a *anInstruction) GetBlock() *BasicBlock {
	if a.block == nil {
		return nil
	}
	if block, ok := ToBasicBlock(a.block); ok {
		return block
	}
	log.Warnf("GetBlock: block is not a BasicBlock but: %v", a.block)
	return nil
}

// source code position
func (c *anInstruction) GetRange() memedit.RangeIf { return c.R }

func (c *anInstruction) SetRange(pos memedit.RangeIf) {
	// if c.Pos == nil {
	c.R = pos
	// }
}

// func (c *anInstruction) SetRangeInit(editor *memedit.MemEditor) {
// 	if c.R == nil {
// 		fullRange := editor.GetFullRange()
// 		c.R = NewRange(editor, fullRange.GetStart(), fullRange.GetEnd())
// 	}
// }

func (c *anInstruction) IsExtern() bool   { return c.isExtern }
func (c *anInstruction) SetExtern(b bool) { c.isExtern = b }

// error logger
func (c *anInstruction) NewError(kind ErrorKind, tag ErrorTag, msg string) {
	c.GetFunc().NewErrorWithPos(kind, tag, c.GetRange(), msg)
}

// symbol-table
// func (a *anInstruction) GetScope() *Scope  { return a.scope }
// func (a *anInstruction) SetScope(s *Scope) { a.scope = s }

// variable
func (a *anInstruction) SetName(v string) { a.name = v }
func (a *anInstruction) GetName() string  { return a.name }

// id
func (a *anInstruction) SetId(id int64) { a.id = id }

func (a *anInstruction) GetId() int64 {
	if a == nil {
		return -1
	}
	return a.id
}

func (a *anInstruction) LineDisasm() string { return "" }

// opcode
func (a *anInstruction) GetOpcode() Opcode { return SSAOpcodeUnKnow } // cover by instruction

var _ Instruction = (*anInstruction)(nil)

type parserMode int

const (
	prePar parserMode = iota
	Paring
	ParEnd
)

type anValue struct {
	anInstruction

	typ      Type
	userList Users

	object Value
	key    Value
	member *omap.OrderedMap[Value, Value] // map[Value]Value

	variables *omap.OrderedMap[string, *Variable] // map[string]*Variable

	// mask is a map, key is variable name, value is variable value
	// it record the variable is masked by closure function or some scope changed
	mask *omap.OrderedMap[string, Value]

	pointer   Values // the pointer is point to this value
	reference Value  // the value is pointed by this value

	//parse value
	parseMode parserMode //parse mode
	spinValue Value
	build     func() Value
	this      Value
}

func (n *anValue) SetOrdinalBuild(Builder func() Value) {
	n.build = Builder
}
func (n *anValue) Build() Value {
	switch n.parseMode {
	case prePar:
		n.parseMode = Paring
		val := n.build()
		n.parseMode = ParEnd
		n.this = val
		return val
	case Paring:
		if utils.IsNil(n.spinValue) {
			n.spinValue = n.GetFunc().builder.EmitConstInst("spin value")
		}
		return n.spinValue
	case ParEnd:
		return n.this
	}
	return nil
}
func (n *anValue) FixSpinUdChain() {
	if !utils.IsNil(n.spinValue) {
		if !utils.IsNil(n.this) {
			ReplaceAllValue(n.spinValue, n.this)
		} else {
			ReplaceAllValue(n.spinValue, n.GetFunc().builder.EmitConstInst("unknown_replace_value"))
		}
	}
}
func (n *anValue) CheckAndFinishBuild() bool {
	if n.parseMode == prePar {
		n.Build()
		return true
	}
	return false
}
func NewValue() anValue {
	return anValue{
		anInstruction: NewInstruction(),
		typ:           BasicTypes[AnyTypeKind],
		userList:      make(Users, 0),
		object:        nil,
		key:           nil,
		member:        omap.NewOrderedMap(map[Value]Value{}),

		variables: omap.NewOrderedMap(map[string]*Variable{}),
		mask:      omap.NewOrderedMap(map[string]Value{}),

		parseMode: prePar,
		build: func() Value {
			return nil
		},
	}
}

func (n *anValue) IsMember() bool {
	return n.object != nil && n.key != nil
}

func (n *anValue) SetObject(v Value) {
	n.object = v
}

func (n *anValue) GetObject() Value {
	return n.object
}

func (n *anValue) SetKey(k Value) {
	n.key = k
}

func (n *anValue) GetKey() Value {
	return n.key
}

func (n *anValue) IsObject() bool {
	return n.member.Len() != 0
}

func (n *anValue) AddMember(k, v Value) {
	// n.member = append(n.member, v)
	// n.member[k] = v
	n.member.Set(k, v)
}

func (n *anValue) DeleteMember(k Value) {
	n.member.Delete(k)
}

func (n *anValue) GetMember(key Value) (Value, bool) {
	ret, ok := n.member.Get(key)
	if !ok {
		return nil, false
	}
	return ret, true
}

func (n *anValue) GetIndexMember(i int) (Value, bool) {
	return n.member.GetByIndex(i)
}

func (n *anValue) GetStringMember(key string) (Value, bool) {
	for _, i := range n.member.Keys() {
		lit, ok := i.(*ConstInst)
		if !ok {
			continue
		}
		if lit.value == key {
			return n.member.Get(i)
		}
	}
	return nil, false
}

func (n *anValue) SetStringMember(key string, v Value) {
	for _, i := range n.member.Keys() {
		lit, ok := i.(*ConstInst)
		if !ok {
			continue
		}
		if lit.value == key {
			n.member.Set(lit, v)
		}
	}
}

func (n *anValue) GetAllMember() map[Value]Value {
	return n.member.GetMap()
}

func (n *anValue) ForEachMember(fn func(Value, Value) bool) {
	n.member.ForEach(fn)
}

func (n *anValue) String() string { return "" }

// has/get user and value
func (n *anValue) HasUsers() bool  { return len(n.userList) != 0 }
func (n *anValue) GetUsers() Users { return n.userList }

// for Value
func (n *anValue) AddUser(u User) {
	if index := slices.Index(n.userList, u); index == -1 {
		n.userList = append(n.userList, u)
	}
}

func (n *anValue) RemoveUser(u User) {
	n.userList = utils.RemoveSliceItem(n.userList, u)
}

// for Value : type
func (n *anValue) GetType() Type {
	if n == nil {
		log.Errorf("BUG in *anValue.GetType(), the *anValue is nil!")
		return GetAnyType()
	}
	return n.typ
}

func (n *anValue) SetType(typ Type) {
	if typ == nil {
		return
	}

	if n.IsFromDB() {
		n.typ = typ
		return
	}

	getThis := func() Value {
		value, ok := n.GetProgram().GetInstructionById(n.GetId()).(Value)
		if !ok {
			log.Errorf("SetType: value is not Value but is %d", n.GetId())
		}
		return value
	}

	switch t := typ.(type) {
	case *ClassBluePrint:
		n.typ = t.Apply(getThis())
	case *FunctionType:
		n.typ = typ
		this := getThis()
		if this == nil {
			return
		}
		if fun := t.This; fun != nil {
			Point(this, fun)
		}
		for _, f := range t.AnnotationFunc {
			f(this)
		}

	default:
		n.typ = typ
	}
}

func (a *anValue) GetVariable(name string) *Variable {
	if ret, ok := a.variables.Get(name); ok {
		return ret
	} else {
		return nil
	}
}

func (a *anValue) GetLastVariable() *Variable {
	_, v, _ := a.variables.Last()
	return v
}

func (a *anValue) GetAllVariables() map[string]*Variable {
	return a.variables.GetMap()
}

func (a *anValue) AddVariable(v *Variable) {
	name := v.GetName()
	a.variables.Set(name, v)
	a.variables.BringKeyToLastOne(name)
}

func (i *anValue) AddMask(v Value) {
	i.mask.Add(v)
}

func (i *anValue) GetMask() []Value {
	return i.mask.Values()
}

func (i *anValue) Masked() bool {
	return i.mask.Len() != 0
}

func (i *anValue) SetReference(v Value) {
	i.reference = v
}

func (i *anValue) GetReference() Value {
	return i.reference
}

func (i *anValue) AddPointer(v Value) {
	i.pointer = append(i.pointer, v)
}

func (i *anValue) GetPointer() Values {
	return i.pointer
}
