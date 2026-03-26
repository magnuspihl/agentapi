package msgfmt

import (
	"strings"
)

// Usually something like
// ───────────────
// >
// ───────────────
// Used by Claude Code, Goose, and Aider.
func findGreaterThanMessageBox(lines []string) int {
	for i := len(lines) - 1; i >= max(len(lines)-6, 0); i-- {
		if strings.Contains(lines[i], ">") {
			if i > 0 && strings.Contains(lines[i-1], "───────────────") {
				return i - 1
			}
			return i
		}
	}
	return -1
}

// Usually something like
// ───────────────
// |
// ───────────────
func findGenericSlimMessageBox(lines []string) int {
	for i := len(lines) - 3; i >= max(len(lines)-9, 0); i-- {
		if strings.Contains(lines[i], "───────────────") &&
			(strings.Contains(lines[i+1], "|") || strings.Contains(lines[i+1], "│") || strings.Contains(lines[i+1], "❯")) &&
			strings.Contains(lines[i+2], "───────────────") {
			return i
		}
	}
	return -1
}

func removeMessageBox(msg string) string {
	lines := strings.Split(msg, "\n")

	messageBoxStartIdx := findGreaterThanMessageBox(lines)
	if messageBoxStartIdx == -1 {
		messageBoxStartIdx = findGenericSlimMessageBox(lines)
	}

	if messageBoxStartIdx != -1 {
		lines = lines[:messageBoxStartIdx]
	}

	return strings.Join(lines, "\n")
}

func removeCodexMessageBox(msg string) string {
	lines := strings.Split(msg, "\n")
	if len(lines) >= 3 && strings.Contains(lines[len(lines)-3], "›") {
		idx := len(lines) - 3
		lines = append(lines[:idx], lines[idx+2])
	}
	return strings.Join(lines, "\n")
}

func removeOpencodeMessageBox(msg string) string {
	lines := strings.Split(msg, "\n")
	//
	//  ┃
	//  ┃
	//  ┃
	//  ┃  Build  Anthropic Claude Sonnet 4
	//  ╹▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀
	//                                tab switch agent  ctrl+p commands
	//
	for i := len(lines) - 1; i >= 4; i-- {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "╹▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀") {
			lines = lines[:i-4]
			break
		}
	}
	return strings.Join(lines, "\n")
}

// findFirstInputBox searches from the top of the message for the first
// occurrence of a Claude Code input box pattern:
//
//	───────────────
//	❯ (or > with ─── above)
//	───────────────
//
// Returns the index of the first separator line, or -1 if not found.
// This is used to strip the input prompt and any trailing conversation
// history that appears in the screen diff when the terminal is very tall
// and prefix-based diffing captures the entire screen.
// hasManyDashes returns true if the line contains at least minCount
// box-drawing dash characters (─). This handles garbled separator lines
// where VT100 rendering corruption breaks consecutive dashes with spaces.
func hasManyDashes(line string, minCount int) bool {
	count := 0
	for _, r := range line {
		if r == '─' {
			count++
			if count >= minCount {
				return true
			}
		}
	}
	return false
}

func findFirstInputBox(lines []string) int {
	// Search for the three-line slim input box pattern: ───/❯/───
	// Uses hasManyDashes to handle garbled separators from VT100 corruption
	for i := 0; i < len(lines)-2; i++ {
		if hasManyDashes(lines[i], 10) &&
			strings.Contains(lines[i+1], "❯") &&
			hasManyDashes(lines[i+2], 10) {
			return i
		}
	}
	// Fallback: search for ───/> pattern (older Claude Code versions)
	for i := 0; i < len(lines)-1; i++ {
		if hasManyDashes(lines[i], 10) &&
			strings.TrimSpace(lines[i+1]) == ">" {
			return i
		}
	}
	return -1
}

// isClaudeTUIArtifact returns true if the line consists only of TUI
// box-drawing characters and whitespace (e.g. "│", "  │  ", "─╯").
func isClaudeTUIArtifact(line string) bool {
	hasBoxChar := false
	for _, r := range line {
		if r == ' ' || r == '\t' {
			continue
		}
		switch r {
		case '│', '─', '╭', '╮', '╰', '╯', '┌', '┐', '└', '┘', '║', '═':
			hasBoxChar = true
		default:
			return false
		}
	}
	return hasBoxChar
}

// stripClaudeFrameBorders removes the │ TUI frame borders that Claude Code
// renders at the left and right edges of the terminal. These are visual
// frame characters at column 0 and the last column, not meaningful content.
func stripClaudeFrameBorders(lines []string) []string {
	pipe := "│"
	for i, line := range lines {
		// Strip right-edge │ border (and trailing whitespace before it)
		trimRight := strings.TrimRight(line, " \t")
		if strings.HasSuffix(trimRight, pipe) {
			line = strings.TrimRight(trimRight[:len(trimRight)-len(pipe)], " \t")
		}
		// Strip left-edge │ border followed by a space (frame + content padding)
		trimLeft := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimLeft, pipe+" ") {
			line = trimLeft[len(pipe)+1:]
		} else if trimLeft == pipe {
			line = ""
		}
		lines[i] = line
	}
	return lines
}

func removeClaudeMessageBox(msg string) string {
	lines := strings.Split(msg, "\n")

	// Search from the top for the first input box. This handles the case
	// where the screen diff captures the entire terminal (very tall terminals)
	// and the input box appears near the top, after the latest response.
	idx := findFirstInputBox(lines)
	if idx != -1 {
		lines = lines[:idx]
	} else {
		// Fall back to the generic bottom-up search
		messageBoxStartIdx := findGreaterThanMessageBox(lines)
		if messageBoxStartIdx == -1 {
			messageBoxStartIdx = findGenericSlimMessageBox(lines)
		}
		if messageBoxStartIdx != -1 {
			lines = lines[:messageBoxStartIdx]
		}
	}

	// Strip trailing lines that are pure TUI artifacts (separators, borders)
	for len(lines) > 0 && isClaudeTUIArtifact(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}

	// Strip │ frame borders from line edges
	lines = stripClaudeFrameBorders(lines)

	return strings.Join(lines, "\n")
}

func removeAmpMessageBox(msg string) string {
	lines := strings.Split(msg, "\n")
	msgBoxEndFound := false
	msgBoxStartIdx := len(lines)
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if !msgBoxEndFound && strings.HasPrefix(line, "╰") && strings.HasSuffix(line, "╯") {
			msgBoxEndFound = true
		}
		if msgBoxEndFound && strings.HasPrefix(line, "╭") && strings.HasSuffix(line, "╮") {
			msgBoxStartIdx = i
			break
		}
	}
	formattedMsg := strings.Join(lines[:msgBoxStartIdx], "\n")
	if len(formattedMsg) == 0 {
		return "Welcome to Amp"
	}
	return formattedMsg
}
