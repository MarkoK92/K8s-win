# Generate a Self-Signed Certificate for 'ticker' hostname
# This allows HTTPS locally (browser will warn "Not Secure", which is expected)

# 1. Generate Private Key
openssl genrsa -out tls.key 2048

# 2. Generate Certificate Signing Request (CSR)
# CN=ticker is crucial (matches your hosts file)
openssl req -new -key tls.key -out tls.csr -subj "/CN=ticker"

# 3. Generate Self-Signed Certificate (valid for 365 days)
openssl x509 -req -days 365 -in tls.csr -signkey tls.key -out tls.crt

# 4. Create Kubernetes Secret
kubectl create secret tls ticker-tls-cert --cert=tls.crt --key=tls.key --dry-run=client -o yaml > charts/ingress/templates/tls-secret.yaml

# 5. Cleanup
rm tls.key tls.csr tls.crt
