package dbquery

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

type resetItem struct {
	label string
	path  string
}

func runReset(cfg Config) error {
	items := resetItemsForTarget(cfg)
	if len(items) == 0 {
		return fmt.Errorf("unsupported reset target %q", cfg.ResetTarget)
	}

	if !cfg.Yes {
		ok, err := confirmReset(cfg.ResetTarget, items)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(os.Stderr, "Reset cancelled.")
			return nil
		}
	}

	removed := make([]string, 0, len(items))
	missing := make([]string, 0, len(items))
	wouldRemove := make([]string, 0, len(items))

	for _, item := range items {
		if cfg.DryRun {
			if _, err := os.Stat(item.path); err == nil {
				wouldRemove = append(wouldRemove, fmt.Sprintf("%s (%s)", item.label, item.path))
				continue
			} else if errors.Is(err, os.ErrNotExist) {
				missing = append(missing, fmt.Sprintf("%s (%s)", item.label, item.path))
				continue
			} else {
				return fmt.Errorf("check %s: %w", item.label, err)
			}
		}

		err := os.Remove(item.path)
		if err == nil {
			removed = append(removed, fmt.Sprintf("%s (%s)", item.label, item.path))
			continue
		}
		if errors.Is(err, os.ErrNotExist) {
			missing = append(missing, fmt.Sprintf("%s (%s)", item.label, item.path))
			continue
		}
		return fmt.Errorf("remove %s: %w", item.label, err)
	}

	if cfg.DryRun {
		fmt.Println("Dry run (no files deleted).")
		if len(wouldRemove) == 0 {
			fmt.Println("Nothing would be removed.")
		} else {
			fmt.Println("Would remove:")
			for _, line := range wouldRemove {
				fmt.Printf("- %s\n", line)
			}
		}
	} else if len(removed) == 0 {
		fmt.Println("Nothing was removed.")
	} else {
		fmt.Println("Removed:")
		for _, line := range removed {
			fmt.Printf("- %s\n", line)
		}
	}

	if len(missing) > 0 {
		fmt.Println("Already missing:")
		for _, line := range missing {
			fmt.Printf("- %s\n", line)
		}
	}

	return nil
}

func resetItemsForTarget(cfg Config) []resetItem {
	switch cfg.ResetTarget {
	case "config":
		return []resetItem{{label: "config", path: cfg.SettingsFile}}
	case "profile":
		return []resetItem{{label: "profile", path: cfg.ProfilesFile}}
	case "all":
		return []resetItem{
			{label: "config", path: cfg.SettingsFile},
			{label: "profile", path: cfg.ProfilesFile},
			{label: "history", path: cfg.HistoryFile},
		}
	default:
		return nil
	}
}

func confirmReset(target string, items []resetItem) (bool, error) {
	fmt.Fprintf(os.Stderr, "Reset %s? This will delete:\n", target)
	for _, item := range items {
		fmt.Fprintf(os.Stderr, "- %s (%s)\n", item.label, item.path)
	}
	fmt.Fprint(os.Stderr, "Continue? [Y/n]: ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) {
			if strings.TrimSpace(line) == "" {
				return false, nil
			}
		} else if errors.Is(err, os.ErrClosed) {
			return false, nil
		} else if len(strings.TrimSpace(line)) == 0 {
			return false, err
		}
	}

	answer := strings.ToLower(strings.TrimSpace(line))
	switch answer {
	case "", "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return false, nil
	}
}
