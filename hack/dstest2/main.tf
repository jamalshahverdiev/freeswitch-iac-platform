terraform {
  required_providers { freeswitch = { source = "local/freeswitch" } }
}
provider "freeswitch" {
  endpoint     = "https://localhost:8080"
  token        = "dev-token"
  ca_cert_file = "../../deploy/tls/ca.crt"
}
data "freeswitch_callcenter_queue"  "q" { name = "support@192.168.48.143" }
data "freeswitch_callcenter_agent"  "a" { name = "4201@192.168.48.143" }
data "freeswitch_callcenter_tier" "t" {
  queue = data.freeswitch_callcenter_queue.q.name
  agent = data.freeswitch_callcenter_agent.a.name
}
data "freeswitch_conference_profile" "p" { name = "video-grid" }
data "freeswitch_conference_room"    "r" { name = "standup" }
data "freeswitch_conference_status"  "s" { name = "standup" }
output "queue_strategy"  { value = data.freeswitch_callcenter_queue.q.strategy }
output "agent_contact"   { value = data.freeswitch_callcenter_agent.a.contact }
output "tier_level"      { value = data.freeswitch_callcenter_tier.t.level }
output "profile_video"   { value = "${data.freeswitch_conference_profile.p.video_mode}/${data.freeswitch_conference_profile.p.video_layout}" }
output "room_number"     { value = data.freeswitch_conference_room.r.number }
output "conf_running"    { value = data.freeswitch_conference_status.s.running }
