param([switch]$CheckSource)
$ErrorActionPreference='Stop'
$taskRoot=Split-Path $PSScriptRoot -Parent
$taskManifest=Get-Content -Raw (Join-Path $taskRoot 'migration/source-manifest.json') | ConvertFrom-Json
$taskChanged=@('README.md','Makefile','.gitignore','start-cotriage.sh')
$taskCorePattern='^(log-collector|edge-infer|router-controller|cloud-gateway|server)/'
$taskChecked=0
foreach($entry in $taskManifest.files){
 if($CheckSource){$path=Join-Path $taskManifest.source $entry.path}
 else{
  if($entry.path -in $taskChanged){continue}
  if($entry.path -match $taskCorePattern -and ($entry.path -match '\.go$|/(go\.mod|go\.sum|Dockerfile|Makefile|README\.md)$')){continue}
  if($entry.path -match '^charts/(log-collector|edge-infer|router-controller|cloud-gateway|server)/(deployment|daemonset)\.yaml$'){continue}
  if($entry.path -eq 'sop/router-controller运行SOP.md'){continue}
  $path=Join-Path $taskRoot $entry.path
 }
 if(!(Test-Path -LiteralPath $path)){throw "Missing preserved file: $($entry.path)"}
 if((Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant() -ne $entry.sha256){throw "Hash changed: $($entry.path)"}
 $taskChecked++
}
[PSCustomObject]@{scope=$(if($CheckSource){'original source'}else{'preserved Nova files'});verifiedFiles=$taskChecked;sourceCommit=$taskManifest.commit}|ConvertTo-Json
