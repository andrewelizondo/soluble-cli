// Copyright 2020 Soluble Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package root

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/soluble-ai/soluble-cli/cmd/agent"
	"github.com/soluble-ai/soluble-cli/cmd/auth"
	"github.com/soluble-ai/soluble-cli/cmd/aws"
	"github.com/soluble-ai/soluble-cli/cmd/build"
	"github.com/soluble-ai/soluble-cli/cmd/cfnscan"
	"github.com/soluble-ai/soluble-cli/cmd/codescan"
	configcmd "github.com/soluble-ai/soluble-cli/cmd/config"
	"github.com/soluble-ai/soluble-cli/cmd/depscan"
	"github.com/soluble-ai/soluble-cli/cmd/downloadcmd"
	"github.com/soluble-ai/soluble-cli/cmd/imagescan"
	"github.com/soluble-ai/soluble-cli/cmd/inventorycmd"
	"github.com/soluble-ai/soluble-cli/cmd/k8sscan"
	"github.com/soluble-ai/soluble-cli/cmd/logincmd"
	modelcmd "github.com/soluble-ai/soluble-cli/cmd/model"
	"github.com/soluble-ai/soluble-cli/cmd/postcmd"
	"github.com/soluble-ai/soluble-cli/cmd/query"
	"github.com/soluble-ai/soluble-cli/cmd/secretsscan"
	"github.com/soluble-ai/soluble-cli/cmd/tfscan"
	"github.com/soluble-ai/soluble-cli/cmd/version"
	"github.com/soluble-ai/soluble-cli/pkg/blurb"
	"github.com/soluble-ai/soluble-cli/pkg/config"
	"github.com/soluble-ai/soluble-cli/pkg/exit"
	"github.com/soluble-ai/soluble-cli/pkg/log"
	"github.com/soluble-ai/soluble-cli/pkg/model"
	"github.com/soluble-ai/soluble-cli/pkg/options"
	"github.com/soluble-ai/soluble-cli/pkg/tools"
	"github.com/soluble-ai/soluble-cli/pkg/tools/autoscan"
	"github.com/soluble-ai/soluble-cli/pkg/tools/checkov"
	v "github.com/soluble-ai/soluble-cli/pkg/version"
	"github.com/spf13/cobra"
)

var (
	profile    string
	setProfile string
)

func Command() *cobra.Command {
	// rootCmd represents the base command when called without any subcommands
	rootCmd := &cobra.Command{
		Use:           "soluble",
		Long:          fmt.Sprintf(`Soluble CLI version %s built %s`, v.Version, v.BuildTime),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if f, _ := cmd.Flags().GetBool("no-color"); f {
				color.NoColor = true
			}
			if f, _ := cmd.Flags().GetBool("force-color"); f {
				color.NoColor = false
			}
			if f, _ := cmd.Flags().GetBool("quiet"); f {
				log.Level = log.Error
			}
			if f, _ := cmd.Flags().GetBool("debug"); f {
				log.Level = log.Debug
			}
			log.Debugf("Loaded configuration from {primary:%s}", config.ConfigFile)
			if setProfile != "" {
				config.SelectProfile(setProfile)
				config.Save()
			}
			if profile != "" {
				config.SelectProfile(profile)
			}
			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if config.Config.GetAPIToken() == "" {
				if cmd.Use != "version" {
					blurb.SignupBlurb(nil, "Finding {primary:soluble} useful?", "")
				}
			}
			if exit.Code != 0 {
				if exit.Func != nil {
					exit.Func()
				}
				os.Exit(exit.Code)
			}
		},
		Version: v.Version,
	}

	flags := rootCmd.PersistentFlags()
	flags.StringVar(&profile, "profile", "", "Use this configuration profile (see 'config list-profiles')")
	flags.StringVar(&setProfile, "set-profile", "", "Set the current profile to this (and save it.)")
	flags.Bool("debug", false, "Run with debug logging")
	flags.Bool("quiet", false, "Run with no logging")
	flags.Bool("no-color", false, "Disable color output")
	flags.Bool("force-color", false, "Enable color output")
	flags.BoolVar(&blurb.Blurbed, "no-blurb", false, "Don't blurb about Soluble")

	config.Load()
	addBuiltinCommands(rootCmd)
	loadModels()
	for _, model := range model.Models {
		mergeCommands(rootCmd, model.Command.GetCommand().GetCobraCommand(), model)
	}
	setupHelp(rootCmd)
	setupCompat(rootCmd)
	return rootCmd
}

func setupCompat(rootCmd *cobra.Command) {
	// temporary compatibility
	buildReport := getCommandCopy(rootCmd, []string{"build", "report"})
	buildReport.Hidden = true
	options.AddPreRunE(buildReport, func(c *cobra.Command, s []string) error {
		log.Warnf("This command has been moved to {warning:build report} and will be removed at some point")
		return nil
	})
	rootCmd.AddCommand(buildReport)
}

func getCommandCopy(rootCmd *cobra.Command, args []string) *cobra.Command {
	c, _, _ := rootCmd.Find(args)
	copy := *c
	return &copy
}

func addBuiltinCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(
		auth.Command(),
		agent.Command(),
		aws.Command(),
		configcmd.Command(),
		modelcmd.Command(),
		version.Command(),
		query.Command(),
		downloadcmd.Command(),
		postcmd.Command(),
		imagescan.Command(),
		inventorycmd.Command(),
		logincmd.Command(),
		build.Command(),
		depscan.Command(),
		k8sscan.Command(),
		tfscan.Command(),
		secretsscan.Command(),
		cfnscan.Command(),
		tools.CreateCommand(&autoscan.Tool{}),
		tools.CreateCommand(&checkov.Tool{}),
		codescan.Command(),
	)
}

func loadModels() {
	if err := model.Load(getEmbeddedModelsSource()); err != nil {
		panic(err)
	}
	if os.Getenv("SOLUBLE_DISABLE_CLI_MODELS") != "" {
		return
	}
	sources := make(chan model.Source)
	for _, loc := range config.GetModelLocations() {
		location := loc
		go func() {
			source, err := model.GetGitSource(location)
			if err != nil {
				log.Warnf("Could not get models from {info:%s}: {warning:%s}", location, err.Error())
			} else {
				sources <- source
			}
		}()
	}
	for i := len(config.GetModelLocations()); i > 0; i-- {
		source := <-sources
		if source != nil {
			err := model.Load(source)
			if err != nil {
				log.Warnf("Could not load models from {info:%s}: {warning:%s}", source.String(), err)
				continue
			}
		}
	}
}

func mergeCommands(root, cmd *cobra.Command, m *model.Model) {
	for _, existingCommand := range root.Commands() {
		if existingCommand.Use == cmd.Use {
			subCommands := cmd.Commands()
			if len(subCommands) == 0 {
				root.RemoveCommand(existingCommand)
				break
			}
			if existingCommand.Short == "" && cmd.Short != "" {
				// take short from model if present
				existingCommand.Short = cmd.Short
			}
			for _, subCommand := range subCommands {
				mergeCommands(existingCommand, subCommand, m)
			}
			return
		}
	}
	if !m.Source.IsEmbedded() {
		cmd.Short += " (" + m.Source.String() + ")"
	}
	root.AddCommand(cmd)
}
