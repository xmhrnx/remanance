package model

import "github.com/xmhrnx/remanance/locales"

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
	{ToolRecover, locales.T("tool.recover"), locales.T("tool.recover"), "recover"},
	{ToolSpace, locales.T("tool.disk_space"), locales.T("tool.space"), "chart"},
	{ToolDupes, locales.T("tool.duplicates"), locales.T("tool.duplicates"), "copy"},
	{ToolSync, locales.T("tool.sync"), locales.T("tool.sync"), "sync"},
	{ToolRename, locales.T("tool.rename"), locales.T("tool.rename"), "rename"},
}
