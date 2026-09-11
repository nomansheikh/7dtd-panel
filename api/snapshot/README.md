# Vendored OpenAPI snapshot

Captured verbatim from a live 7 Days to Die dedicated server so the build
is reproducible and needs no network access.

- Source: `GET /api/openapi/openapi.yaml` plus the 28 sub-specs it
  `$ref`s, each from `GET /api/OpenAPI/{name}.openapi.yaml`
- Server: V 3.2.0 (b10)
- Mods: TFP_Harmony, Allocs_Commands 30, Allocs_Core 38, Allocs_Webinterface 52
- Captured: 2026-09-11

Do not hand-edit these files. They are the upstream truth; fixes for
upstream bugs live as explicit, addressed patches in
`tools/specbundle`, so a refresh surfaces any that no longer apply.

Refresh with:

    go run ./tools/specbundle -from http://<host>:<port> -out api/
