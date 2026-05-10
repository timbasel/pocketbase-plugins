// The `typegencmd` package adds a "typegen" command to the PocketBase instance. It generates
// both `RecordModel` for Typescript and `core.RecordProxy` for Go from the database schema.
package typegencmd

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"
)

type Config struct {
	GoDir          string
	TypescriptFile string
}

func MustRegister(app core.App, rootCmd *cobra.Command, config Config) {
	if err := Register(app, rootCmd, config); err != nil {
		panic(err)
	}
}

func Register(app core.App, rootCmd *cobra.Command, config Config) error {
	command := &cobra.Command{
		Use:   "typegen",
		Short: "Generates type definitions from the database schema.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if config.GoDir != "" {
				if err := GenerateGoTypes(app, config.GoDir); err != nil {
					return err
				}
			}
			if config.TypescriptFile != "" {
				if err := GenerateTypescriptTypes(app, config.TypescriptFile); err != nil {
					return err
				}
			}
			return nil
		},
	}
	if rootCmd != nil {
		rootCmd.AddCommand(command)
	}
	return nil
}
