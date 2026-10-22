$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot
while (-not (Test-Path (Join-Path $root 'go.mod'))) { $root = Split-Path $root }
$vcvars = 'C:\Program Files\Microsoft Visual Studio\2022\Community\VC\Auxiliary\Build\vcvars64.bat'
$nvcc = 'C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA\v13.4\bin\nvcc.exe'
if (-not (Test-Path $vcvars)) { throw "vcvars64 not found at $vcvars" }
if (-not (Test-Path $nvcc)) { $nvcc = (Get-Command nvcc).Source }
$outDir = Join-Path $PSScriptRoot 'bin'
New-Item -ItemType Directory -Force $outDir | Out-Null
$dll = Join-Path $outDir 'nnevalcuda.dll'
$src = Join-Path $PSScriptRoot 'gpu.cu'
$cmd = "`"$vcvars`" >NUL && `"$nvcc`" -shared -O3 -o `"$dll`" `"$src`""
& cmd.exe /c $cmd
if ($LASTEXITCODE -ne 0) { throw "nvcc failed with exit $LASTEXITCODE" }
"built $dll"
