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
		fmt.Fprintln(out, "Stripped AI attribution trailer from commit message.")
	}
	subject := ""
	if len(lines) > 0 {
		subject = lines[0]
	}
	if mergeRe.MatchString(subject) {
		return nil
	}
	if subject == "" {
		fmt.Fprintln(out, "ERROR: Commit message cannot be empty.")
		fmt.Fprintln(out, "Expected format: type(scope): description")
		fmt.Fprintln(out, "Types: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert, break, bump")
		return fmt.Errorf("hook: commit-msg rejected empty subject in %s", msgPath)
	}
	length := utf8.RuneCountInString(subject)
	if !conventionalRe.MatchString(subject) {
		fmt.Fprintln(out, "ERROR: Commit message does not follow Conventional Commits format.")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Expected format: type(scope): description")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Type (required): feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert, break, bump")
		fmt.Fprintln(out, "Scope (optional): component or module in parentheses")
		fmt.Fprintf(out, "Description: brief description (subject capped at %d characters)\n", maxSubjectLength)
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Examples:")
		fmt.Fprintln(out, "  feat(api): add user authentication endpoint")
		fmt.Fprintln(out, "  fix(auth): resolve token expiration issue")
		fmt.Fprintln(out, "  docs: update installation instructions")
		fmt.Fprintln(out, "  test(auth): add unit tests for login flow")
		fmt.Fprintln(out)
		fmt.Fprintf(out, "Your subject: %s\n", subject)
		fmt.Fprintf(out, "Subject length: %d characters (max %d)\n", length, maxSubjectLength)
		return fmt.Errorf("hook: commit-msg rejected %s", subject)
	}
	if length > maxSubjectLength {
		fmt.Fprintf(out, "ERROR: Subject exceeds %d characters.\n", maxSubjectLength)
		fmt.Fprintf(out, "Subject length: %d characters (max %d)\n", length, maxSubjectLength)
		fmt.Fprintf(out, "Your subject: %s\n", subject)
		return fmt.Errorf("hook: commit-msg subject too long in %s", msgPath)
	}
	if len(lines) > 1 && lines[1] != "" {
		fmt.Fprintln(out, "ERROR: Commit message must have a blank line between subject and body.")
		fmt.Fprintln(out)
		fmt.Fprintf(out, "Subject: %s\n", subject)
		fmt.Fprintf(out, "Second line: %s\n", lines[1])
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Please add a blank line after the subject line.")
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
		fmt.Fprintf(out, "hook: commit-msg rewrite %s: %v\n", msgPath, err)
	}
	return true
}
