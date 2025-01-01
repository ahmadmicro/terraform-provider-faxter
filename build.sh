GOOS=darwin GOARCH=arm64 go build -o bin/terraform-provider-faxter_v0.1.0_darwin_arm64
GOOS=darwin GOARCH=amd64 go build -o bin/terraform-provider-faxter_v0.1.0_darwin_amd64
GOOS=windows GOARCH=amd64 go build -o bin/terraform-provider-faxter_v0.1.0_windows_amd64.exe
GOOS=linux GOARCH=amd64 go build -o bin/terraform-provider-faxter_v0.1.0_linux_amd64 