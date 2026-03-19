package cmd

import (
	"io"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
)

// cyberTable creates a cyberpunk-themed borderless table matching the project's
// devops theme (cyan headers, magenta accents, dim separators, no borders).
func cyberTable(w io.Writer) *tablewriter.Table {
	colorCfg := renderer.ColorizedConfig{
		Borders: tw.Border{
			Left:   tw.Off,
			Right:  tw.Off,
			Top:    tw.Off,
			Bottom: tw.Off,
		},
		Settings: tw.Settings{
			Separators: tw.Separators{
				BetweenColumns: tw.On,
				BetweenRows:    tw.Off,
				ShowHeader:     tw.Off,
				ShowFooter:     tw.Off,
			},
			Lines: tw.Lines{
				ShowTop:        tw.Off,
				ShowBottom:     tw.Off,
				ShowHeaderLine: tw.On,
				ShowFooterLine: tw.Off,
			},
		},
		Header: renderer.Tint{
			FG: renderer.Colors{color.FgHiCyan, color.Bold},
		},
		Column: renderer.Tint{
			FG: renderer.Colors{color.FgHiWhite},
		},
		Border: renderer.Tint{
			FG: renderer.Colors{color.FgHiBlack},
		},
		Separator: renderer.Tint{
			FG: renderer.Colors{color.FgHiBlack},
		},
		Symbols: tw.NewSymbols(tw.StyleLight),
	}

	return tablewriter.NewTable(w,
		tablewriter.WithRenderer(renderer.NewColorized(colorCfg)),
		tablewriter.WithConfig(tablewriter.Config{
			Header: tw.CellConfig{
				Alignment: tw.CellAlignment{Global: tw.AlignLeft},
				Formatting: tw.CellFormatting{
					AutoFormat: tw.On,
				},
			},
			Row: tw.CellConfig{
				Alignment: tw.CellAlignment{Global: tw.AlignLeft},
			},
		}),
	)
}
