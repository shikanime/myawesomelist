#!/usr/bin/env nix
#! nix develop --impure --command nu
use std/log

# Function to run templ generation in watch mode
def watch_templ [] {
	(
		templ generate
			--watch
			--proxy="http://localhost:8080"
			--open-browser=false
	)
}

# Function to run air for Go file changes detection
def watch_go [] {
	mkdir .air/tmp/go
	(
		air
			--build.cmd $"go build -o cmd/myawesomelist/myawesomelist~ ($env.FILE_PWD)/cmd/myawesomelist"
			--build.bin $"($env.FILE_PWD)/cmd/myawesomelist/myawesomelist~"
			--build.delay 100
			--build.exclude_dir node_modules
			--build.include_ext go
			--build.stop_on_error false
			--misc.clean_on_exit true
			--tmp_dir $"($env.FILE_PWD)/.air/tmp/go"
	)
}

# Function to run tailwindcss in watch mode
def watch_tailwind [] {
	(
		npx tailwindcss
			-i $"($env.FILE_PWD)/cmd/myawesomelist/app/assets/app.css"
			-o $"($env.FILE_PWD)/cmd/myawesomelist/app/public/styles.css"
			--minify
			--watch
	)
}

# Function to watch for assets changes and reload browser
def watch_assets [] {
	mkdir .air/tmp/assets
	(
		air
			--build.cmd $"templ generate --notify-proxy ($env.FILE_PWD)/cmd/myawesomelist/app/assets"
			--build.bin true
			--build.delay 100
			--build.include_dir $"($env.FILE_PWD)/cmd/myawesomelist/app/assets"
			--build.include_ext css
			--tmp_dir $"($env.FILE_PWD)/.air/tmp/assets"
	)
}

# Main function that runs all development processes in parallel
def main [] {
	[
		{ watch_templ }
		{ watch_go }
		{ watch_tailwind }
		{ watch_assets }
	]
	| par-each { |func| do $func }
}
