package cmd

import (
	"github.com/spf13/cobra"

	"github.com/s-oravec/cage/internal/cage"
	"github.com/s-oravec/cage/internal/images"
	"github.com/s-oravec/cage/internal/imgstore"
)

// completeCageNames returns cage names suitable for shell completion of a
// command argument. When runningOnly is true, only cages with status=running
// are included (useful for ssh/exec/stop).
func completeCageNames(runningOnly bool) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		cages, err := cage.List()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		names := make([]string, 0, len(cages))
		for _, c := range cages {
			if runningOnly && c.Status != cage.StatusRunning {
				continue
			}
			names = append(names, c.Name)
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeImageNames returns image names (base + custom + layered refs) for
// `cage image rm/inspect`.
func completeImageNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	imgs, err := images.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	names := make([]string, 0, len(imgs))
	for _, i := range imgs {
		names = append(names, i.Name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// completeLocalRefs returns all local refs (both local and registry-qualified)
// for commands like `cage tag` (source) and `cage push` (must be registry-qualified
// to actually push, but show both so the user can spot a mistake).
func completeLocalRefs(registryOnly bool) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		refs, err := imgstore.ListRefs()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		out := make([]string, 0, len(refs))
		for _, e := range refs {
			if registryOnly && !e.Ref.IsRegistry() {
				continue
			}
			display := e.Ref.Name + ":" + e.Ref.Tag
			if e.Ref.IsRegistry() {
				display = e.Ref.Host + "/" + e.Ref.Owner + "/" + e.Ref.Name + ":" + e.Ref.Tag
			}
			out = append(out, display)
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeBaseImageNames returns canonical base distro names for flags like
// `cage pull --base`.
func completeBaseImageNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return images.ListAvailable(), cobra.ShellCompDirectiveNoFileComp
}
