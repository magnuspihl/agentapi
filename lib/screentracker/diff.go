package screentracker

import (
	"strings"

	"github.com/coder/agentapi/lib/msgfmt"
)

// screenDiff compares two screen states and attempts to find latest message of the given agent type.
// It uses positional comparison: strip common trailing blank lines, find the common prefix,
// and return the content in between.
func screenDiff(oldScreen, newScreen string, agentType msgfmt.AgentType) string {
	oldLines := strings.Split(oldScreen, "\n")
	newLines := strings.Split(newScreen, "\n")

	// Strip trailing blank lines from both screens.
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
