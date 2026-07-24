package planner

import (
	"fmt"
	"strings"

	"motor-consultas-sql/internal/functions"
	"motor-consultas-sql/internal/parser"
	"motor-consultas-sql/internal/storage"
)

// analysis es el resultado de examinar la lista de seleccion y las clausulas
// que dependen de ella.
type analysis struct {
	// aggregated indica si la consulta agrupa, ya sea por GROUP BY o por usar
	// una funcion de agregacion.
	aggregated bool
	// calls son las llamadas de agregacion que debe calcular el operador
	// Aggregate, sin repeticiones.
	calls  []parser.FunctionCall
	items  []ProjectItem
	having parser.Expression
	order  []parser.SortTerm
}

// analyze prepara las expresiones de la consulta para el plan.
//
// Cuando la consulta agrupa, las expresiones del SELECT, del HAVING y del ORDER
// BY se reescriben: cada llamada de agregacion y cada expresion que coincide con
// una clave de GROUP BY se sustituyen por una referencia a la columna que
// produce el operador Aggregate. Asi el Project que va por encima se limita a
// combinar columnas ya calculadas.
//
//	SELECT zona, COUNT(*) * 2 AS doble FROM ventas GROUP BY zona
//	  Aggregate produce:  zona | COUNT(*)
//	  Project evalua:     zona , COUNT(*) * 2   -> ya como referencias
func analyze(statement *parser.Query) (*analysis, error) {
	result := &analysis{}
	rewriter := newRewriter(statement.GroupBy)
	result.aggregated = len(statement.GroupBy) > 0 || usesAggregate(statement)

	if statement.Where != nil && containsAggregate(statement.Where) {
		return nil, fmt.Errorf("no se permiten funciones de agregacion en WHERE")
	}
	if statement.Having != nil && !result.aggregated {
		return nil, fmt.Errorf("HAVING requiere GROUP BY o una funcion de agregacion")
	}
	if err := validateFunctions(statement); err != nil {
		return nil, err
	}

	// El ORDER BY puede citar un alias del SELECT; se resuelve antes de nada.
	order := make([]parser.SortTerm, len(statement.OrderBy))
	for index, term := range statement.OrderBy {
		order[index] = parser.SortTerm{
			Expression: resolveAliases(term.Expression, statement.Select),
			Descending: term.Descending,
		}
	}

	if !result.aggregated {
		result.items = projectItems(statement.Select, nil)
		result.order = order
		return result, nil
	}

	for _, item := range statement.Select {
		if item.Star {
			return nil, fmt.Errorf("no se puede usar * junto a funciones de agregacion")
		}
	}

	// El nombre de salida se calcula con la expresion original, para que el
	// usuario vea lo que escribio y no la version reescrita.
	rewritten := make([]parser.Expression, len(statement.Select))
	for index, item := range statement.Select {
		expression, err := rewriter.rewrite(item.Expression)
		if err != nil {
			return nil, err
		}
		rewritten[index] = expression
	}
	if statement.Having != nil {
		having, err := rewriter.rewrite(statement.Having)
		if err != nil {
			return nil, err
		}
		result.having = having
	}
	for index, term := range order {
		expression, err := rewriter.rewrite(term.Expression)
		if err != nil {
			return nil, err
		}
		order[index].Expression = expression
	}

	// Lo que no se haya sustituido es una columna suelta: ni clave de grupo ni
	// argumento de un agregado.
	for _, expression := range rewritten {
		if err := rewriter.validate(expression); err != nil {
			return nil, err
		}
	}
	if result.having != nil {
		if err := rewriter.validate(result.having); err != nil {
			return nil, err
		}
	}
	for _, term := range order {
		if err := rewriter.validate(term.Expression); err != nil {
			return nil, err
		}
	}

	result.calls = rewriter.calls
	result.items = projectItems(statement.Select, rewritten)
	result.order = order
	return result, nil
}

// projectItems construye las columnas de salida. Si rewritten no es nil, se usa
// esa expresion en lugar de la original, pero el nombre sale siempre de la
// original o del alias.
func projectItems(selected []parser.SelectItem, rewritten []parser.Expression) []ProjectItem {
	items := make([]ProjectItem, len(selected))
	for index, item := range selected {
		if item.Star {
			items[index] = ProjectItem{Star: true}
			continue
		}
		expression := item.Expression
		if rewritten != nil {
			expression = rewritten[index]
		}
		name := item.Alias
		if name == "" {
			name = parser.Format(item.Expression)
		}
		items[index] = ProjectItem{Expression: expression, Name: name}
	}
	return items
}

// rewriter sustituye agregados y claves de grupo por referencias a las columnas
// que produce el operador Aggregate.
type rewriter struct {
	// groups asocia la forma canonica normalizada de una clave de grupo con el
	// nombre de la columna que la publica.
	groups map[string]string
	// produced son los nombres de columna que produce el Aggregate.
	produced  map[string]bool
	calls     []parser.FunctionCall
	callNames map[string]bool
}

func newRewriter(groupBy []parser.Expression) *rewriter {
	r := &rewriter{
		groups:    map[string]string{},
		produced:  map[string]bool{},
		callNames: map[string]bool{},
	}
	for _, expression := range groupBy {
		name := parser.Format(expression)
		r.groups[storage.NormalizeName(name)] = name
		r.produced[storage.NormalizeName(name)] = true
	}
	return r
}

func (r *rewriter) rewrite(expression parser.Expression) (parser.Expression, error) {
	if name, ok := r.groups[storage.NormalizeName(parser.Format(expression))]; ok {
		return parser.Identifier{Name: name}, nil
	}

	switch expression := expression.(type) {
	case parser.FunctionCall:
		return r.rewriteCall(expression)
	case parser.BinaryExpression:
		left, err := r.rewrite(expression.Left)
		if err != nil {
			return nil, err
		}
		right, err := r.rewrite(expression.Right)
		if err != nil {
			return nil, err
		}
		return parser.BinaryExpression{Left: left, Operator: expression.Operator, Right: right}, nil
	case parser.UnaryExpression:
		operand, err := r.rewrite(expression.Operand)
		if err != nil {
			return nil, err
		}
		return parser.UnaryExpression{Operator: expression.Operator, Operand: operand}, nil
	default:
		return expression, nil
	}
}

func (r *rewriter) rewriteCall(call parser.FunctionCall) (parser.Expression, error) {
	name := strings.ToUpper(call.Name)
	spec, ok := functions.Lookup(call.Name)
	if !ok {
		return nil, fmt.Errorf("la funcion %q no existe", call.Name)
	}
	if call.Star && !spec.AcceptsStar {
		return nil, fmt.Errorf("%s requiere una columna", name)
	}
	if !call.Star && len(call.Args) != 1 {
		return nil, fmt.Errorf("%s requiere exactamente un argumento", name)
	}
	if !call.Star && containsAggregate(call.Args[0]) {
		return nil, fmt.Errorf("no se pueden anidar funciones de agregacion")
	}

	column := parser.Format(call)
	if !r.callNames[column] {
		r.callNames[column] = true
		r.calls = append(r.calls, call)
	}
	r.produced[storage.NormalizeName(column)] = true
	return parser.Identifier{Name: column}, nil
}

// validate comprueba que no quede ninguna columna suelta tras la reescritura.
func (r *rewriter) validate(expression parser.Expression) error {
	switch expression := expression.(type) {
	case parser.Identifier:
		if !r.produced[storage.NormalizeName(expression.Name)] {
			return fmt.Errorf("la columna %q debe aparecer en GROUP BY", expression.Name)
		}
		return nil
	case parser.BinaryExpression:
		if err := r.validate(expression.Left); err != nil {
			return err
		}
		return r.validate(expression.Right)
	case parser.UnaryExpression:
		return r.validate(expression.Operand)
	default:
		return nil
	}
}

// resolveAliases sustituye las referencias a un alias del SELECT por la
// expresion a la que da nombre, de modo que ORDER BY pueda citarlo.
func resolveAliases(expression parser.Expression, selected []parser.SelectItem) parser.Expression {
	switch expression := expression.(type) {
	case parser.Identifier:
		for _, item := range selected {
			if item.Alias != "" && storage.NormalizeName(item.Alias) == storage.NormalizeName(expression.Name) {
				return item.Expression
			}
		}
		return expression
	case parser.BinaryExpression:
		return parser.BinaryExpression{
			Left:     resolveAliases(expression.Left, selected),
			Operator: expression.Operator,
			Right:    resolveAliases(expression.Right, selected),
		}
	case parser.UnaryExpression:
		return parser.UnaryExpression{
			Operator: expression.Operator,
			Operand:  resolveAliases(expression.Operand, selected),
		}
	default:
		return expression
	}
}

// usesAggregate indica si la consulta usa alguna funcion de agregacion fuera
// del WHERE.
func usesAggregate(statement *parser.Query) bool {
	for _, item := range statement.Select {
		if !item.Star && containsAggregate(item.Expression) {
			return true
		}
	}
	if statement.Having != nil && containsAggregate(statement.Having) {
		return true
	}
	for _, term := range statement.OrderBy {
		if containsAggregate(term.Expression) {
			return true
		}
	}
	return false
}

// validateFunctions rechaza las llamadas a funciones que no estan registradas,
// en cualquier clausula de la consulta.
func validateFunctions(statement *parser.Query) error {
	expressions := []parser.Expression{statement.Where, statement.Having}
	for _, item := range statement.Select {
		if !item.Star {
			expressions = append(expressions, item.Expression)
		}
	}
	expressions = append(expressions, statement.GroupBy...)
	for _, term := range statement.OrderBy {
		expressions = append(expressions, term.Expression)
	}

	for _, expression := range expressions {
		if expression == nil {
			continue
		}
		if err := checkFunctions(expression); err != nil {
			return err
		}
	}
	return nil
}

func checkFunctions(expression parser.Expression) error {
	switch expression := expression.(type) {
	case parser.FunctionCall:
		if !functions.IsAggregate(expression.Name) {
			return fmt.Errorf("la funcion %q no existe", expression.Name)
		}
		for _, argument := range expression.Args {
			if err := checkFunctions(argument); err != nil {
				return err
			}
		}
		return nil
	case parser.BinaryExpression:
		if err := checkFunctions(expression.Left); err != nil {
			return err
		}
		return checkFunctions(expression.Right)
	case parser.UnaryExpression:
		return checkFunctions(expression.Operand)
	default:
		return nil
	}
}

func containsAggregate(expression parser.Expression) bool {
	switch expression := expression.(type) {
	case parser.FunctionCall:
		if functions.IsAggregate(expression.Name) {
			return true
		}
		for _, argument := range expression.Args {
			if containsAggregate(argument) {
				return true
			}
		}
		return false
	case parser.BinaryExpression:
		return containsAggregate(expression.Left) || containsAggregate(expression.Right)
	case parser.UnaryExpression:
		return containsAggregate(expression.Operand)
	default:
		return false
	}
}
