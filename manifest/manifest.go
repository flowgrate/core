package manifest

type Migration struct {
	Migration string      `json:"migration"` // e.g. "20260402_163107_CreateUsersTable"
	Up        []Operation `json:"up"`
	Down      []Operation `json:"down"`
}

type Operation struct {
	Action      string       `json:"action"` // create_table, alter_table, drop_table
	Table       string       `json:"table"`
	IfExists    bool         `json:"if_exists,omitempty"`
	Columns     []Column     `json:"columns,omitempty"`
	Indexes     []Index      `json:"indexes,omitempty"`
	ForeignKeys []ForeignKey `json:"foreign_keys,omitempty"`
}

type Column struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	ColumnAction  string `json:"column_action,omitempty"`
	Length        *int   `json:"length,omitempty"`
	Nullable      bool   `json:"nullable,omitempty"`
	Primary       bool   `json:"primary,omitempty"`
	AutoIncrement bool   `json:"auto_increment,omitempty"`
	Default       any    `json:"default,omitempty"`
	Comment       string `json:"comment,omitempty"`
}

type Index struct {
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique,omitempty"`
	Name    string   `json:"name,omitempty"`
}

type ForeignKey struct {
	Column           string `json:"column"`
	ReferencesTable  string `json:"references_table"`
	ReferencesColumn string `json:"references_column"`
	OnUpdate         string `json:"on_update,omitempty"`
	OnDelete         string `json:"on_delete,omitempty"`
}
