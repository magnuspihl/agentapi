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

// isGarbledSeparator returns true if the line consists only of box-drawing
// dash characters (─) and whitespace — i.e. a separator line that may have
// been corrupted by VT100 rendering. It does NOT match lines with box corners
// (╭╮╰╯ etc.) which are part of welcome boxes.
func isGarbledSeparator(line string) bool {
	hasDash := false
	for _, r := range line {
		if r == ' ' || r == '\t' {
			continue
		}
		if r == '─' || r == '═' {
			hasDash = true
			continue
		}
		return false
	}
	return hasDash
}

// isCorruptedLine returns true if the line shows signs of VT100 rendering
// corruption — specifically, box-drawing characters (─) appearing between
// letters, which happens when TUI frame redraws overlap with text content.
func isCorruptedLine(line string) bool {
	runes := []rune(strings.TrimSpace(line))
	if len(runes) < 3 {
		return false
	}
	// Count transitions from letter to ─ or ─ to letter
	transitions := 0
	for i := 1; i < len(runes); i++ {
		prev := runes[i-1]
		curr := runes[i]
		if (isLetter(prev) && curr == '─') || (prev == '─' && isLetter(curr)) {
			transitions++
		}
	}
	// 3+ transitions strongly indicates corruption
	return transitions >= 3
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
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

	// Strip leading noise: garbled separators and lines before the first ●
	for len(lines) > 0 && isGarbledSeparator(lines[0]) {
		lines = lines[1:]
	}
	// If the first line doesn't start with ● but a later one does,
	// strip leading VT100 corruption lines
	if len(lines) > 0 && !strings.Contains(lines[0], "●") && !strings.Contains(lines[0], "⏺") && !strings.Contains(lines[0], "╭") {
		for i, line := range lines {
			if strings.Contains(line, "●") || strings.Contains(line, "⏺") {
				lines = lines[i:]
				break
			}
		}
	}

	// Strip trailing lines that are pure TUI artifacts (separators, borders)
	for len(lines) > 0 && isClaudeTUIArtifact(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}
	// Strip trailing lines that are VT100 corruption artifacts
	for len(lines) > 0 {
		line := lines[len(lines)-1]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || isGarbledSeparator(line) || isCorruptedLine(line) {
			lines = lines[:len(lines)-1]
			continue
		}
		break
	}

	// Strip │ frame borders from line edges
	lines = stripClaudeFrameBorders(lines)

	// Remove mid-content TUI artifacts and welcome box fragments
	lines = removeInlineArtifacts(lines)

	return strings.Join(lines, "\n")
}

// removeInlineArtifacts removes lines from the middle of the message that
// are clearly TUI rendering artifacts rather than content. Only strips
// artifacts that appear after the first agent response marker (● or ⏺),
// preserving welcome box formatting in the initial message.
func removeInlineArtifacts(lines []string) []string {
	result := make([]string, 0, len(lines))
	pastFirstMarker := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !pastFirstMarker && (strings.Contains(line, "●") || strings.Contains(line, "⏺")) {
			pastFirstMarker = true
		}
		if pastFirstMarker {
			// Remove lines that are purely TUI box-drawing characters
			if isClaudeTUIArtifact(line) {
				continue
			}
			// Remove Claude welcome box art characters
			if isWelcomeBoxArt(trimmed) {
				continue
			}
		}
		result = append(result, line)
	}
	return result
}

// isWelcomeBoxArt returns true if the line contains only Claude welcome
// box ASCII art characters (block elements used in the logo).
func isWelcomeBoxArt(line string) bool {
	if line == "" {
		return false
	}
	for _, r := range line {
		if r == ' ' || r == '\t' {
			continue
		}
		switch r {
		case '▐', '▛', '█', '▜', '▌', '▝', '▘', '▀', '▗', '▖', '▞', '▟', '▙', '▚':
			continue
		default:
			return false
		}
	}
	return true
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
