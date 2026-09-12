param([string]$ImagePrefix='cotriage-nova',[string]$ImageTag='java25',[string]$OutputPath='')
$ErrorActionPreference='Stop'
$taskRoot=Split-Path $PSScriptRoot -Parent
$taskParts=foreach($service in @('log-collector','edge-infer','router-controller','cloud-gateway','server')){
  $name=if($service -eq 'log-collector'){'daemonset.yaml'}else{'deployment.yaml'}
  $body=[IO.File]::ReadAllText((Join-Path $taskRoot "charts/$service/$name"))
  $body=$body.Replace("cotriage-nova/${service}:java25","${ImagePrefix}/${service}:${ImageTag}")
  $body.Trim()
}
$taskYaml=($taskParts -join "`n---`n")+"`n"
if($OutputPath){[IO.File]::WriteAllText([IO.Path]::GetFullPath($OutputPath),$taskYaml,[Text.UTF8Encoding]::new($false))}else{$taskYaml}
