# WebRTC-enabled user accounts.
#
# In FreeSWITCH a "WebRTC account" is a REGULAR directory user — WebRTC is just
# a different transport (SIP over WSS :7443 + DTLS-SRTP + Opus), already enabled
# on the server. So these are plain freeswitch_user resources; a browser client
# (SIP.js / JsSIP) registers with the same credentials over
#   wss://192.168.48.143:7443
# See docs/webrtc.md for the browser setup (incl. trusting our CA).
#
#   tofu -chdir=examples/webrtc-users apply

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

variable "webrtc_password" {
  type      = string
  sensitive = true
  # set via TF_VAR_webrtc_password (see deploy/SECRETS.md)
}

locals {
  domain = "192.168.48.143"
  users  = ["4201", "4202"]
}

resource "freeswitch_user" "webrtc" {
  for_each = toset(local.users)

  domain = local.domain
  number = each.value
  params = {
    password    = var.webrtc_password
    vm-password = each.value
  }
  variables = {
    effective_caller_id_name   = "WebRTC ${each.value}"
    effective_caller_id_number = each.value
    user_context               = "company"
  }
}

# Route 4xxx numbers to local users (covers 4100 and the WebRTC accounts).
resource "freeswitch_dialplan_extension" "internal_4xxx" {
  name     = "internal-4xxx"
  domain   = local.domain
  context  = "company"
  priority = 11

  condition {
    field      = "destination_number"
    expression = "^(4[0-9]{3})$"

    # Pin MOH on both bridge legs (export → also the originated B-leg). Without
    # this the global hold_music default isn't materialised on inbound WebRTC
    # channels, so a hold by the callee left the caller hearing silence.
    action {
      application = "export"
      data        = "hold_music=local_stream://moh"
    }

    action {
      application = "bridge"
      data        = "user/$1@${local.domain}"
    }
  }
}

resource "freeswitch_reloadxml" "apply" {
  triggers = {
    users = join(",", [for u in freeswitch_user.webrtc : u.id])
    route = freeswitch_dialplan_extension.internal_4xxx.id
  }
}

output "webrtc_connection" {
  value = {
    wss_url   = "wss://${local.domain}:7443"
    sip_uris  = [for u in local.users : "sip:${u}@${local.domain}"]
    note      = "trust deploy/tls/ca.crt in the browser OS first (or accept the cert at https://${local.domain}:7443)"
  }
}
