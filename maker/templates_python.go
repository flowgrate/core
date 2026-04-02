package maker

var pythonTemplates = langTemplates{
	Create: `from flowgrate import Migration, Schema


class {{.ClassName}}(Migration):
    def up(self):
        with Schema.create("{{.Table}}") as table:
            table.id()
            # TODO: add columns
            table.timestamps()

    def down(self):
        Schema.drop_if_exists("{{.Table}}")
`,

	DropTable: `from flowgrate import Migration, Schema


class {{.ClassName}}(Migration):
    def up(self):
        Schema.drop_if_exists("{{.Table}}")

    def down(self):
        with Schema.create("{{.Table}}") as table:
            table.id()
            # TODO: restore columns
            table.timestamps()
`,

	AddColumn: `from flowgrate import Migration, Schema


class {{.ClassName}}(Migration):
    def up(self):
        with Schema.table("{{.Table}}") as table:
            table.add_column("{{.Column}}").string(255)

    def down(self):
        with Schema.table("{{.Table}}") as table:
            table.drop_column("{{.Column}}")
`,

	ChangeColumn: `from flowgrate import Migration, Schema


class {{.ClassName}}(Migration):
    def up(self):
        with Schema.table("{{.Table}}") as table:
            table.change_column("{{.Column}}").string(255)

    def down(self):
        with Schema.table("{{.Table}}") as table:
            table.change_column("{{.Column}}").string(255)
`,

	DropColumn: `from flowgrate import Migration, Schema


class {{.ClassName}}(Migration):
    def up(self):
        with Schema.table("{{.Table}}") as table:
            table.drop_column("{{.Column}}")

    def down(self):
        with Schema.table("{{.Table}}") as table:
            table.add_column("{{.Column}}").string(255)
`,

	Blank: `from flowgrate import Migration, Schema


class {{.ClassName}}(Migration):
    def up(self):
        pass  # TODO

    def down(self):
        pass  # TODO
`,

	FileExt: ".py",
	NamespaceFunc: func(datePrefix string) string {
		return datePrefix
	},
}
