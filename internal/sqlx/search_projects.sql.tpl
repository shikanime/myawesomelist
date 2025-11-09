SELECT
    COALESCE(p.name,'') AS name,
    COALESCE(p.description,'') AS description,
    p.repo_hostname AS hostname,
    p.repo_owner AS owner,
    p.repo_repo AS repo
FROM projects p
JOIN categories cat ON cat.id = p.category_id
JOIN collections c ON c.id = cat.collection_id
WHERE
    (LOWER(p.name) LIKE $1 OR LOWER(p.description) LIKE $1)
{{- if .RepoPositions }}
    AND (
        {{- range $i, $pos := .RepoPositions }}
        {{- if $i }} OR {{ end }}
        (c.hostname = ${{ $pos.HostnamePosition }} AND c.owner = ${{ $pos.OwnerPosition }} AND c.repo = ${{ $pos.RepoPosition }})
        {{- end }}
    )
{{- end }}
LIMIT ${{ .LimitPosition }}