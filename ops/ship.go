package ops

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/talkersoft/hive-deck/internal/checkout"
	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/github"
	"github.com/talkersoft/hive-deck/internal/namegen"
	"github.com/talkersoft/hive-deck/internal/resolve"
	"github.com/talkersoft/hive-deck/internal/ship"
	"github.com/talkersoft/hive-deck/internal/teardown"
)

// openPRsInBrowser retries ListOpenPRs up to 5 times (3s apart) to handle
// GitHub API indexing delay after a fresh ship, then opens each URL.
func openPRsInBrowser(l *config.Loaded) {
	browserCmd := "open"
	if runtime.GOOS != "darwin" {
		browserCmd = "xdg-open"
	}
	plan, err := resolve.Build(l)
	if err != nil {
		return
	}
	for attempt := 0; attempt < 5; attempt++ {
		var urls []string
		for _, repo := range plan.Repos {
			for _, pr := range github.ListOpenPRs(repo.Dest) {
				urls = append(urls, pr.URL)
			}
		}
		if len(urls) > 0 {
			for _, u := range urls {
				exec.Command(browserCmd, u).Start() //nolint:errcheck
			}
			return
		}
		time.Sleep(3 * time.Second)
	}
}

func RunShip(in ShipInput) (string, error) {
	if in.Title == "" {
		return "", fmt.Errorf("title is required")
	}
	l, err := config.LoadDeck(in.Deck)
	if err != nil {
		return "", err
	}

	autoMerge := l.Setup.Ship.PRMode == config.PRModeAutoMerge || l.Setup.Ship.PRMode == ""

	if err := ship.Run(l, ship.Options{
		Message:             in.Message,
		Title:               in.Title,
		Body:                in.Body,
		DeleteBranchOnMerge: l.Setup.Ship.DeleteBranchOnMerge,
		AutoMerge:           autoMerge,
	}); err != nil {
		return "", err
	}

	if l.Setup.Ship.OpenBrowser {
		openPRsInBrowser(l)
	}

	switch l.Setup.Ship.PRMode {
	case config.PRModeAutoMerge, "":
		if l.Setup.Ship.TeardownOnShip {
			return "", teardown.Run(l, teardown.Options{RequireMergedPR: false})
		}
		nextBranch := namegen.Generate()
		return "", checkout.Run(l, checkout.Options{
			RequireMergedPR: false,
			NextBranch:      nextBranch,
		})

	case config.PRModeAwaitMerge:
		return "\nawait_merge: PRs opened — merge them, then run hv next to transition", nil

	case config.PRModeManual:
		return "\nmerge-gate: PRs opened — merge them, then run hv next to transition", nil

	default:
		return "", fmt.Errorf("unknown pr_mode %q", l.Setup.Ship.PRMode)
	}
}
