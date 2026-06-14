# Time-based routing declared in Terraform: callers to 6000 reach the support
# queue during business hours, and an "office is closed" message otherwise.
#
# How it works (pure dialplan, no runtime moving parts): two extensions match
# the same number, ordered by priority. The first carries a FreeSWITCH time
# window (wday + hour); when the current time is outside it the condition fails,
# the extension is skipped, and the call falls through to the lower-priority
# "closed" extension.
#
#   tofu -chdir=examples/time-routing apply

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

locals {
  domain = "192.168.48.143"
  ctx    = "company"
}

# Business hours: Mon–Fri (wday 2–6), 09:00–17:59 -> into the support queue.
resource "freeswitch_dialplan_extension" "support_hours" {
  name     = "tr-support-hours"
  domain   = local.domain
  context  = local.ctx
  priority = 20

  condition {
    field      = "destination_number"
    expression = "^(6000)$"
    time       = { wday = "2-6", hour = "9-17" }
    action {
      application = "answer"
    }
    action {
      application = "callcenter"
      data        = "support@${local.domain}"
    }
  }
}

# Off-hours fallback (reached only when the time window above did not match).
resource "freeswitch_dialplan_extension" "support_closed" {
  name     = "tr-support-closed"
  domain   = local.domain
  context  = local.ctx
  priority = 21

  condition {
    field      = "destination_number"
    expression = "^(6000)$"
    action {
      application = "answer"
    }
    action {
      application = "speak"
      data        = "tts_commandline|en_US-ryan-medium|Our office is closed. Please call back during business hours, Monday to Friday, nine to six."
    }
    action {
      application = "hangup"
    }
  }
}

resource "freeswitch_reloadxml" "apply" {
  triggers = {
    hours  = freeswitch_dialplan_extension.support_hours.id
    closed = freeswitch_dialplan_extension.support_closed.id
  }
}
