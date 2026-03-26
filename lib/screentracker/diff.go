package screentracker

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coder/agentapi/lib/msgfmt"
)

var diffCallCount atomic.Int64

// screenDiff compares two screen states and attempts to find latest message of the given agent type.
// It strips trailing blank lines from each screen independently, finds the longest common prefix,
// and returns the content between the prefix end and the last non-blank line of the new screen.
func screenDiff(oldScreen, newScreen string, agentType msgfmt.AgentType) string {
	oldLines := strings.Split(oldScreen, "\n")
	newLines := strings.Split(newScreen, "\n")

	// Strip trailing blank lines from both screens independently.
	oi := len(oldLines) - 1
	for oi >= 0 && strings.TrimSpace(oldLines[oi]) == "" {
		oi--
	}
	ni := len(newLines) - 1
	for ni >= 0 && strings.TrimSpace(newLines[ni]) == "" {
		ni--
	}

	// Find longest common prefix
	startOffset := 0
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
		logDiff(oldLines, newLines, oi, ni, prefixEnd, "(empty result)")
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
		logDiff(oldLines, newLines, oi, ni, prefixEnd, "(all empty)")
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
	result := strings.Join(newSectionLines[startLine:endLine+1], "\n")
	logDiff(oldLines, newLines, oi, ni, prefixEnd, fmt.Sprintf("(%d lines)", endLine-startLine+1))
	return result
}

func logDiff(oldLines, newLines []string, oi, ni, prefixEnd int, resultSummary string) {
	callNum := diffCallCount.Add(1)
	// Only log every 10th call to avoid huge files during streaming
	if callNum%10 != 1 {
		return
	}

	f, err := os.OpenFile("/tmp/screendiff.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "\n========== CALL #%d at %s ==========\n", callNum, time.Now().Format("15:04:05.000"))
	fmt.Fprintf(f, "oldLines: %d total, last non-blank at index %d\n", len(oldLines), oi)
	fmt.Fprintf(f, "newLines: %d total, last non-blank at index %d\n", len(newLines), ni)
	fmt.Fprintf(f, "prefixEnd: %d\n", prefixEnd)
	fmt.Fprintf(f, "result: %s\n", resultSummary)

	// Log the area around where prefix matching stopped
	fmt.Fprintf(f, "\n--- OLD SCREEN (lines %d to %d) ---\n", max(0, prefixEnd-2), min(oi+3, len(oldLines)))
	for i := max(0, prefixEnd-2); i < min(oi+3, len(oldLines)); i++ {
		fmt.Fprintf(f, "  old[%4d]: %q\n", i, oldLines[i])
	}

	fmt.Fprintf(f, "\n--- NEW SCREEN (lines %d to %d) ---\n", max(0, prefixEnd-2), min(ni+3, len(newLines)))
	for i := max(0, prefixEnd-2); i < min(ni+3, len(newLines)); i++ {
		fmt.Fprintf(f, "  new[%4d]: %q\n", i, newLines[i])
	}

	// Also log the first 10 and last 10 non-blank lines of each screen for context
	fmt.Fprintf(f, "\n--- OLD SCREEN first 10 non-blank ---\n")
	count := 0
	for i, line := range oldLines {
		if strings.TrimSpace(line) != "" {
			fmt.Fprintf(f, "  old[%4d]: %q\n", i, line)
			count++
			if count >= 10 {
				break
			}
		}
	}
	fmt.Fprintf(f, "\n--- NEW SCREEN first 10 non-blank ---\n")
	count = 0
	for i, line := range newLines {
		if strings.TrimSpace(line) != "" {
			fmt.Fprintf(f, "  new[%4d]: %q\n", i, line)
			count++
			if count >= 10 {
				break
			}
		}
	}
	fmt.Fprintf(f, "\n--- NEW SCREEN last 10 non-blank ---\n")
	count = 0
	for i := len(newLines) - 1; i >= 0; i-- {
		if strings.TrimSpace(newLines[i]) != "" {
			fmt.Fprintf(f, "  new[%4d]: %q\n", i, newLines[i])
			count++
			if count >= 10 {
				break
			}
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
