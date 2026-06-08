package loop

import "testing"

func TestParseTaskResult_CleanJSON(t *testing.T) {
	input := `{"review_approved": true, "review_issues": "", "pr_url": "https://github.com/agentra-ai/agentra/pull/42", "pr_number": 42, "branch_name": "loop/issue-1-0"}`
	got := parseTaskResult([]byte(input))
	if got == nil {
		t.Fatal("parseTaskResult returned nil for clean JSON")
	}
	if got.ReviewApproved == nil || !*got.ReviewApproved {
		t.Errorf("ReviewApproved = %v, want true", got.ReviewApproved)
	}
	if got.PRURL != "https://github.com/agentra-ai/agentra/pull/42" {
		t.Errorf("PRURL = %q", got.PRURL)
	}
	if got.PRNumber == nil || *got.PRNumber != 42 {
		t.Errorf("PRNumber = %v, want 42", got.PRNumber)
	}
	if got.BranchName != "loop/issue-1-0" {
		t.Errorf("BranchName = %q", got.BranchName)
	}
}

func TestParseTaskResult_WrappedInMarkdownFences(t *testing.T) {
	input := "```json\n" +
		`{"review_approved": true, "review_issues": "", "pr_url": "https://github.com/agentra-ai/agentra/pull/7", "pr_number": 7, "branch_name": "loop/issue-2-0"}` +
		"\n```"
	got := parseTaskResult([]byte(input))
	if got == nil {
		t.Fatal("parseTaskResult returned nil for fenced JSON")
	}
	if got.ReviewApproved == nil || !*got.ReviewApproved {
		t.Errorf("ReviewApproved = %v, want true", got.ReviewApproved)
	}
	if got.PRURL != "https://github.com/agentra-ai/agentra/pull/7" {
		t.Errorf("PRURL = %q", got.PRURL)
	}
	if got.PRNumber == nil || *got.PRNumber != 7 {
		t.Errorf("PRNumber = %v, want 7", got.PRNumber)
	}
}

func TestParseTaskResult_PreambleAndPostamble(t *testing.T) {
	input := "The review verdict is:\n" +
		`{"review_approved": true, "pr_url": "https://github.com/agentra-ai/agentra/pull/9", "pr_number": 9, "branch_name": "loop/issue-3-0"}` +
		"\nEnd."
	got := parseTaskResult([]byte(input))
	if got == nil {
		t.Fatal("parseTaskResult returned nil for JSON with preamble/postamble")
	}
	if got.ReviewApproved == nil || !*got.ReviewApproved {
		t.Errorf("ReviewApproved = %v, want true", got.ReviewApproved)
	}
	if got.BranchName != "loop/issue-3-0" {
		t.Errorf("BranchName = %q", got.BranchName)
	}
}

func TestParseTaskResult_EmptyInput(t *testing.T) {
	if got := parseTaskResult(nil); got != nil {
		t.Errorf("parseTaskResult(nil) = %+v, want nil", got)
	}
	if got := parseTaskResult([]byte("")); got != nil {
		t.Errorf("parseTaskResult(\"\") = %+v, want nil", got)
	}
}

func TestParseTaskResult_NoJSONAtAll(t *testing.T) {
	input := "some prose with no braces"
	if got := parseTaskResult([]byte(input)); got != nil {
		t.Errorf("parseTaskResult(no JSON) = %+v, want nil", got)
	}
}

func TestParseTaskResult_MalformedJSON(t *testing.T) {
	input := "{not valid json"
	if got := parseTaskResult([]byte(input)); got != nil {
		t.Errorf("parseTaskResult(malformed) = %+v, want nil", got)
	}
}

func TestParseTaskResult_NestedJSONInStringField(t *testing.T) {
	input := `{"comment": "has { and } inside", "review_approved": true}`
	got := parseTaskResult([]byte(input))
	if got == nil {
		t.Fatal("parseTaskResult returned nil for JSON with braces in string field")
	}
	if got.ReviewApproved == nil || !*got.ReviewApproved {
		t.Errorf("ReviewApproved = %v, want true", got.ReviewApproved)
	}
}
