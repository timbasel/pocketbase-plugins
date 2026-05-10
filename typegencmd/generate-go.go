package typegencmd

import (
	"go/format"
	"os"
	"path"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

func GenerateGoTypes(app core.App, dir string) error {
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}

	collections, err := app.FindAllCollections()
	if err != nil {
		return err
	}
	collections = filterSystemCollections(collections)

	for _, collection := range collections {
		gen := GoGenerator{
			Name:        strings.TrimSuffix(toCamelCase(collection.Name, true), "s"),
			PackageName: path.Base(dir),
		}
		gen.writeCollection(collection)

		fileName := collection.Name + ".go"
		formatted, err := format.Source([]byte(gen.String()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(path.Join(dir, fileName), formatted, 0644); err != nil {
			return err
		}
	}
	return nil
}

type GoGenerator struct {
	generator
	Name        string
	PackageName string
}

func (gen *GoGenerator) writeCollection(collection *core.Collection) {
	gen.writePackageHeader()
	gen.writeImports()

	gen.writeStruct(collection)
	gen.writeStructMethods(collection)

	gen.writeRecordStruct()
	for _, name := range collection.Fields.FieldNames() {
		field := collection.Fields.GetByName(name)
		if f, ok := field.(*core.SelectField); ok {
			gen.writeSelectOptions(f)
		}
		gen.writeFieldGetter(field)
		gen.writeFieldSetter(field)
	}

}

func (gen *GoGenerator) writePackageHeader() {
	gen.write("/**\n")
	gen.write(" * This file is @generated using the typegencmd plugin\n")
	gen.write(" */\n")
	gen.write("package %s\n\n", gen.PackageName)
}

func (gen *GoGenerator) writeImports() {
	gen.write("import (\n")
	gen.writeIndent("%s\n", quote("github.com/pocketbase/pocketbase/core"))
	gen.writeIndent("%s\n", quote("github.com/pocketbase/pocketbase/tools/types"))
	gen.write(")\n\n")
}

func (gen *GoGenerator) writeStruct(collection *core.Collection) {
	gen.write("type %s struct {\n", toCamelCase(gen.Name, true))
	for _, name := range collection.Fields.FieldNames() {
		field := collection.Fields.GetByName(name)
		name := field.GetName()
		gen.writeIndent("%s %s `json:\"%s\" db:\"%s\"`\n", toCamelCase(name, true), gen.getFieldType(field), name, name)
	}
	gen.write("}\n\n")
}

func (gen *GoGenerator) writeStructMethods(collection *core.Collection) {
	recv := strings.ToLower(gen.Name[:1])
	gen.write("func (%s %s) SetRecord(record *core.Record) {\n", recv, toCamelCase(gen.Name, true))
	for _, name := range collection.Fields.FieldNames() {
		field := collection.Fields.GetByName(name)
		gen.writeIndent("record.Set(%s, %s.%s)\n", quote(field.GetName()), recv, toCamelCase(field.GetName(), true))
	}
	gen.write("}\n\n")
}

func (gen *GoGenerator) writeRecordStruct() {
	gen.write("var _ core.RecordProxy = (*%sRecord)(nil)\n\n", gen.Name)

	gen.write("type %sRecord struct {\n", gen.Name)
	gen.writeIndent("core.BaseRecordProxy\n")
	gen.write("}\n\n")
}

func (gen *GoGenerator) writeSelectOptions(field *core.SelectField) {
	enumName := gen.getEnumName(field)
	gen.write("type %s string\n\n", enumName)
	gen.write("const (\n")
	for _, value := range field.Values {
		name := capitalize(value)
		if name == "" {
			name = "_"
		}
		gen.writeIndent("%s %s = %s\n", enumName+name, enumName, quote(value))
	}
	gen.write(")\n\n")
}

func (gen *GoGenerator) writeFieldGetter(field core.Field) {
	recv := strings.ToLower(gen.Name[:1])
	name := field.GetName()
	typ := gen.getFieldType(field)
	gen.write("func (%s %sRecord) %s() %s {\n", recv, gen.Name, toCamelCase(name, true), typ)
	switch f := field.(type) {
	case *core.BoolField:
		gen.writeIndent("return %s.GetBool(%s)\n", recv, quote(name))
	case *core.TextField, *core.EditorField, *core.EmailField, *core.URLField, *core.PasswordField:
		gen.writeIndent("return %s.GetString(%s)\n", recv, quote(name))
	case *core.JSONField:
		gen.writeIndent("return %s.Get(%s).(%s)\n", recv, quote(name), typ)
	case *core.DateField, *core.AutodateField:
		gen.writeIndent("return %s.GetDateTime(%s)\n", recv, quote(name))
	case *core.GeoPointField:
		gen.writeIndent("return %s.GetGeoPoint(%s)\n", recv, quote(name))
	case *core.NumberField:
		if f.OnlyInt {
			gen.writeIndent("return %s.GetInt(%s)\n", recv, quote(name))
		} else {
			gen.writeIndent("return %s.GetFloat(%s)\n", recv, quote(name))
		}
	case *core.RelationField:
		if f.MaxSelect > 1 {
			gen.writeIndent("return %s.GetStringSlice(%s)\n", recv, quote(name))
		} else {
			gen.writeIndent("return %s.GetString(%s)\n", recv, quote(name))
		}
	case *core.FileField:
		if f.MaxSelect > 1 {
			gen.writeIndent("return %s.GetStringSlice(%s)\n", recv, quote(name))
		} else {
			gen.writeIndent("return %s.GetString(%s)\n", recv, quote(name))
		}
	case *core.SelectField:
		if f.MaxSelect > 1 {
			gen.writeIndent("return %s.Get(%s).([]%s)\n", recv, quote(name), gen.getEnumName(f))
		} else {
			gen.writeIndent("return %s(%s.GetString(%s))\n", gen.getEnumName(f), recv, quote(name))
		}
	}
	gen.write("}\n\n")
}

func (gen *GoGenerator) writeFieldSetter(field core.Field) {
	if _, ok := field.(*core.AutodateField); ok {
		return
	}

	recv := strings.ToLower(gen.Name[:1])
	varName := toCamelCase(field.GetName(), false)
	gen.write("func (%s *%sRecord) Set%s(%s %s) {\n", recv, gen.Name, toCamelCase(field.GetName(), true), varName, gen.getFieldType(field))
	gen.writeIndent("%s.Set(%s, %s)\n", recv, quote(field.GetName()), varName)
	gen.write("}\n\n")
}

func (gen *GoGenerator) getFieldType(field core.Field) string {
	switch f := field.(type) {
	case *core.BoolField:
		return "bool"
	case *core.TextField, *core.EditorField, *core.EmailField, *core.URLField, *core.PasswordField:
		return "string"
	case *core.JSONField:
		return "types.JSONRaw"
	case *core.DateField, *core.AutodateField:
		return "types.DateTime"
	case *core.GeoPointField:
		return "types.GeoPoint"
	case *core.NumberField:
		if f.OnlyInt {
			return "int"
		} else {
			return "float64"
		}
	case *core.RelationField:
		if f.MaxSelect > 1 {
			return "[]string"
		} else {
			return "string"
		}
	case *core.FileField:
		if f.MaxSelect > 1 {
			return "[]string"
		} else {
			return "string"
		}
	case *core.SelectField:
		if f.MaxSelect > 1 {
			return "[]" + gen.getEnumName(f)
		} else {
			return gen.getEnumName(f)
		}
	}
	return ""
}

func (gen *GoGenerator) getEnumName(field *core.SelectField) string {
	return gen.Name + toCamelCase(field.GetName(), true)
}

func (gen *GoGenerator) getReceiverName() string {
	return strings.ToLower(gen.Name[:1])
}
