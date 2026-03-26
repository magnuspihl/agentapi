package screentracker

import (
	"strings"

	"github.com/coder/agentapi/lib/msgfmt"
)

// screenDiff compares two screen states and attempts to find latest message of the given agent type.
// It strips trailing blank lines from each screen independently, finds the longest common prefix,
// and returns the content between the prefix end and the last non-blank line of the new screen.
func screenDiff(oldScreen, newScreen string, agentType msgfmt.AgentType) string {
	// For Claude, use a response-marker-based extraction instead of prefix
	// matching. Claude's TUI renders agent responses starting with ● and
	// input boxes with ❯ between ─── separators. This approach is immune
	// to VT100 corruption that breaks prefix matching on tall terminals.
	if agentType == msgfmt.AgentTypeClaude {
		return claudeScreenDiff(oldScreen, newScreen)
	}

	return genericScreenDiff(oldScreen, newScreen, agentType)
}

// claudeScreenDiff extracts the latest Claude agent response by finding
// the last ● response block before any input box (❯ prompt area).
func claudeScreenDiff(oldScreen, newScreen string) string {
	newLines := strings.Split(newScreen, "\n")

	// Find the end of content (strip trailing blank lines)
	ni := len(newLines) - 1
	for ni >= 0 && strings.TrimSpace(newLines[ni]) == "" {
		ni--
	}
	if ni < 0 {
		return ""
	}

	// Find the last input box (❯ between ─── separators) searching from bottom
	// This marks where the agent response ends and the input prompt begins.
	inputBoxStart := -1
	for i := ni; i >= 2; i-- {
		if strings.Contains(newLines[i], "❯") || strings.TrimSpace(newLines[i]) == ">" {
			// Check if surrounded by separator lines (within 1-2 lines)
			hasSepAbove := false
			hasSepBelow := false
			for j := max(0, i-2); j < i; j++ {
				if hasManyDashes(newLines[j]) {
					hasSepAbove = true
					break
				}
			}
			for j := i + 1; j <= min(ni, i+2); j++ {
				if hasManyDashes(newLines[j]) {
					hasSepBelow = true
					break
				}
			}
			if hasSepAbove && hasSepBelow {
				// Found input box — find the separator above it
				for j := i - 1; j >= max(0, i-2); j-- {
					if hasManyDashes(newLines[j]) {
						inputBoxStart = j
						break
					}
				}
				break
			} else if hasSepAbove && strings.TrimSpace(newLines[i]) == ">" {
				// Older ───/> pattern without bottom separator
				for j := i - 1; j >= max(0, i-2); j-- {
					if hasManyDashes(newLines[j]) {
						inputBoxStart = j
						break
					}
				}
				break
			}
		}
	}

	// Determine the end of the response area
	responseEnd := ni
	if inputBoxStart != -1 {
		responseEnd = inputBoxStart - 1
		// Strip blank/whitespace lines above input box
		for responseEnd >= 0 && strings.TrimSpace(newLines[responseEnd]) == "" {
			responseEnd--
		}
	}
	if responseEnd < 0 {
		return ""
	}

	// Now find the start of the latest response by searching backward
	// for the last ● that begins a response block.
	// We also need the prefix diff to know where new content starts.
	oldLines := strings.Split(oldScreen, "\n")
	oi := len(oldLines) - 1
	for oi >= 0 && strings.TrimSpace(oldLines[oi]) == "" {
		oi--
	}

	// Find prefix using standard matching first
	prefixEnd := 0
	oldLimit := oi + 1
	newLimit := responseEnd + 1
	for prefixEnd < oldLimit && prefixEnd < newLimit && oldLines[prefixEnd] == newLines[prefixEnd] {
		prefixEnd++
	}

	// If prefix matching gives a reasonable result, use it directly.
	// This handles both short and streaming responses well.
	if prefixEnd <= responseEnd {
		sectionSize := responseEnd - prefixEnd + 1
		if sectionSize <= 80 {
			result := newLines[prefixEnd : responseEnd+1]
			return trimWhitespaceLines(result)
		}
	} else {
		// prefixEnd > responseEnd means screens are identical up to the response area
		return ""
	}

	// Prefix matching returned too much content (>80 lines) — fall back to
	// finding the last ● response marker as the start. Only search within
	// the diff section (from prefixEnd onward) to avoid finding old markers.
	responseStart := prefixEnd
	for i := responseEnd; i >= prefixEnd; i-- {
		if strings.Contains(newLines[i], "●") || strings.Contains(newLines[i], "⏺") {
			responseStart = i
			break
		}
	}

	result := newLines[responseStart : responseEnd+1]
	return trimWhitespaceLines(result)
}

// hasManyDashes returns true if the line contains at least 10 box-drawing
// dash characters (─), handling garbled separators from VT100 corruption.
func hasManyDashes(line string) bool {
	count := 0
	for _, r := range line {
		if r == '─' {
			count++
			if count >= 10 {
				return true
			}
		}
	}
	return false
}

// trimWhitespaceLines strips leading/trailing whitespace-only lines and
// returns the joined result.
func trimWhitespaceLines(lines []string) string {
	start := 0
	end := len(lines) - 1
	for start <= end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end >= start && strings.TrimSpace(lines[end]) == "" {
		end--
	}
	if start > end {
		return ""
	}
	return strings.Join(lines[start:end+1], "\n")
}

func genericScreenDiff(oldScreen, newScreen string, agentType msgfmt.AgentType) string {
	oldLines := strings.Split(oldScreen, "\n")
	newLines := strings.Split(newScreen, "\n")

	// Strip trailing blank lines from both screens independently.
	// Only blank lines are stripped — this avoids false suffix anchoring on non-blank
	// content that shifts position during streaming (when content scrolls).
	oi := len(oldLines) - 1
	for oi >= 0 && strings.TrimSpace(oldLines[oi]) == "" {
		oi--
	}
	ni := len(newLines) - 1
	for ni >= 0 && strings.TrimSpace(newLines[ni]) == "" {
		ni--
	}

	// Find longest common prefix (top of screen — typically header/previous conversation)
	startOffset := 0
	// Skip Opencode header lines to avoid false positives from dynamic content
	// (token count, context percentage, cost) that changes between screens.
	if len(newLines) >= 2 && agentType == msgfmt.AgentTypeOpencode {
		startOffset = 2
	}

	prefixEnd := startOffset
	oldPrefixLimit := oi + 1
	newPrefixLimit := ni + 1
	for prefixEnd < oldPrefixLimit && prefixEnd < newPrefixLimit && oldLines[prefixEnd] == newLines[prefixEnd] {
		prefixEnd++
	}

	// The new content is between prefixEnd and ni (inclusive)
	if prefixEnd > ni {
		return ""
	}
	newSectionLines := newLines[prefixEnd : ni+1]

	// Check if the section is all whitespace
	allEmpty := true
	for _, line := range newSectionLines {
		if strings.TrimSpace(line) != "" {
			allEmpty = false
			break
		}
	}
	if allEmpty {
		return ""
	}

	// Remove leading and trailing lines which are empty or have only whitespace
	startLine := 0
	endLine := len(newSectionLines) - 1
	for i := range newSectionLines {
		if strings.TrimSpace(newSectionLines[i]) != "" {
			startLine = i
			break
		}
	}
	for i := len(newSectionLines) - 1; i >= 0; i-- {
		if strings.TrimSpace(newSectionLines[i]) != "" {
			endLine = i
			break
		}
	}
	return strings.Join(newSectionLines[startLine:endLine+1], "\n")
}
