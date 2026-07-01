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

    # Voicemail on no-answer: hang up after a completed call (no voicemail then),
    # but on a failed bridge (no answer / unavailable / busy within the timeout)
    # fall through to the voicemail deposit below.
    action {
      application = "set"
      data        = "hangup_after_bridge=true"
    }
    action {
      application = "set"
      data        = "continue_on_fail=true"
    }
    action {
      application = "set"
      data        = "call_timeout=20"
    }

    # Record the conversation. record_session runs on the B-leg (callee) right
    # before the bridge connects (bridge_pre_execute_bleg_*), so recording starts
    # at answer with no ringback and no race with the inbound A-leg's answer. The
    # file name encodes both parties + uuid so the control-plane can scope "my
    # recordings" per extension: <caller>_<dest>_<uuid>.wav under the per-day tree
    # in $${recordings_dir}. $$ escapes Terraform interpolation so the ${...}
    # reach FreeSWITCH verbatim.
    action {
      application = "set"
      data        = "RECORD_STEREO=true"
    }
    action {
      application = "set"
      data        = "recording_follow_transfer=true"
    }
    # Direct record_session on the A-leg before bridge: attaches a media bug now,
    # captures once media flows (post-bridge), and the file name resolves from the
    # A-leg's clean values (caller 4201, destination 4202) rather than the
    # originated B-leg's opaque contact token.
    action {
      application = "record_session"
      data        = "/var/lib/freeswitch/recordings/$${strftime(%Y/%m/%d)}/$${caller_id_number}_$${destination_number}_$${uuid}.wav"
    }

    action {
      application = "bridge"
      data        = "user/$1@${local.domain}"
    }

    # Reached only when the bridge failed (continue_on_fail) — leave a message.
    # Answer first (200 OK): the voicemail greeting would otherwise play as early
    # media (183) which WebRTC clients like our SIP.js phone don't render, so the
    # caller heard silence and hung up before the beep.
    action {
      application = "answer"
    }
    action {
      application = "voicemail"
      data        = "default ${local.domain} $1"
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
