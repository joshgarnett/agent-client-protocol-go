package cmd

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var filterCmd = &cobra.Command{
	Use:   "filter",
	Short: "Filter conflicting types from generated code",
	Long:  `Remove type declarations that conflict with our custom generated types.`,
	Run:   runFilter,
}

func init() {
	rootCmd.AddCommand(filterCmd)
}

func runFilter(_ *cobra.Command, _ []string) {
	if err := doFilter(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func doFilter() error {
	// Types that we generate manually and should be filtered out
	conflictingTypes := []string{
		"ContentBlock",
		"SessionUpdate",
		"ToolCallContent",
		"ToolKind",
		"ToolCallStatus",
		"PlanEntryStatus",
		"PlanEntryPriority",
		"PermissionOptionKind",
		"StopReason",
	}

	// Read the generated file
	file, err := os.Open("acp/api/types_generated.go")
	if err != nil {
		return fmt.Errorf("opening types_generated.go: %w", err)
	}
	defer file.Close()

	var filteredLines []string
	scanner := bufio.NewScanner(file)

	// Regex patterns to match type declarations and their duplicates
	typeRegex := regexp.MustCompile(`^type\s+(\w+)(?:_\d+)?\s+`)

	skipUntilNextType := false
	currentType := ""

	for scanner.Scan() {
		line := scanner.Text()

		// Check if this is a type declaration
		if matches := typeRegex.FindStringSubmatch(line); matches != nil {
			baseTypeName := matches[1]
			// Remove trailing digits and underscore (e.g., "PermissionOptionKind_1" -> "PermissionOptionKind")
			baseTypeName = regexp.MustCompile(`_\d+$`).ReplaceAllString(baseTypeName, "")

			// Check if this type conflicts with our generated types
			isConflicting := false
			for _, conflictType := range conflictingTypes {
				if baseTypeName == conflictType {
					isConflicting = true
					break
				}
			}

			if isConflicting {
				skipUntilNextType = true
				currentType = baseTypeName
				fmt.Fprintf(os.Stderr, "Filtering out conflicting type: %s\n", matches[1])
				continue
			}
			skipUntilNextType = false
			currentType = ""
		}

		// Skip lines if we're in a conflicting type declaration
		if skipUntilNextType {
			// Check if we've reached the end of the current type declaration
			// This is a simple heuristic - if we see another type declaration or certain keywords
			if strings.HasPrefix(line, "type ") && !strings.Contains(line, currentType) {
				skipUntilNextType = false
				// Process this line since it's a new type
				filteredLines = append(filteredLines, line)
			}
			// Skip this line
			continue
		}

		filteredLines = append(filteredLines, line)
	}

	if scanErr := scanner.Err(); scanErr != nil {
		return fmt.Errorf("reading file: %w", scanErr)
	}

	// Write the filtered content back to the file
	outputFile, err := os.Create("acp/api/types_generated.go")
	if err != nil {
		return fmt.Errorf("creating filtered file: %w", err)
	}
	defer outputFile.Close()

	for _, line := range filteredLines {
		if _, writeErr := outputFile.WriteString(line + "\n"); writeErr != nil {
			return fmt.Errorf("writing filtered file: %w", writeErr)
		}
	}

	fmt.Fprintf(os.Stderr, "Filtered acp/api/types_generated.go, removed %d conflicting types\n", len(conflictingTypes))
	return nil
}
