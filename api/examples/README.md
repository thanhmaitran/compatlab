# API examples (PowerShell)

The full description is in `api/openapi/compatlab.yaml`. Start the API with `go run ./apps/api/cmd/server` (set `JWT_SECRET` first).

```powershell
$base = "http://127.0.0.1:8080/api/v1"
$token = (Invoke-RestMethod -Method Post "$base/auth/register" -ContentType application/json `
  -Body (@{organization="Acme"; email="admin@acme.io"; password="correct-horse-1"} | ConvertTo-Json)).token
$h = @{ Authorization = "Bearer $token" }

$project = Invoke-RestMethod -Method Post "$base/projects" -Headers $h -ContentType application/json -Body (@{name="Users API"} | ConvertTo-Json)
foreach ($v in @(@("v1","testdata/compatible/users-v1.yaml"), @("v2","testdata/breaking/users-v2-breaking.yaml"))) {
  $body = @{version_label=$v[0]; filename=(Split-Path $v[1] -Leaf); content=(Get-Content $v[1] -Raw)} | ConvertTo-Json
  Invoke-RestMethod -Method Post "$base/projects/$($project.id)/specifications" -Headers $h -ContentType application/json -Body $body
}
$versions = Invoke-RestMethod "$base/projects/$($project.id)/versions" -Headers $h
$analysis = Invoke-RestMethod -Method Post "$base/projects/$($project.id)/analyze" -Headers $h -ContentType application/json `
  -Body (@{base_version_id=$versions[0].id; target_version_id=$versions[1].id} | ConvertTo-Json)
$analysis.backward
```
