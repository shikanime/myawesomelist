#!/usr/bin/env nix
#! nix develop --impure --command nu

# Update workflows
print "[workflows] Updating GitHub Actions workflows..."
nu $"($env.FILE_PWD)/.github/workflows/update.nu"
    | lines
    | each { |line|
        print $"[workflows] ($line)"
    }
    | ignore
