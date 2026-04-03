package maker

// stubTemplates is the Variant A fallback for unknown SDKs.
// It generates a JSON skeleton that documents what the SDK must output.
var stubTemplates = langTemplates{
	Create: `{
  "migration": "{{.Version}}_{{.ClassName}}",
  "up": [
    {
      "action": "create_table",
      "table": "{{.Table}}",
      "columns": [
        {"name": "id", "type": "big_integer", "primary": true, "auto_increment": true}
      ],
      "indexes": [],
      "foreign_keys": []
    }
  ],
  "down": [
    {
      "action": "drop_table",
      "table": "{{.Table}}",
      "if_exists": true
    }
  ]
}
`,

	DropTable: `{
  "migration": "{{.Version}}_{{.ClassName}}",
  "up": [
    {"action": "drop_table", "table": "{{.Table}}", "if_exists": true}
  ],
  "down": [
    {
      "action": "create_table",
      "table": "{{.Table}}",
      "columns": [],
      "indexes": [],
      "foreign_keys": []
    }
  ]
}
`,

	AddColumn: `{
  "migration": "{{.Version}}_{{.ClassName}}",
  "up": [
    {
      "action": "alter_table",
      "table": "{{.Table}}",
      "columns": [
        {"name": "{{.Column}}", "type": "string", "column_action": "add"}
      ]
    }
  ],
  "down": [
    {
      "action": "alter_table",
      "table": "{{.Table}}",
      "columns": [
        {"name": "{{.Column}}", "type": "string", "column_action": "drop"}
      ]
    }
  ]
}
`,

	ChangeColumn: `{
  "migration": "{{.Version}}_{{.ClassName}}",
  "up": [
    {
      "action": "alter_table",
      "table": "{{.Table}}",
      "columns": [
        {"name": "{{.Column}}", "type": "string", "column_action": "change"}
      ]
    }
  ],
  "down": [
    {
      "action": "alter_table",
      "table": "{{.Table}}",
      "columns": [
        {"name": "{{.Column}}", "type": "string", "column_action": "change"}
      ]
    }
  ]
}
`,

	DropColumn: `{
  "migration": "{{.Version}}_{{.ClassName}}",
  "up": [
    {
      "action": "alter_table",
      "table": "{{.Table}}",
      "columns": [
        {"name": "{{.Column}}", "type": "string", "column_action": "drop"}
      ]
    }
  ],
  "down": [
    {
      "action": "alter_table",
      "table": "{{.Table}}",
      "columns": [
        {"name": "{{.Column}}", "type": "string", "column_action": "add"}
      ]
    }
  ]
}
`,

	Blank: `{
  "migration": "{{.Version}}_{{.ClassName}}",
  "up": [],
  "down": []
}
`,

	FileExt: ".migration",
	NamespaceFunc: func(_ string) string { return "" },
}
