package main

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/agentra-ai/agentra/server/internal/eval/seed"
)

func init() {
	// Wire the headless-mode answer lookup so `agentra eval run` works without
	// a daemon. Safe to call in all CLI subcommands; is a no-op on subsequent calls.
	seed.RegisterLookup()
}

var evalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Run the Agentra-Eval benchmark suite (Issue #13)",
}

var evalRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute the golden dataset in headless smoke mode",
	Long: `Scores the v0 golden dataset (20 cases) against canned answers.
No daemon, no GitHub access required.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cases := seed.DefaultCases
		var report struct {
			Cases  []evalCaseResult `json:"cases"`
			Total  int              `json:"total"`
			Passed int              `json:"passed"`
			Failed int              `json:"failed"`
			Score  float64          `json:"score"`
		}
		report.Total = len(cases)

		for _, c := range cases {
			output := seed.LookupAnswer(c.Slug)
			score := scoreCase(c.ExpectedTest, output)
			cr := evalCaseResult{
				Slug:     c.Slug,
				Category: c.Category,
				Score:    score,
				Passed:   score >= 0.5,
			}
			if cr.Passed {
				report.Passed++
			} else {
				report.Failed++
			}
			report.Cases = append(report.Cases, cr)
		}
		if report.Total > 0 {
			report.Score = float64(report.Passed) / float64(report.Total) * 100.0
		}

		b, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(b))
		return nil
	},
}

type evalCaseResult struct {
	Slug     string  `json:"slug"`
	Category string  `json:"category"`
	Score    float64 `json:"score"`
	Passed   bool    `json:"passed"`
}

func scoreCase(expected, output string) float64 {
	if expected == "" {
		return 1.0
	}
	matched, err := regexp.MatchString(expected, output)
	if err != nil || !matched {
		return 0
	}
	return 1.0
}

func init() {
	evalCmd.AddCommand(evalRunCmd)
	rootCmd.AddCommand(evalCmd)
}
