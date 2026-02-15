# Generate Self-Signed Certificate for Localhost

$cert = New-SelfSignedCertificate -DnsName "ticker" -CertStoreLocation "cert:\CurrentUser\My"

# Export Certificate (Public Key)
$certParams = @{
  Cert = $cert
  Type = "CERT"
  FilePath = "tls.crt"
}
Export-Certificate @certParams

# Export Private Key (requires password, we use empty/dummy)
$pwd = ConvertTo-SecureString -String "password" -Force -AsPlainText
$keyParams = @{
  Cert = $cert
  FilePath = "tls.pfx"
  Password = $pwd
}
Export-PfxCertificate @keyParams

# Extract Key from PFX (tricky on Windows without OpenSSL, but K8s accepts PFX sometimes or we can use a workaround)
# Actually, for local dev, let's just use the PFX directly or rely on the certificate we just made. 
# Wait, Kubernetes needs key/cert pair in PEM format usually. 
# Since we don't have OpenSSL to convert PFX -> PEM, we might be stuck unless we use a specific tool.

# ALTERNATIVE: Use a simple C# snippet or just assume the user has Git Bash which HAS OpenSSL.
# Check if "git" is installed, usually comes with "openssl.exe" in its bin folder.
