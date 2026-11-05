$ErrorActionPreference = 'Stop'
function Invoke-LabCompose {
    & docker compose -f lab/compose.yaml @args
    if ($LASTEXITCODE -ne 0) { throw "Docker Compose failed: $args" }
}
function Wait-Route([string]$Prefix, [string]$State, [string]$Family = 'ipv4') {
    $label = if ($State -eq 'notfound') { 'not found' } else { $State }
    for ($attempt = 0; $attempt -lt 90; $attempt++) {
        $output = Invoke-LabCompose exec -T router vtysh -c "show bgp $Family unicast $Prefix"
        if ($output -match "rpki validation-state: $([regex]::Escape($label))\b") {
            Write-Output "PASS: $Prefix $State"
            return
        }
        Start-Sleep -Seconds 2
    }
    Invoke-LabCompose exec -T router vtysh -c 'show rpki cache-connection' -c 'show rpki prefix-table' -c "show bgp $Family unicast"
    throw "$Prefix never reached $State"
}
Invoke-LabCompose up -d --build
$before = Invoke-LabCompose ps -q mini-rpki
Invoke-LabCompose exec -T mini-rpki mini-rpki clear
Invoke-LabCompose exec -T mini-rpki mini-rpki add 192.0.2.0/24 --asn 65001
Invoke-LabCompose exec -T mini-rpki mini-rpki add 2001:db8:100::/48 --asn 65001
Wait-Route '192.0.2.0/24' valid
Wait-Route '192.0.2.0/25' invalid
Wait-Route '198.51.100.0/24' notfound
Wait-Route '2001:db8:100::/48' valid ipv6
Invoke-LabCompose exec -T router vtysh -c 'show rpki cache-connection' -c 'show rpki prefix-table'
Invoke-LabCompose exec -T mini-rpki mini-rpki remove 192.0.2.0/24 --asn 65001
Invoke-LabCompose exec -T mini-rpki mini-rpki add 192.0.2.0/24 --asn 65002
Wait-Route '192.0.2.0/24' invalid
Invoke-LabCompose exec -T mini-rpki mini-rpki remove 192.0.2.0/24 --asn 65002
Wait-Route '192.0.2.0/24' notfound
Invoke-LabCompose exec -T mini-rpki mini-rpki add 192.0.2.0/24 --asn 65001
Wait-Route '192.0.2.0/24' valid
if ($before -ne (Invoke-LabCompose ps -q mini-rpki)) { throw 'Cache container was replaced' }
Write-Output 'PASS: FRR route states and live updates without container replacement'
