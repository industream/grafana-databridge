package databridge

// DatabaseInfo represents a database from the DataBridge API.
type DatabaseInfo struct {
	Name string `json:"name"`
}

// DatasetInfo represents a dataset from the DataBridge API.
type DatasetInfo struct {
	Name string `json:"name"`
}

// DatasetSchema holds column information for a dataset.
type DatasetSchema struct {
	Columns []ColumnInfo `json:"columns"`
}

// ColumnInfo describes a single column.
type ColumnInfo struct {
	Name     string `json:"name"`
	DataType string `json:"dataType"`
}

// RecordsQuery is the request body for POST /records/query.
type RecordsQuery struct {
	Select  []SelectClause `json:"select,omitempty"`
	Where   interface{}    `json:"where,omitempty"`
	GroupBy []GroupClause  `json:"groupBy,omitempty"`
	OrderBy []OrderClause  `json:"orderBy,omitempty"`
	Limit   int            `json:"limit,omitempty"`
	Offset  int            `json:"offset,omitempty"`
}

// SelectClause represents a column selection with optional aggregation.
type SelectClause struct {
	Column     string       `json:"column,omitempty"`
	Function   string       `json:"function,omitempty"`
	Parameters []QueryParam `json:"parameters,omitempty"`
	Alias      string       `json:"alias,omitempty"`
}

// QueryParam is a polymorphic parameter that can be a column reference or a constant.
type QueryParam struct {
	Column   string      `json:"column,omitempty"`
	Constant interface{} `json:"constant,omitempty"`
}

// GroupClause represents a GROUP BY expression.
type GroupClause struct {
	Column     string       `json:"column,omitempty"`
	Function   string       `json:"function,omitempty"`
	Parameters []QueryParam `json:"parameters,omitempty"`
	Alias      string       `json:"alias,omitempty"`
}

// OrderClause represents an ORDER BY expression.
type OrderClause struct {
	Column    string `json:"column"`
	Direction string `json:"direction"`
}

// WhereExpression is a boolean expression tree for the WHERE clause.
type WhereExpression struct {
	Operator   string             `json:"operator"`
	Conditions []WhereCondition   `json:"conditions,omitempty"`
	Left       *WhereOperand      `json:"left,omitempty"`
	Right      *WhereOperand      `json:"right,omitempty"`
}

// WhereCondition is a single condition in a boolean expression.
type WhereCondition struct {
	Operator string        `json:"operator"`
	Left     *WhereOperand `json:"left"`
	Right    *WhereOperand `json:"right"`
}

// WhereOperand is either a column reference or a constant value.
type WhereOperand struct {
	Column   string      `json:"column,omitempty"`
	Constant interface{} `json:"constant,omitempty"`
}

// RecordsResponse is the response from POST /records/query.
type RecordsResponse struct {
	Columns []string        `json:"columns"`
	Items   [][]interface{} `json:"items"`
}
