package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/twpayne/chezmoi/internal/chezmoi"
	vfs "github.com/twpayne/go-vfs"
)

var addCmd = &cobra.Command{
	Use:      "add targets...",
	Aliases:  []string{"manage"},
	Args:     cobra.MinimumNArgs(1),
	Short:    "Add an existing file, directory, or symlink to the source state",
	Long:     mustGetLongHelp("add"),
	Example:  getExample("add"),
	PreRunE:  config.ensureNoError,
	RunE:     config.runAddCmd,
	PostRunE: config.autoCommitAndAutoPush,
}

type addCmdConfig struct {
	recursive bool
	prompt    bool
	options   chezmoi.AddOptions
}

func init() {
	rootCmd.AddCommand(addCmd)

	persistentFlags := addCmd.PersistentFlags()
	persistentFlags.BoolVarP(&config.add.options.Empty, "empty", "e", false, "add empty files")
	persistentFlags.BoolVar(&config.add.options.Encrypt, "encrypt", false, "encrypt files")
	persistentFlags.BoolVarP(&config.add.options.Exact, "exact", "x", false, "add directories exactly")
	persistentFlags.BoolVarP(&config.add.prompt, "prompt", "p", false, "prompt before adding")
	persistentFlags.BoolVarP(&config.add.recursive, "recursive", "r", false, "recurse in to subdirectories")
	persistentFlags.BoolVarP(&config.add.options.Template, "template", "T", false, "add files as templates")
	persistentFlags.BoolVarP(&config.add.options.AutoTemplate, "autotemplate", "a", false, "auto generate the template when adding files as templates")
}

func (c *Config) runAddCmd(cmd *cobra.Command, args []string) (err error) {
	ts, err := c.getTargetState(nil)
	if err != nil {
		return err
	}
	if err := c.ensureSourceDirectory(); err != nil {
		return err
	}
	destDirPrefix := filepath.FromSlash(ts.DestDir + "/")
	var quit int // quit is an int with a unique address
	defer func() {
		if r := recover(); r != nil {
			if p, ok := r.(*int); ok && p == &quit {
				err = nil
			} else {
				panic(r)
			}
		}
	}()
	for _, arg := range args {
		path, err := filepath.Abs(arg)
		if err != nil {
			return err
		}
		if c.add.recursive {
			if err := vfs.Walk(c.fs, path, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if ts.TargetIgnore.Match(strings.TrimPrefix(path, destDirPrefix)) {
					fmt.Fprintf(c.Stderr(), "warning: skipping file ignored by .chezmoiignore: %s\n", path)
					return nil
				}
				if c.add.prompt {
					choice, err := c.prompt(fmt.Sprintf("Add %s", path), "ynqa")
					if err != nil {
						return err
					}
					switch choice {
					case 'y':
					case 'n':
						return nil
					case 'q':
						panic(&quit) // abort vfs.Walk by panicking
					case 'a':
						c.add.prompt = false
					}
				}
				return ts.Add(c.fs, c.add.options, path, info, c.Follow, c.mutator)
			}); err != nil {
				return err
			}
		} else {
			if ts.TargetIgnore.Match(strings.TrimPrefix(path, destDirPrefix)) {
				fmt.Fprintf(c.Stderr(), "warning: skipping file ignored by .chezmoiignore: %s\n", path)
				continue
			}
			if c.add.prompt {
				choice, err := c.prompt(fmt.Sprintf("Add %s", path), "ynqa")
				if err != nil {
					return err
				}
				switch choice {
				case 'y':
				case 'n':
					continue
				case 'q':
					return nil
				case 'a':
					c.add.prompt = false
				}
			}
			if err := ts.Add(c.fs, c.add.options, path, nil, c.Follow, c.mutator); err != nil {
				return err
			}
		}
	}
	return nil
}
