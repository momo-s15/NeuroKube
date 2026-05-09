# Sends a synthetic Alertmanager-style firing alert to the brain (KubePodOOMKilled).
# Requires: kubectl context kind-neurokube, nginx-victim pod in default, brain in neurokube.
$ErrorActionPreference = "Stop"
$bin = Join-Path $env:USERPROFILE "bin"
if (Test-Path $bin) { $env:PATH = "$bin;$env:PATH" }

$pod = kubectl get pods -n default -l app=nginx-victim -o jsonpath='{.items[0].metadata.name}'
if (-not $pod) { throw "No pod with label app=nginx-victim in default" }

$body = @{
  alerts = @(
    @{
      status      = "firing"
      labels      = @{
        alertname = "KubePodOOMKilled"
        namespace = "default"
        pod       = $pod
      }
      annotations = @{
        summary = "Synthetic E2E test (scripts/post-synthetic-alert.ps1)"
      }
    }
  )
} | ConvertTo-Json -Depth 8 -Compress

$pf = Start-Job -ScriptBlock {
  $env:PATH = "$using:env:USERPROFILE\bin;$env:PATH"
  kubectl port-forward svc/neurokube-brain 18083:8080 -n neurokube
}
Start-Sleep -Seconds 4
try {
  $r = Invoke-WebRequest -Uri "http://127.0.0.1:18083/alert" -Method POST -Body $body `
    -ContentType "application/json; charset=utf-8" -UseBasicParsing
  Write-Host "OK HTTP $($r.StatusCode) - check Slack and: kubectl logs -n neurokube -l app=neurokube-brain --tail=30"
}
finally {
  Stop-Job $pf -ErrorAction SilentlyContinue
  Remove-Job $pf -ErrorAction SilentlyContinue
}
