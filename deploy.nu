#!/usr/bin/env nix
#! nix develop --impure --command nu

let container_id: string = (
    scw container container list --output json
    | from json
    | where name == "container-focused-kepler"
    | get id
    | first
)

let image: string = (
    skaffold build --cache-artifacts=false --output={{json .}} --quiet
    | from json
    | get builds.0.tag
    | split row "@"
    | first
)

scw container container update $container_id registry-image=($image) --wait
