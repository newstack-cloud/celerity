package configui

import (
	"fmt"
	"io"
	"strings"

	"github.com/newstack-cloud/celerity/apps/cli/internal/appconfig"
)

const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

// ConfigOutput handles formatted stdout for config commands.
type ConfigOutput struct {
	writer  io.Writer
	isColor bool
}

// NewConfigOutput creates a new ConfigOutput writer.
func NewConfigOutput(writer io.Writer, isColor bool) *ConfigOutput {
	return &ConfigOutput{writer: writer, isColor: isColor}
}

// PrintSuccess prints a completed step with a green checkmark.
func (o *ConfigOutput) PrintSuccess(message string) {
	icon := o.color(colorGreen, "✓")
	fmt.Fprintf(o.writer, "  %s %s\n", icon, message)
}

// PrintError prints a failed step with a red X and error detail.
func (o *ConfigOutput) PrintError(message string, err error) {
	icon := o.color(colorRed, "✗")
	fmt.Fprintf(o.writer, "  %s %s: %v\n", icon, message, err)
}

// PrintInfo prints an informational message.
func (o *ConfigOutput) PrintInfo(message string) {
	icon := o.color(colorBlue, "●")
	fmt.Fprintf(o.writer, "  %s %s\n", icon, message)
}

// PrintWarning prints a warning message.
func (o *ConfigOutput) PrintWarning(message string) {
	icon := o.color(colorYellow, "!")
	fmt.Fprintf(o.writer, "  %s %s\n", icon, message)
}

// PrintTable prints a formatted table with headers and rows.
func (o *ConfigOutput) PrintTable(headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}

	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, col := range row {
			if i < len(widths) && len(col) > widths[i] {
				widths[i] = len(col)
			}
		}
	}

	o.printTableRow(headers, widths, true)
	sep := make([]string, len(widths))
	for i, w := range widths {
		sep[i] = strings.Repeat("─", w)
	}
	fmt.Fprintf(o.writer, "  %s\n", strings.Join(sep, "  "))
	for _, row := range rows {
		o.printTableRow(row, widths, false)
	}
}

func (o *ConfigOutput) printTableRow(
	cols []string,
	widths []int,
	isHeader bool,
) {
	parts := make([]string, len(cols))
	for i, col := range cols {
		w := widths[i]
		padded := col + strings.Repeat(" ", max(0, w-len(col)))
		if isHeader {
			padded = o.color(colorBold, padded)
		}
		parts[i] = padded
	}
	fmt.Fprintf(o.writer, "  %s\n", strings.Join(parts, "  "))
}

// PrintStoreList prints a summary table of config stores.
func (o *ConfigOutput) PrintStoreList(stores []appconfig.StoreInfo) {
	headers := []string{"Store", "Kind", "Resource"}
	rows := make([][]string, len(stores))
	for i, s := range stores {
		rows[i] = []string{s.StoreName, string(s.StoreKind), s.ResourceName}
	}
	o.PrintTable(headers, rows)
}

// PrintEntryTable prints a table of config entries.
func (o *ConfigOutput) PrintEntryTable(
	entries []appconfig.ConfigEntry,
	showSecrets bool,
) {
	headers := []string{"Key", "Value", "Type"}
	rows := make([][]string, len(entries))
	for i, e := range entries {
		value := e.Value
		if e.Sensitive && !showSecrets {
			value = "********"
		}
		entryType := "String"
		if e.Sensitive {
			entryType = "SecureString"
		}
		rows[i] = []string{e.Key, value, entryType}
	}
	o.PrintTable(headers, rows)
}

func (o *ConfigOutput) color(code string, text string) string {
	if !o.isColor {
		return text
	}
	return code + text + colorReset
}
