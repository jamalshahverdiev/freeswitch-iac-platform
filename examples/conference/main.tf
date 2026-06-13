# Video conferences declared in Terraform, served to FreeSWITCH from
# PostgreSQL (conference.conf via mod_xml_curl + room entry extensions in
# the dialplan):
#
#   profile video-grid   mux video, group:grid layout, 1280x720@15fps
#   room    standup      dial 3500 -> everyone sees the grid
#   room    private      dial 3501 -> PIN 2580
#
#   tofu -chdir=examples/conference apply
#
# Profiles are read when a NEW conference starts, so no module reload is
# needed — only reloadxml so the dialplan picks up the entry extensions.

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

  # Per-day folders: YYYY/MM/DD/conf_<room>_<time>.wav
  # $${...} is HCL-escaped -> mod_conference expands it when the room starts.
  recordings_dir = "/var/lib/freeswitch/recordings"
  auto_record    = "${local.recordings_dir}/$${strftime(%Y/%m/%d)}/conf_$${conference_name}_$${strftime(%H-%M-%S)}.wav"
}

resource "freeswitch_conference_profile" "video_grid" {
  name        = "video-grid"
  video_mode  = "mux"
  auto_record = local.auto_record
  # defaults: group:grid layout, 1280x720, 15 fps, 48 kHz, MOH while alone
}

resource "freeswitch_conference_room" "standup" {
  name    = "standup"
  number  = "3500"
  domain  = local.domain
  context = local.ctx
  profile = freeswitch_conference_profile.video_grid.name
}

resource "freeswitch_conference_room" "private" {
  name    = "private"
  number  = "3501"
  domain  = local.domain
  context = local.ctx
  profile = freeswitch_conference_profile.video_grid.name
  pin     = "2580"
}

# Room entry extensions live in the dialplan -> flush the XML cache.
resource "freeswitch_reloadxml" "rooms" {
  triggers = {
    rooms = join(",", [
      freeswitch_conference_room.standup.id,
      freeswitch_conference_room.private.id,
    ])
  }
}
