#!/usr/bin/env bash
# If run by bash/zsh: re-exec under PowerShell (pwsh)
command -v pwsh >/dev/null 2>&1 && exec pwsh -NoProfile -File "$0" -- "$@"
echo "pwsh not found on PATH"; exit 127
# Everything below must be valid (or ignored) PowerShell

# PowerShell starts here (the lines above are comments to PowerShell because they start with '#')
param([Parameter(ValueFromRemainingArguments=$true)][string[]]$Args)

Write-Host "Hello from PowerShell $($PSVersionTable.PSVersion)"
# ...rest of your PowerShell...
