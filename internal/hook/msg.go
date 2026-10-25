package hook

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

const maxSubjectLength = 100

func report(out io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(out, format+"\n", a...)
}

var attributionRe = regexp.MustCompile(`^(Co-Authored-By:.+(Claude|Anthropic|Copilot|Cursor|Gemini|Codeium|Windsurf|Cline|Aider)|Generated with .*(Claude|Copilot|Cursor))`)

var conventionalRe = regexp.MustCompile(`^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert|break|bump)(\(.+\))?: .+$`)

var mergeRe = regexp.MustCompile(`^(Merge|Revert)`)

func CommitMsg(msgPath string, out io.Writer) error {
	b, err := os.ReadFile(msgPath)
	if err != nil {
		return fmt.Errorf("hook: commit-msg read %s: %w", msgPath, err)
	}
	lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	if stripAttribution(&lines, msgPath, out) {
		report(out, "Stripped AI attribution trailer from commit message.")
	}
	subject := ""
	if len(lines) > 0 {
		subject = lines[0]
	}
	if mergeRe.MatchString(subject) {
		return nil
	}
	if subject == "" {
		report(out, "ERROR: Commit message cannot be empty.")
		report(out, "Expected format: type(scope): description")
		report(out, "Types: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert, break, bump")
		return fmt.Errorf("hook: commit-msg rejected empty subject in %s", msgPath)
	}
	length := utf8.RuneCountInString(subject)
	if !conventionalRe.MatchString(subject) {
		report(out, "ERROR: Commit message does not follow Conventional Commits format.")
		report(out, "")
		report(out, "Expected format: type(scope): description")
		report(out, "")
		report(out, "Type (required): feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert, break, bump")
		report(out, "Scope (optional): component or module in parentheses")
		report(out, "Description: brief description (subject capped at %d characters)", maxSubjectLength)
		report(out, "")
		report(out, "Examples:")
		report(out, "  feat(api): add user authentication endpoint")
		report(out, "  fix(auth): resolve token expiration issue")
		report(out, "  docs: update installation instructions")
		report(out, "  test(auth): add unit tests for login flow")
		report(out, "")
		report(out, "Your subject: %s", subject)
		report(out, "Subject length: %d characters (max %d)", length, maxSubjectLength)
		return fmt.Errorf("hook: commit-msg rejected %s", subject)
	}
	if length > maxSubjectLength {
		report(out, "ERROR: Subject exceeds %d characters.", maxSubjectLength)
		report(out, "Subject length: %d characters (max %d)", length, maxSubjectLength)
		report(out, "Your subject: %s", subject)
		return fmt.Errorf("hook: commit-msg subject too long in %s", msgPath)
	}
	if len(lines) > 1 && lines[1] != "" {
		report(out, "ERROR: Commit message must have a blank line between subject and body.")
		report(out, "")
		report(out, "Subject: %s", subject)
		report(out, "Second line: %s", lines[1])
		report(out, "")
		report(out, "Please add a blank line after the subject line.")
		return fmt.Errorf("hook: commit-msg missing blank line in %s", msgPath)
	}
	return nil
}

func stripAttribution(lines *[]string, msgPath string, out io.Writer) bool {
	stripped := false
	kept := make([]string, 0, len(*lines))
	for _, l := range *lines {
		if attributionRe.MatchString(l) {
			stripped = true
			continue
		}
		kept = append(kept, l)
	}
	if !stripped {
		return false
	}
	*lines = kept
	if err := os.WriteFile(msgPath, []byte(strings.Join(kept, "\n")+"\n"), 0o644); err != nil {
		report(out, "hook: commit-msg rewrite %s: %v", msgPath, err)
	}
	return true
}
