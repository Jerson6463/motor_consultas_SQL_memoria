package parser

// Query representa una consulta SELECT analizada.
type Query struct {
	Select  []SelectItem
	From    FromItem
	Where   Expression
	GroupBy []Expression
	Having  Expression
	OrderBy []SortTerm
	Limit   *int
	Offset  *int
}

// SelectItem es un elemento de la lista de seleccion: una expresion con un
// alias opcional, o el comodin `*`.
type SelectItem struct {
	Star       bool
	Expression Expression
	Alias      string
}

// FromItem es una fuente de filas de la clausula FROM.
type FromItem interface {
	fromItem()
}

// TableRef nombra una tabla del catalogo.
type TableRef struct {
	Name string
}

// JoinRef combina dos fuentes. Los INNER JOIN encadenados se pliegan hacia la
// izquierda, de modo que `a JOIN b JOIN c` es JoinRef{JoinRef{a, b}, c}.
type JoinRef struct {
	Left  FromItem
	Right FromItem
	On    Expression
}

func (*TableRef) fromItem() {}
func (*JoinRef) fromItem()  {}

// SortTerm describe una expresion de ordenamiento y su direccion.
type SortTerm struct {
	Expression Expression
	Descending bool
}

// Expression es un nodo del arbol de expresiones.
type Expression interface {
	expression()
}

// Identifier representa el nombre de una columna.
type Identifier struct {
	Name string
}

func (Identifier) expression() {}

// LiteralKind distingue los literales admitidos por el AST.
type LiteralKind uint8

const (
	LiteralInvalid LiteralKind = iota
	LiteralString
	LiteralNumber
	LiteralBoolean
	LiteralNull
)

// Literal representa un texto, numero, booleano o NULL.
type Literal struct {
	Value string
	Kind  LiteralKind
}

func (Literal) expression() {}

// FunctionCall representa una llamada como SUM(salario) o COUNT(*). El parser
// no conoce los nombres validos: los resuelve el registro de funciones.
type FunctionCall struct {
	Name string
	Args []Expression
	Star bool
}

func (FunctionCall) expression() {}

// BinaryOp identifica un operador logico, de comparacion o aritmetico.
type BinaryOp uint8

const (
	OpInvalid BinaryOp = iota
	OpAnd
	OpOr
	OpEqual
	OpNotEqual
	OpLess
	OpGreater
	OpLessEqual
	OpGreaterEqual
	OpAdd
	OpSubtract
	OpMultiply
	OpDivide
)

// String devuelve la representacion SQL del operador, usada en los mensajes de
// error y en el nombre por defecto de las columnas calculadas.
func (o BinaryOp) String() string {
	switch o {
	case OpAnd:
		return "AND"
	case OpOr:
		return "OR"
	case OpEqual:
		return "="
	case OpNotEqual:
		return "<>"
	case OpLess:
		return "<"
	case OpGreater:
		return ">"
	case OpLessEqual:
		return "<="
	case OpGreaterEqual:
		return ">="
	case OpAdd:
		return "+"
	case OpSubtract:
		return "-"
	case OpMultiply:
		return "*"
	case OpDivide:
		return "/"
	default:
		return "invalido"
	}
}

// IsArithmetic indica si el operador produce un valor numerico en vez de un
// booleano.
func (o BinaryOp) IsArithmetic() bool {
	switch o {
	case OpAdd, OpSubtract, OpMultiply, OpDivide:
		return true
	default:
		return false
	}
}

// BinaryExpression representa una operacion con dos operandos.
type BinaryExpression struct {
	Left     Expression
	Operator BinaryOp
	Right    Expression
}

func (BinaryExpression) expression() {}

// UnaryOp identifica un operador de un solo operando.
type UnaryOp uint8

const (
	UnaryInvalid UnaryOp = iota
	OpNegate
	OpPositive
)

func (o UnaryOp) String() string {
	switch o {
	case OpNegate:
		return "-"
	case OpPositive:
		return "+"
	default:
		return "invalido"
	}
}

// UnaryExpression representa el signo aplicado a un operando, como -salario.
type UnaryExpression struct {
	Operator UnaryOp
	Operand  Expression
}

func (UnaryExpression) expression() {}
