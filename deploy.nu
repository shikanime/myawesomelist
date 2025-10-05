#!/usr/bin/env nix
#! nix develop --impure --command nu

let container_id: string = (
    scw container container list --output json
    | from json
    | where name == "container-focused-kepler"
    | get id
    | first
)

let image_tag: string = (
    skaffold build --cache-artifacts=false --output={{json .}} --quiet
    | from json
    | get builds.0.tag
    | split row "@"
    | first
    | split row ":"
    | last
)

scw container container update $container_id tags.0=($image_tag) --wait
