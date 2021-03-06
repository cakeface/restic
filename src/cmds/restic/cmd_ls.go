package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"restic"
	"restic/errors"
	"restic/repository"
)

var cmdLs = &cobra.Command{
	Use:   "ls [flags] snapshot-ID",
	Short: "list files in a snapshot",
	Long: `
The "ls" command allows listing files and directories in a snapshot.

The special snapshot-ID "latest" can be used to list files and directories of the latest snapshot in the repository.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLs(globalOptions, args)
	},
}

// LsOptions collects all options for the ls command.
type LsOptions struct {
	ListLong bool
	Host     string
	Paths    []string
}

var lsOptions LsOptions

func init() {
	cmdRoot.AddCommand(cmdLs)

	flags := cmdLs.Flags()
	flags.BoolVarP(&lsOptions.ListLong, "long", "l", false, "use a long listing format showing size and mode")

	flags.StringVarP(&lsOptions.Host, "host", "H", "", `only consider snapshots for this host when the snapshot ID is "latest"`)
	flags.StringSliceVar(&lsOptions.Paths, "path", nil, "only consider snapshots which include this (absolute) `path` for snapshot ID \"latest\"")
}

func printNode(prefix string, n *restic.Node) string {
	if !lsOptions.ListLong {
		return filepath.Join(prefix, n.Name)
	}

	switch n.Type {
	case "file":
		return fmt.Sprintf("%s %5d %5d %6d %s %s",
			n.Mode, n.UID, n.GID, n.Size, n.ModTime.Format(TimeFormat), filepath.Join(prefix, n.Name))
	case "dir":
		return fmt.Sprintf("%s %5d %5d %6d %s %s",
			n.Mode|os.ModeDir, n.UID, n.GID, n.Size, n.ModTime.Format(TimeFormat), filepath.Join(prefix, n.Name))
	case "symlink":
		return fmt.Sprintf("%s %5d %5d %6d %s %s -> %s",
			n.Mode|os.ModeSymlink, n.UID, n.GID, n.Size, n.ModTime.Format(TimeFormat), filepath.Join(prefix, n.Name), n.LinkTarget)
	default:
		return fmt.Sprintf("<Node(%s) %s>", n.Type, n.Name)
	}
}

func printTree(prefix string, repo *repository.Repository, id restic.ID) error {
	tree, err := repo.LoadTree(id)
	if err != nil {
		return err
	}

	for _, entry := range tree.Nodes {
		Printf(printNode(prefix, entry) + "\n")

		if entry.Type == "dir" && entry.Subtree != nil {
			err = printTree(filepath.Join(prefix, entry.Name), repo, *entry.Subtree)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func runLs(gopts GlobalOptions, args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return errors.Fatal("no snapshot ID given")
	}

	repo, err := OpenRepository(gopts)
	if err != nil {
		return err
	}

	err = repo.LoadIndex()
	if err != nil {
		return err
	}

	snapshotIDString := args[0]
	var id restic.ID

	if snapshotIDString == "latest" {
		id, err = restic.FindLatestSnapshot(repo, lsOptions.Paths, lsOptions.Host)
		if err != nil {
			Exitf(1, "latest snapshot for criteria not found: %v Paths:%v Host:%v", err, lsOptions.Paths, lsOptions.Host)
		}
	} else {
		id, err = restic.FindSnapshot(repo, snapshotIDString)
		if err != nil {
			Exitf(1, "invalid id %q: %v", snapshotIDString, err)
		}
	}

	sn, err := restic.LoadSnapshot(repo, id)
	if err != nil {
		return err
	}

	Verbosef("snapshot of %v at %s:\n", sn.Paths, sn.Time)

	return printTree("", repo, *sn.Tree)
}
