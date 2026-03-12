package code

// Tree-sitter query strings per language.
// These are proven patterns from mache's examples/go-schema.json.

// GoQueries defines the tree-sitter queries for extracting Go entities.
var GoQueries = []EntityQuery{
	{
		Kind:  "function",
		Query: `(function_declaration name: (identifier) @name) @scope`,
	},
	{
		Kind:  "method",
		Query: `(method_declaration receiver: (parameter_list (parameter_declaration type: (pointer_type (type_identifier) @receiver))) name: (field_identifier) @name) @scope`,
	},
	{
		Kind:  "method",
		Query: `(method_declaration receiver: (parameter_list (parameter_declaration type: (type_identifier) @receiver)) name: (field_identifier) @name) @scope`,
	},
	{
		Kind:  "type",
		Query: `(type_declaration (type_spec name: (type_identifier) @name) @scope)`,
	},
	{
		Kind:  "constant",
		Query: `(const_spec name: (identifier) @name) @scope`,
	},
	{
		Kind:  "variable",
		Query: `(var_spec name: (identifier) @name) @scope`,
	},
}

// EntityQuery pairs a construct kind with a tree-sitter query pattern.
type EntityQuery struct {
	Kind  string
	Query string
}
