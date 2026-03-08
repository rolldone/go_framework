package main

import (
	"syscall"

	"go_framework/internal/console"
)

func main() {
	// Ensure console commands create group-writable files by default
	syscall.Umask(0o002)
	// To register additional plugins and their console commands:
	// import "go_framework/plugins/myplugin"
	// console.RegisterAdditionalPlugins([]plugins.Plugin{myplugin.New()})
	console.Execute()
}
