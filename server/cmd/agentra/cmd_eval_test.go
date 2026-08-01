package main

import (
	"testing"

	"github.com/agentra-ai/agentra/server/internal/eval/seed"
)

func TestEvalFixtureContractIsValidAndNotAQualityGate(t *testing.T) {
	report := validateEvalFixtures(seed.FixtureCases, seed.FixtureAnswers)

	if !report.Valid {
		t.Fatalf("fixture report is invalid: %+v", report)
	}
	if report.QualityGate {
		t.Fatal("fixture validation must not claim to be a quality gate")
	}
	if report.Mode != "fixture_contract" {
		t.Fatalf("mode = %q, want fixture_contract", report.Mode)
	}
	if report.Total != 25 {
		t.Fatalf("total = %d, want 25", report.Total)
	}
	if report.ValidPatterns != report.Total || report.FixtureMatches != report.Total || report.Failures != 0 {
		t.Fatalf("unexpected fixture counts: %+v", report)
	}
}

func TestEvalFixtureContractRejectsInvalidOrMisleadingFixtures(t *testing.T) {
	cases := []seed.FixtureCase{
		{Slug: "empty-pattern", ExpectedTest: ""},
		{Slug: "invalid-pattern", ExpectedTest: "["},
		{Slug: "missing-answer", ExpectedTest: "expected"},
		{Slug: "mismatch", ExpectedTest: "expected"},
	}
	answers := map[string]string{
		"empty-pattern":   "anything",
		"invalid-pattern": "anything",
		"mismatch":        "different",
	}

	report := validateEvalFixtures(cases, answers)
	if report.Valid {
		t.Fatal("invalid fixtures must fail validation")
	}
	if report.Failures != len(cases) {
		t.Fatalf("failures = %d, want %d", report.Failures, len(cases))
	}
}

func TestEvalCommandDoesNotExposeFakeBenchmarkRun(t *testing.T) {
	commands := evalCmd.Commands()
	if len(commands) != 1 {
		t.Fatalf("eval subcommand count = %d, want 1", len(commands))
	}
	if commands[0].Name() != "validate-fixtures" {
		t.Fatalf("eval subcommand = %q, want validate-fixtures", commands[0].Name())
	}
}
