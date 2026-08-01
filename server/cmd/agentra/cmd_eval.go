package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/agentra-ai/agentra/server/internal/eval/seed"
)

var evalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Inspect the experimental evaluation fixture contract",
}

var evalValidateFixturesCmd = &cobra.Command{
	Use:   "validate-fixtures",
	Short: "Validate eval regex patterns against their deterministic fixtures",
	Long: `Validates the experimental eval dataset's regex and fixture contract.
This command does not execute an agent, measure implementation quality, persist
a benchmark run, or act as a release gate.`,
	Args: cobra.NoArgs,
	RunE: runEvalValidateFixtures,
}

type evalFixtureCaseResult struct {
	Slug           string `json:"slug"`
	Category       string `json:"category"`
	PatternValid   bool   `json:"pattern_valid"`
	FixtureDefined bool   `json:"fixture_defined"`
	FixtureMatched bool   `json:"fixture_matched"`
	Error          string `json:"error,omitempty"`
}

type evalFixtureReport struct {
	Mode           string                  `json:"mode"`
	QualityGate    bool                    `json:"quality_gate"`
	Valid          bool                    `json:"valid"`
	Total          int                     `json:"total"`
	ValidPatterns  int                     `json:"valid_patterns"`
	FixtureMatches int                     `json:"fixture_matches"`
	Failures       int                     `json:"failures"`
	Cases          []evalFixtureCaseResult `json:"cases"`
}

func runEvalValidateFixtures(cmd *cobra.Command, _ []string) error {
	report := validateEvalFixtures(seed.FixtureCases, seed.FixtureAnswers)
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode fixture report: %w", err)
	}
	if _, err := fmt.Fprintln(cmd.OutOrStdout(), string(encoded)); err != nil {
		return fmt.Errorf("write fixture report: %w", err)
	}
	if !report.Valid {
		return fmt.Errorf("eval fixture contract has %d invalid case(s)", report.Failures)
	}
	return nil
}

func validateEvalFixtures(cases []seed.FixtureCase, answers map[string]string) evalFixtureReport {
	report := evalFixtureReport{
		Mode:        "fixture_contract",
		QualityGate: false,
		Total:       len(cases),
		Cases:       make([]evalFixtureCaseResult, 0, len(cases)),
	}

	for _, fixtureCase := range cases {
		result := evalFixtureCaseResult{
			Slug:     fixtureCase.Slug,
			Category: fixtureCase.Category,
		}
		var problems []string

		if fixtureCase.ExpectedTest == "" {
			problems = append(problems, "expected test is empty")
		} else if pattern, err := regexp.Compile(fixtureCase.ExpectedTest); err != nil {
			problems = append(problems, "invalid expected-test regex: "+err.Error())
		} else {
			result.PatternValid = true
			report.ValidPatterns++

			answer, exists := answers[fixtureCase.Slug]
			result.FixtureDefined = exists
			if !exists {
				problems = append(problems, "fixture answer is missing")
			} else if !pattern.MatchString(answer) {
				problems = append(problems, "fixture answer does not match expected-test regex")
			} else {
				result.FixtureMatched = true
				report.FixtureMatches++
			}
		}

		if !result.PatternValid {
			_, result.FixtureDefined = answers[fixtureCase.Slug]
		}
		if len(problems) > 0 {
			result.Error = strings.Join(problems, "; ")
			report.Failures++
		}
		report.Cases = append(report.Cases, result)
	}

	report.Valid = report.Total > 0 && report.Failures == 0
	return report
}

func init() {
	evalCmd.AddCommand(evalValidateFixturesCmd)
	rootCmd.AddCommand(evalCmd)
}
