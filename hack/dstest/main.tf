terraform {
  required_providers {
    freeswitch = { source = "local/freeswitch" }
  }
}
provider "freeswitch" {
  endpoint     = "https://localhost:8080"
  token        = "dev-token"
  ca_cert_file = "../../deploy/tls/ca.crt"
}

data "freeswitch_domain" "main" { name = "192.168.48.143" }

data "freeswitch_user" "u2001" {
  domain = "192.168.48.143"
  number = "2001"
}

data "freeswitch_user_registration" "u2001" {
  user   = "2001"
  domain = "192.168.48.143"
}

data "freeswitch_gateway" "ds" {
  profile = "external"
  name    = "ds-test"
}

data "freeswitch_dialplan_extension" "ivr" {
  id = "69d630a3-40ff-4e52-9bda-46b6f025e27f"
}

data "freeswitch_gateway_status" "ex" {
  profile = "external"
  name    = "example.com"
}

output "domain_enabled" { value = data.freeswitch_domain.main.enabled }
output "user_caller_id" { value = data.freeswitch_user.u2001.variables["effective_caller_id_name"] }
output "user_registered" { value = data.freeswitch_user_registration.u2001.registered }
output "gateway_proxy" { value = data.freeswitch_gateway.ds.proxy }
output "dialplan_name" { value = data.freeswitch_dialplan_extension.ivr.name }
output "dialplan_cond0_expr" { value = data.freeswitch_dialplan_extension.ivr.condition[0].expression }
output "gateway_status_state" { value = data.freeswitch_gateway_status.ex.state }
