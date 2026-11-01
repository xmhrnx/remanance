package model

// Tool is one of the app's tools.
type Tool int

const (
	ToolRecover Tool = iota
	ToolSpace
	ToolDupes
	ToolSync
	ToolRename
)

// ToolInfo is how a tool is shown in the navigation.
type ToolInfo struct {
	Tool  Tool
	Label string
	Short string // on a phone's tab bar
	Icon  string
}

// Tools are the tools, in the order the navigation lists them.
var Tools = []ToolInfo{
	{ToolRecover, "Recover", "Recover", "recover"},
	{ToolSpace, "Disk space", "Space", "chart"},
	{ToolDupes, "Duplicates", "Duplicates", "copy"},
	{ToolSync, "Sync", "Sync", "sync"},
	{ToolRename, "Rename", "Rename", "rename"},
}
