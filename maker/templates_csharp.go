package maker

var csharpTemplates = langTemplates{
	Create: `using Flowgrate;

namespace {{.Namespace}};

public class {{.ClassName}} : Migration
{
    public static string Version => "{{.Version}}";

    public override void Up()
    {
        Schema.Create("{{.Table}}", table =>
        {
            table.Id();
            // TODO: add columns
            table.Timestamps();
        });
    }

    public override void Down()
    {
        Schema.DropIfExists("{{.Table}}");
    }
}
`,

	DropTable: `using Flowgrate;

namespace {{.Namespace}};

public class {{.ClassName}} : Migration
{
    public static string Version => "{{.Version}}";

    public override void Up()
    {
        Schema.DropIfExists("{{.Table}}");
    }

    public override void Down()
    {
        Schema.Create("{{.Table}}", table =>
        {
            table.Id();
            // TODO: restore columns
            table.Timestamps();
        });
    }
}
`,

	AddColumn: `using Flowgrate;

namespace {{.Namespace}};

public class {{.ClassName}} : Migration
{
    public static string Version => "{{.Version}}";

    public override void Up()
    {
        Schema.Table("{{.Table}}", table =>
        {
            table.AddColumn("{{.Column}}").String(255);
        });
    }

    public override void Down()
    {
        Schema.Table("{{.Table}}", table =>
        {
            table.DropColumn("{{.Column}}");
        });
    }
}
`,

	ChangeColumn: `using Flowgrate;

namespace {{.Namespace}};

public class {{.ClassName}} : Migration
{
    public static string Version => "{{.Version}}";

    public override void Up()
    {
        Schema.Table("{{.Table}}", table =>
        {
            table.ChangeColumn("{{.Column}}").String(255);
        });
    }

    public override void Down()
    {
        Schema.Table("{{.Table}}", table =>
        {
            table.ChangeColumn("{{.Column}}").String(255);
        });
    }
}
`,

	DropColumn: `using Flowgrate;

namespace {{.Namespace}};

public class {{.ClassName}} : Migration
{
    public static string Version => "{{.Version}}";

    public override void Up()
    {
        Schema.Table("{{.Table}}", table =>
        {
            table.DropColumn("{{.Column}}");
        });
    }

    public override void Down()
    {
        Schema.Table("{{.Table}}", table =>
        {
            table.AddColumn("{{.Column}}").String(255);
        });
    }
}
`,

	Blank: `using Flowgrate;

namespace {{.Namespace}};

public class {{.ClassName}} : Migration
{
    public static string Version => "{{.Version}}";

    public override void Up()
    {
        // TODO
    }

    public override void Down()
    {
        // TODO
    }
}
`,

	FileExt: ".cs",
	NamespaceFunc: func(datePrefix string) string {
		return "Migrations._" + datePrefix
	},
}
