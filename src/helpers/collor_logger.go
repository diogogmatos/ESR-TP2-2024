package helpers

import "github.com/fatih/color"

var ErrorLogger = color.New(color.BgRed)
var WarnLogger = color.New(color.BgYellow).Add(color.FgBlack)
var ReaderLogger = color.New(color.FgCyan)
var NormalLogger = color.New(color.FgWhite)
var OkLogger = color.New(color.FgGreen)
