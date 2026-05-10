package typegencmd

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"unsafe"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

func GenerateTypescriptTypes(app core.App, path string) error {
	collections, err := app.FindAllCollections()
	if err != nil {
		return err
	}
	collections = filterSystemCollections(collections)

	routes, err := getCustomSendRoutes(app)
	if err != nil {
		return err
	}

	gen := TypescriptGenerator{app: app}
	gen.writeHeader()
	gen.writeCollections(collections)

	for _, collection := range collections {
		gen.writeCollectionRecord(collection)
		gen.writeCollectionResponse(collection)
	}

	gen.writeCollectionRecords(collections)
	gen.writeCollectionResponses(collections)

	gen.writeCustomSendRoutes(routes)

	return os.WriteFile(path, []byte(gen.String()), 0644)
}

type TypescriptGenerator struct {
	generator
	app core.App
}

func (gen *TypescriptGenerator) writeHeader() {
	gen.write("/**\n")
	gen.write(" * This file is @generated using the typegencmd plugin\n")
	gen.write(" */\n")
	gen.write("\n")

	gen.write("type ExpandType<T = unknown> = T extends unknown ? { expand?: unknown } : { expand: T }\n")
	gen.write("\n")

	gen.write("export type BaseCollectionFields<T = unknown> = {\n")
	gen.writeIndent("id: string;\n")
	gen.writeIndent("collectionId: string;\n")
	gen.writeIndent("collectionName: Collections;\n")
	gen.write("} & ExpandType<T>\n")
	gen.write("\n")

	gen.write("export type AuthCollectionFields<T = unknown> = {\n")
	gen.writeIndent("email: string;\n")
	gen.writeIndent("emailVisibility: string;\n")
	gen.writeIndent("username: string;\n")
	gen.writeIndent("verified: boolean;\n")
	gen.write("} & BaseCollectionFields<T>\n")
	gen.write("\n")
}

func (gen *TypescriptGenerator) writeCollections(collections []*core.Collection) {
	gen.write("export type Collections = \n")
	for index, collection := range collections {
		gen.writeIndent("| %s", quote(collection.Name))
		if index < len(collections)-1 {
			gen.write("\n")
		} else {
			gen.write(";\n\n")
		}
	}
}

func (gen *TypescriptGenerator) writeCollectionRecord(collection *core.Collection) {
	name := capitalize(collection.Name)
	gen.write("export type %sRecord%s = {\n", name, formatGenericArgs(getGenericArgs(collection, true)...))
	for _, name := range collection.Fields.FieldNames() {
		field := collection.Fields.GetByName(name)
		gen.writeIndent("%s?: %s,\n", name, gen.getFieldType(field))
	}
	gen.write("}\n\n")
}

func (gen *TypescriptGenerator) writeCollectionResponse(collection *core.Collection) {
	name := capitalize(collection.Name)
	gen.write("export type %sResponse%s = Required<%sRecord%s> & BaseCollectionFields<TExpand>;\n\n", name, formatGenericArgs(append(getGenericArgs(collection, true), "TExpand = unknown")...), name, formatGenericArgs(getGenericArgs(collection, false)...))
}

func (gen *TypescriptGenerator) writeCollectionRecords(collections []*core.Collection) {
	gen.write("export type CollectionRecords = {\n")
	for _, collection := range collections {
		gen.writeIndent("%s: %sRecord,\n", collection.Name, capitalize(collection.Name))
	}
	gen.write("}\n\n")
}

func (gen *TypescriptGenerator) writeCollectionResponses(collections []*core.Collection) {
	gen.write("export type CollectionResponses = {\n")
	for _, collection := range collections {
		gen.writeIndent("%s: %sResponse,\n", collection.Name, capitalize(collection.Name))
	}
	gen.write("}\n\n")
}

func (gen *TypescriptGenerator) getFieldType(field core.Field) string {
	switch f := field.(type) {
	case *core.BoolField:
		return "boolean"
	case *core.TextField, *core.EditorField, *core.EmailField, *core.URLField, *core.PasswordField:
		return "string"
	case *core.JSONField:
		return getGenericName(f.Name)
	case *core.DateField, *core.AutodateField:
		return "string | Date"
	case *core.GeoPointField:
		return "{ lon: number; lat: number }"
	case *core.NumberField:
		return "number"
	case *core.RelationField:
		if f.MaxSelect > 1 {
			return "string[]"
		} else {
			return "string"
		}
	case *core.FileField:
		if f.MaxSelect > 1 {
			return "string[]"
		} else {
			return "string"
		}
	case *core.SelectField:
		options := strings.Join(quoteAll(f.Values), " | ")
		if f.MaxSelect > 1 {
			return fmt.Sprintf("(%s)[]", options)
		} else {
			return options
		}
	}
	return ""
}

func getGenericArgs(collection *core.Collection, withDefault bool) []string {
	args := []string{}
	for _, name := range collection.Fields.FieldNames() {
		field := collection.Fields.GetByName(name)
		if field.Type() == "json" {
			generic := getGenericName(name)
			if withDefault {
				generic += " = unknown"
			}
			args = append(args, generic)
		}
	}
	return args
}

func formatGenericArgs(args ...string) string {
	if len(args) == 0 {
		return ""
	}
	return fmt.Sprintf("<%s>", strings.Join(args, ", "))
}

func getGenericName(name string) string {
	return "T" + capitalize(name)
}

func (gen *TypescriptGenerator) writeCustomSendRoutes(routes []Route) {
	gen.write("export const CustomSendRoutes = {\n")
	i := 0
	for i < len(routes) {
		path := routes[i].Path
		methods := []string{routes[i].Method}
		for i+1 < len(routes) && path == routes[i+1].Path {
			methods = append(methods, routes[i+1].Method)
			i++
		}
		gen.writeIndent("%s: [%s],\n", quote(path), strings.Join(quoteAll(methods), ", "))
		i++
	}
	gen.write("} as const;\n")
	gen.write("export type CustomSendRoutes = keyof typeof CustomSendRoutes;\n")
	gen.write("export type CustomSendRoutesOrAny = CustomSendRoutes | (string & {});\n\n")
}

type RouterGroup = router.RouterGroup[*core.RequestEvent]
type Route = router.Route[*core.RequestEvent]

func getCustomSendRoutes(app core.App) ([]Route, error) {
	router, err := apis.NewRouter(app)
	if err != nil {
		return nil, err
	}
	event := new(core.ServeEvent)
	event.App = app
	event.Router = router
	baseRoutes := getRouterGroupRoutes(router.RouterGroup)
	if err := app.OnServe().Trigger(event); err != nil {
		return nil, err
	}
	customRoutes := getRouterGroupRoutes(router.RouterGroup)[len(baseRoutes):]
	return customRoutes, nil
}

func getRouterGroupRoutes(routerGroup *RouterGroup) []Route {
	routes := []Route{}
	for _, child := range unsafeGetRouterGroupChildren(routerGroup) {
		switch c := child.(type) {
		case *RouterGroup:
			childRoutes := getRouterGroupRoutes(c)
			for _, childRoute := range childRoutes {
				childRoute.Path = c.Prefix + childRoute.Path
				routes = append(routes, childRoute)
			}
		case *Route:
			routes = append(routes, *c)
		}
	}
	return routes
}

func unsafeGetRouterGroupChildren(routerGroup *RouterGroup) []any {
	field := reflect.ValueOf(routerGroup).Elem().FieldByName("children")
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface().([]any)
}
