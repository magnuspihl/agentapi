package screentracker

import (
	"strings"

	"github.com/coder/agentapi/lib/msgfmt"
)

// screenDiff compares two screen states and attempts to find latest message of the given agent type.
// It finds the longest common prefix between old and new screens (the unchanged top portion),
// then returns everything after that prefix with leading/trailing whitespace trimmed.
func screenDiff(oldScreen, newScreen string, agentType msgfmt.AgentType) string {
	oldLines := strings.Split(oldScreen, "\n")
	newLines := strings.Split(newScreen, "\n")

	// Find longest common prefix (top of screen — typically header/banner/previous conversation)
	startOffset := 0
	// Skip Opencode header lines to avoid false positives from dynamic content
	// (token count, context percentage, cost) that changes between screens.
	if len(newLines) >= 2 && agentType == msgfmt.AgentTypeOpencode {
		startOffset = 2
	}

	prefixEnd := startOffset
	limit := len(oldLines)
	if len(newLines) < limit {
		limit = len(newLines)
	}
	for prefixEnd < limit && oldLines[prefixEnd] == newLines[prefixEnd] {
		prefixEnd++
	}

	// Everything after the common prefix is potentially new content
	if prefixEnd >= len(newLines) {
		return ""
	}
	newSectionLines := newLines[prefixEnd:]

	// Remove leading and trailing lines which are empty or have only whitespace
	startLine := -1
	endLine := -1
	for i := range newSectionLines {
		if strings.TrimSpace(newSectionLines[i]) != "" {
			if startLine == -1 {
				startLine = i
			}
			endLine = i
		}
	}
	if startLine == -1 {
		return ""
	}
	return strings.Join(newSectionLines[startLine:endLine+1], "\n")
}
