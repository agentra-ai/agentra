package stages

import (
	"fmt"
	"strings"

	"github.com/agentra-ai/agentra/server/internal/loop/prompts"
)

// loadPrompt reads prompts/<name>.md from the embedded FS and substitutes
// the {{.Foo}} template variables from the given TaskRef. The variable
// set is fixed and small:
//
//	{{.IssueID}}      - TaskRef.IssueID
//	{{.IssueTitle}}   - TaskRef.IssueTitle
//	{{.Branch}}       - TaskRef.Branch (may be empty in early iterations)
//	{{.Iteration}}    - TaskRef.Iteration as a decimal integer
//	{{.WorkDir}}      - TaskRef.WorkDir
//
// Unknown template variables are left untouched — the substitute step is
// a plain string replace, not a text/template execution, so a typo in a
// future template does not produce a runtime panic.
func loadPrompt(name string, task TaskRef) (string, error) {
	raw, err := prompts.FS.ReadFile(name + ".md")
	if err != nil {
		return "", fmt.Errorf("load prompt %s: %w", name, err)
	}
	s := string(raw)
	s = strings.ReplaceAll(s, "{{.IssueID}}", task.IssueID)
	s = strings.ReplaceAll(s, "{{.IssueTitle}}", task.IssueTitle)
	s = strings.ReplaceAll(s, "{{.Branch}}", task.Branch)
	s = strings.ReplaceAll(s, "{{.Iteration}}", fmt.Sprintf("%d", task.Iteration))
	s = strings.ReplaceAll(s, "{{.WorkDir}}", task.WorkDir)
	return s, nil
}
