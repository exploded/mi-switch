# Test ASCOM Alpaca Discovery
$discoveryMessage = "alpacadiscovery1"
$port = 32227
$broadcastIP = "255.255.255.255"

# Create UDP client
$udpClient = New-Object System.Net.Sockets.UdpClient
$udpClient.EnableBroadcast = $true
$udpClient.Client.ReceiveTimeout = 3000

try {
    Write-Host "Sending discovery broadcast to port $port..."
    
    # Send broadcast
    $bytes = [System.Text.Encoding]::ASCII.GetBytes($discoveryMessage)
    $endpoint = New-Object System.Net.IPEndPoint([System.Net.IPAddress]::Broadcast, $port)
    $sent = $udpClient.Send($bytes, $bytes.Length, $endpoint)
    Write-Host "Sent $sent bytes"
    
    # Wait for response
    Write-Host "Waiting for responses..."
    $remoteEP = New-Object System.Net.IPEndPoint([System.Net.IPAddress]::Any, 0)
    
    $timeout = 3
    $start = Get-Date
    while (((Get-Date) - $start).TotalSeconds -lt $timeout) {
        if ($udpClient.Available -gt 0) {
            $responseBytes = $udpClient.Receive([ref]$remoteEP)
            $response = [System.Text.Encoding]::ASCII.GetString($responseBytes)
            Write-Host "`nReceived response from $($remoteEP.Address):$($remoteEP.Port)"
            Write-Host "Response: $response"
        }
        Start-Sleep -Milliseconds 100
    }
    
    Write-Host "`nDiscovery test complete"
}
catch {
    Write-Host "Error: $_"
}
finally {
    $udpClient.Close()
}
