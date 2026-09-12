param([switch]$KeepRunning)
$ErrorActionPreference='Stop'
$taskRoot=Split-Path $PSScriptRoot -Parent
$taskCompose=Join-Path $taskRoot 'migration/compose.smoke.yaml'
$taskOutput=Join-Path $taskRoot '.migration/images'
New-Item -ItemType Directory -Force -Path $taskOutput|Out-Null
$taskServices=@('log-collector','edge-infer','router-controller','cloud-gateway','server')
Push-Location $taskRoot
try {
 docker compose -p cotriage-nova-imagecheck -f $taskCompose up -d
 if($LASTEXITCODE -ne 0){throw 'Image startup failed'}
 $taskAddress=(docker compose -p cotriage-nova-imagecheck -f $taskCompose port server 8080).Trim()
 $taskReady=$false
 for($i=0;$i -lt 45;$i++){
  try{$r=Invoke-WebRequest -Uri "http://$taskAddress/api/v1/dashboard/summary" -TimeoutSec 2;if($r.StatusCode -eq 200){$taskReady=$true;break}}catch{}
  Start-Sleep -Seconds 2
 }
 if(!$taskReady){throw 'Image API did not become ready'}
 $taskIds=@()
 $taskRecords=foreach($service in $taskServices){
  $id=(docker compose -p cotriage-nova-imagecheck -f $taskCompose ps -q $service).Trim()
  if(!$id){throw "Missing container: $service"}
  $state=docker inspect --format '{{.State.Status}}' $id
  docker logs $id *> (Join-Path $taskOutput ($service+'.log'))
  if($state -ne 'running'){throw "Container exited: $service"}
  $taskIds+=$id
  $inspection=(docker inspect $id | ConvertFrom-Json)[0]
  if($inspection.HostConfig.Memory -ne 536870912){throw "Memory limit mismatch: $service"}
  [PSCustomObject]@{service=$service;container=$id;image=$inspection.Image;status=$state;memoryLimitBytes=$inspection.HostConfig.Memory;javaToolOptions=($inspection.Config.Env|Where-Object{$_ -like 'JAVA_TOOL_OPTIONS=*'})}
 }
 $taskStats=@(docker stats --no-stream --format '{{json .}}' $taskIds|ForEach-Object{$_|ConvertFrom-Json})
 $taskApis=foreach($path in @('devices','events','edge-config','router/summary','router/state','competition-metrics')){
  $response=Invoke-WebRequest -Uri "http://$taskAddress/api/v1/$path" -TimeoutSec 8
  if($response.StatusCode -ne 200){throw "Image API failed: $path"}
  [PSCustomObject]@{path=$path;status=$response.StatusCode}
 }
 [ordered]@{checkedAt=(Get-Date).ToUniversalTime().ToString('o');containers=$taskRecords;stats=$taskStats;api=$taskApis}|ConvertTo-Json -Depth 10|Set-Content -Encoding utf8 (Join-Path $taskOutput 'result.json')
 Get-Content (Join-Path $taskOutput 'result.json')
} finally {
 if(!$KeepRunning){docker compose -p cotriage-nova-imagecheck -f $taskCompose down --volumes}
 Pop-Location
}
