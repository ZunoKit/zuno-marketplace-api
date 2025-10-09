# Certificate Generation Script for mTLS (Windows PowerShell)
# This script generates CA, server, and client certificates for secure gRPC communication

param(
    [string]$OpenSSLPath = "openssl",  # Path to OpenSSL executable
    [int]$DaysValid = 3650,             # 10 years
    [string]$Country = "US",
    [string]$State = "California", 
    [string]$City = "San Francisco",
    [string]$Organization = "Zuno Marketplace",
    [string]$OrganizationalUnit = "Engineering"
)

# Set error action preference
$ErrorActionPreference = "Stop"

# Get script directory
$CertDir = Split-Path -Parent $MyInvocation.MyCommand.Path

Write-Host "Starting certificate generation for mTLS..." -ForegroundColor Green

# Function to generate certificate
function Generate-Certificate {
    param(
        [string]$Name,
        [string]$CN,
        [string]$Type  # "ca", "server", or "client"
    )
    
    Write-Host "Generating $Type certificate for $Name..." -ForegroundColor Yellow
    
    # Generate private key
    & $OpenSSLPath genpkey -algorithm RSA -out "$CertDir\$Name.key" -pkeyopt rsa_keygen_bits:4096
    
    if ($Type -eq "ca") {
        # Generate CA certificate
        $subject = "/C=$Country/ST=$State/L=$City/O=$Organization/OU=$OrganizationalUnit/CN=$CN"
        & $OpenSSLPath req -new -x509 -days $DaysValid -key "$CertDir\$Name.key" `
            -out "$CertDir\$Name.crt" `
            -subj $subject
    }
    else {
        # Generate certificate signing request
        $subject = "/C=$Country/ST=$State/L=$City/O=$Organization/OU=$OrganizationalUnit/CN=$CN"
        & $OpenSSLPath req -new -key "$CertDir\$Name.key" -out "$CertDir\$Name.csr" `
            -subj $subject
        
        # Create extensions file
        $extFile = "$CertDir\$Name.ext"
        if ($Type -eq "server") {
            @"
subjectAltName = DNS:$CN,DNS:localhost,DNS:*.zuno-marketplace.local,IP:127.0.0.1,IP:::1
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
"@ | Set-Content -Path $extFile
        }
        else {
            # Client certificate extensions
            @"
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth
"@ | Set-Content -Path $extFile
        }
        
        # Sign certificate with CA
        & $OpenSSLPath x509 -req -in "$CertDir\$Name.csr" `
            -CA "$CertDir\ca.crt" -CAkey "$CertDir\ca.key" -CAcreateserial `
            -out "$CertDir\$Name.crt" -days $DaysValid `
            -extfile $extFile
        
        # Clean up temporary files
        Remove-Item -Path "$CertDir\$Name.csr" -ErrorAction SilentlyContinue
        Remove-Item -Path $extFile -ErrorAction SilentlyContinue
    }
    
    Write-Host "✓ Generated $Type certificate for $Name" -ForegroundColor Green
}

# Check if OpenSSL is available
try {
    $version = & $OpenSSLPath version 2>&1
    Write-Host "Using OpenSSL: $version" -ForegroundColor Cyan
}
catch {
    Write-Host "OpenSSL not found. Please install OpenSSL or specify path with -OpenSSLPath parameter" -ForegroundColor Red
    Write-Host "You can download OpenSSL from: https://slproweb.com/products/Win32OpenSSL.html" -ForegroundColor Yellow
    exit 1
}

# Step 1: Generate CA certificate
Generate-Certificate -Name "ca" -CN "Zuno Marketplace CA" -Type "ca"

# Step 2: Generate server certificates for each service
$services = @(
    @{Name="auth-service"; Port=50051},
    @{Name="user-service"; Port=50052},
    @{Name="wallet-service"; Port=50053},
    @{Name="orchestrator-service"; Port=50054},
    @{Name="media-service"; Port=50055},
    @{Name="chain-registry-service"; Port=50056},
    @{Name="catalog-service"; Port=50057},
    @{Name="indexer-service"; Port=50058},
    @{Name="graphql-gateway"; Port=8081}
)

foreach ($service in $services) {
    Generate-Certificate -Name $service.Name -CN "$($service.Name).zuno-marketplace.local" -Type "server"
}

# Step 3: Generate client certificates
Generate-Certificate -Name "graphql-gateway-client" -CN "graphql-gateway-client" -Type "client"
Generate-Certificate -Name "admin-client" -CN "admin-client" -Type "client"

# Step 4: Create certificate bundles
Write-Host "Creating certificate bundles..." -ForegroundColor Yellow
Copy-Item "$CertDir\ca.crt" "$CertDir\ca-bundle.crt"
Copy-Item "$CertDir\ca.crt" "$CertDir\client-ca-bundle.crt"

# Step 5: Generate certificate info file
$certInfo = @{
    generated_at = (Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ")
    valid_days = $DaysValid
    ca = @{
        cn = "Zuno Marketplace CA"
        cert = "ca.crt"
        key = "ca.key"
    }
    servers = @()
    clients = @(
        @{
            name = "graphql-gateway-client"
            cert = "graphql-gateway-client.crt"
            key = "graphql-gateway-client.key"
        },
        @{
            name = "admin-client"
            cert = "admin-client.crt"
            key = "admin-client.key"
        }
    )
}

foreach ($service in $services) {
    $certInfo.servers += @{
        name = $service.Name
        port = $service.Port
        cert = "$($service.Name).crt"
        key = "$($service.Name).key"
        cn = "$($service.Name).zuno-marketplace.local"
    }
}

$certInfo | ConvertTo-Json -Depth 3 | Set-Content -Path "$CertDir\cert-info.json"

# Step 6: Verify certificates
Write-Host "Verifying certificates..." -ForegroundColor Yellow

$allValid = $true
foreach ($service in $services) {
    try {
        & $OpenSSLPath verify -CAfile "$CertDir\ca.crt" "$CertDir\$($service.Name).crt" 2>&1 | Out-Null
        Write-Host "✓ $($service.Name) certificate is valid" -ForegroundColor Green
    }
    catch {
        Write-Host "✗ $($service.Name) certificate verification failed" -ForegroundColor Red
        $allValid = $false
    }
}

# Verify client certificate
try {
    & $OpenSSLPath verify -CAfile "$CertDir\ca.crt" "$CertDir\graphql-gateway-client.crt" 2>&1 | Out-Null
    Write-Host "✓ graphql-gateway-client certificate is valid" -ForegroundColor Green
}
catch {
    Write-Host "✗ graphql-gateway-client certificate verification failed" -ForegroundColor Red
    $allValid = $false
}

if (-not $allValid) {
    Write-Host "Certificate verification failed!" -ForegroundColor Red
    exit 1
}

Write-Host "=====================================" -ForegroundColor Green
Write-Host "Certificate generation completed successfully!" -ForegroundColor Green
Write-Host "=====================================" -ForegroundColor Green
Write-Host ""
Write-Host "Generated files in $CertDir`:"
Get-ChildItem -Path $CertDir -Filter "*.crt" | ForEach-Object { Write-Host "  $($_.Name)" }
Get-ChildItem -Path $CertDir -Filter "*.key" | ForEach-Object { Write-Host "  $($_.Name)" }
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "1. Copy certificates to appropriate service directories"
Write-Host "2. Update service configurations to use TLS"
Write-Host "3. Set proper file permissions in production"
$expiryDate = (Get-Date).AddDays($DaysValid).ToString("yyyy-MM-dd")
Write-Host "4. Implement certificate rotation before expiry ($expiryDate)"
