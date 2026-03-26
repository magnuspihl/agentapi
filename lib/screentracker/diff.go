package screentracker

import (
	"strings"

	"github.com/coder/agentapi/lib/msgfmt"
)

// screenDiff compares two screen states and attempts to find latest message of the given agent type.
// It strips trailing blank lines from each screen independently, finds the longest common prefix,
// and returns the content between the prefix end and the last non-blank line of the new screen.
func screenDiff(oldScreen, newScreen string, agentType msgfmt.AgentType) string {
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

	// For Claude, attempt to skip over VT100-corrupted lines that break
	// exact prefix matching. Look ahead up to 5 lines to find where the
	// screens align again, allowing the prefix to extend further.
	if agentType == msgfmt.AgentTypeClaude {
		maxSkip := 5
		for prefixEnd < oldPrefixLimit && prefixEnd < newPrefixLimit {
			found := false
			for skip := 1; skip <= maxSkip && prefixEnd+skip < oldPrefixLimit && prefixEnd+skip < newPrefixLimit; skip++ {
				if oldLines[prefixEnd+skip] == newLines[prefixEnd+skip] {
					// Lines match again after skipping — advance past the gap
					prefixEnd += skip
					// Continue matching exactly from here
					for prefixEnd < oldPrefixLimit && prefixEnd < newPrefixLimit && oldLines[prefixEnd] == newLines[prefixEnd] {
						prefixEnd++
					}
					found = true
					break
				}
			}
			if !found {
				break
			}
		}
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
