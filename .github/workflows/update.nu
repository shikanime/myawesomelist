#!/usr/bin/env nix
#! nix develop --impure --command nu

def get_latest_action []: string -> string {
    gh api $"repos/($in)/tags"
    | from json
    | get name
    | where ($it =~ '^v[0-9]+$')
    | first
}

def parse_action []: nothing -> string {
    $in | split row "@" | first
}

def parse_version []: nothing -> string {
    $in | split row "@" | last
}

def update_workflow_job_step_actions []: record -> record {
    if "uses" in $in {
        let action = $in.uses | parse_action
        let version = $action | get_latest_action
        $in | update uses $"($action)@($version)"
    } else {
        $in
    }
}

def update_workflow_job_actions []: record -> record {
    $in | update steps {
        par-each { |step|
            $step | update_workflow_job_step_actions
        }
    }
}


def update_workflow_actions []: record -> record {
    $in
    | update jobs {
        items { |$name, job|
            { $name: ($job | update_workflow_job_actions) }
        }
        | into record
    }
}

print "Updating GitHub Actions workflows..."
glob $"($env.FILE_PWD)/*.{yml,yaml}"
    | par-each { |workflow|
        open $workflow
        | update_workflow_actions
        | save --force $workflow
    }
    | ignore
