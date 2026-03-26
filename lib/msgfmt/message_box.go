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
func findFirstInputBox(lines []string) int {
	// Search for the three-line slim input box pattern: ───/❯/───
	for i := 0; i < len(lines)-2; i++ {
		if strings.Contains(lines[i], "───────────────") &&
			strings.Contains(lines[i+1], "❯") &&
			strings.Contains(lines[i+2], "───────────────") {
			return i
		}
	}
	// Fallback: search for ───/> pattern (older Claude Code versions)
	for i := 0; i < len(lines)-1; i++ {
		if strings.Contains(lines[i], "───────────────") &&
			strings.TrimSpace(lines[i+1]) == ">" {
			return i
		}
	}
	return -1
}

// isClaudeTUIArtifact returns true if the line consists only of TUI
// box-drawing characters and whitespace (e.g. "│", "─────", "─╯").
func isClaudeTUIArtifact(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false // empty lines are handled by trimEmptyLines
	}
	for _, r := range trimmed {
		switch r {
		case '│', '─', '╭', '╮', '╰', '╯', '┌', '┐', '└', '┘', '║', '═':
			continue
		default:
			return false
		}
	}
	return true
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
