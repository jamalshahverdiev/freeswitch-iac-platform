# Full call-center scenario declared in Terraform, served to FreeSWITCH from
# PostgreSQL (config via mod_xml_curl, runtime state in Postgres via ODBC —
# no sqlite anywhere):
#
#   queue  support@192.168.48.143  (longest-idle-agent, music on hold)
#   agents 4201 / 4202             (the Browser-Phone WebRTC users)
#   dial   4444  ->  enter the queue (MOH) -> an Available agent rings
#
#   tofu -chdir=examples/callcenter apply

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
  agents = ["4201", "4202"]

  # Per-day folders (user requirement): YYYY/MM/DD/<file>.wav
  # $${...} is HCL-escaped -> FreeSWITCH expands ${strftime}/${uuid} at call time.
  recordings_dir = "/var/lib/freeswitch/recordings"
  record_path    = "${local.recordings_dir}/$${strftime(%Y/%m/%d)}/queue_$${strftime(%H-%M-%S)}_$${uuid}.wav"
}

resource "freeswitch_callcenter_queue" "support" {
  name = "support@${local.domain}"
  # defaults: longest-idle-agent, local_stream://moh, discard-abandoned 60s

  params = {
    # mod_callcenter's native recording: starts when the agent answers, so
    # the file contains the CONVERSATION (record_session on the caller leg
    # captured only MOH — the bridged Opus legs recorded as silence).
    "record-template" = local.record_path
  }
}

resource "freeswitch_callcenter_agent" "agent" {
  for_each = toset(local.agents)

  name    = "${each.value}@${local.domain}"
  contact = "user/${each.value}@${local.domain}"
  status  = "Available"
}

resource "freeswitch_callcenter_tier" "tier" {
  for_each = toset(local.agents)

  queue = freeswitch_callcenter_queue.support.name
  agent = freeswitch_callcenter_agent.agent[each.value].name
}

# Callers reach the queue by dialing 4444.
resource "freeswitch_dialplan_extension" "queue_entry" {
  name     = "cc-support-entry"
  domain   = local.domain
  context  = local.ctx
  # Must beat internal-4xxx (priority 11) which also matches 4444.
  priority = 10

  condition {
    field      = "destination_number"
    expression = "^(4444)$"

    action { application = "answer" }
    action {
      application = "callcenter"
      data        = freeswitch_callcenter_queue.support.name
    }
  }
}

# Apply config changes in FreeSWITCH.
resource "freeswitch_reloadxml" "dialplan" {
  triggers = {
    ext = freeswitch_dialplan_extension.queue_entry.id
  }
}

resource "freeswitch_callcenter_reload" "apply" {
  # Key on updated_at (not id): ids equal the names, so content-only changes
  # (e.g. a new record-template) would never fire the reload otherwise.
  triggers = {
    queue  = freeswitch_callcenter_queue.support.updated_at
    agents = join(",", [for a in freeswitch_callcenter_agent.agent : a.updated_at])
    tiers  = join(",", [for t in freeswitch_callcenter_tier.tier : t.updated_at])
  }
}
