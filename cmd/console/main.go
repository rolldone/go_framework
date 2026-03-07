package main

import (
	"syscall"

	"go_framework/internal/console"
	"go_framework/internal/plugins"
	auth "go_framework/plugins/auth"
	billing "go_framework/plugins/billing"
	node "go_framework/plugins/node"
)

func main() {
	// Ensure console commands create group-writable files by default
	syscall.Umask(0o002)
	// To register additional plugins and their console commands, use:
	// console.RegisterAdditionalPlugins([]plugins.Plugin{plugin.New()})
	console.RegisterAdditionalPlugins([]plugins.Plugin{auth.New(), billing.New(), node.New()})
	console.Execute()
}
