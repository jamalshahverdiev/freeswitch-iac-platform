# End-to-end test config for terraform-provider-freeswitch (run with OpenTofu
# via ~/.terraformrc dev_overrides — no `init` needed). Uses a throwaway domain
# so it never collides with live data; `tofu destroy` cleans up.
terraform {
  required_providers {
    freeswitch = {
      source = "local/freeswitch"
    }
  }
}

provider "freeswitch" {
  endpoint     = "https://localhost:8080"
  token        = "dev-token"
  ca_cert_file = "../../deploy/tls/ca.crt"
}

resource "freeswitch_domain" "e2e" {
  name        = "tf-e2e.local"
  description = "terraform e2e"
  variables   = { default_language = "en" }
}

resource "freeswitch_user" "u5551" {
  domain = freeswitch_domain.e2e.name
  number = "5551"
  params = {
    password    = "e2epass"
    vm-password = "5551"
  }
  variables = {
    user_context = "tfe2e"
  }
}

resource "freeswitch_dialplan_extension" "echo" {
  name     = "e2e-echo"
  domain   = freeswitch_domain.e2e.name
  context  = "tfe2e"
  priority = 10

  condition {
    field      = "destination_number"
    expression = "^(5551)$"

    action { application = "answer" }
    action { application = "echo" }
  }
}
