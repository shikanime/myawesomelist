#!/usr/bin/env nix
#! nix develop --impure --command nu

let images: list<string> = (
    skaffold build --cache-artifacts=false --output={{json .}} --quiet --platform linux/amd64
    | from json
    | get builds
    | each { |build| $build.tag | split row "@" | first }
)

let container_id: string = (
    scw container container list --output json
    | from json
    | where name == "container-serene-dewdney"
    | get id
    | first
)

scw container container update $container_id registry-image=($images.0) --wait

let www_container_id: string = (
    scw container container list --output json
    | from json
    | where name == "container-pensive-mcclintock"
    | get id
    | first
)

scw container container update $www_container_id registry-image=($images.1) --wait
