# Installation

- Download the binary for your OS.
- Place it in your Terraform plugin directory:
    - Linux/macOS: ~/.terraform.d/plugins/local/faxter/faxter/0.1/PLATFORM/terraform-provider-faxter
    - Windows: %APPDATA%\terraform.d\plugins\local\faxter\faxter\0.1\PLATFORM\terraform-provider-faxter.exe

(PLATFORM is usually something like darwin_amd64 or linux_amd64.)
  

You can then reference it in your Terraform configuration:


```h
terraform {
  required_providers {
    faxter = {
      source = "local/faxter/faxter"
      version = "0.1"
    }
  }
}
```