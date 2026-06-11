# Text-to-speech IVR fully in Terraform: you give the TEXT, FreeSWITCH speaks it
# at runtime via mod_flite (no pre-recorded files). Voice "slt" is female;
# "rms"/"awb"/"kal" are male.
#
#   tofu -chdir=examples/ivr-tts apply
#   (dial 9100 -> the prompt below is spoken)

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

variable "ivr_text" {
  type    = string
  default = "Welcome to the terraform managed I V R. Please press one for sales, or two for support."
}

variable "voice" {
  type    = string
  default = "slt" # female; use rms / awb / kal for male
}

data "freeswitch_domain" "main" {
  name = "192.168.48.143"
}

resource "freeswitch_dialplan_extension" "tts_menu" {
  name     = "tts-menu"
  domain   = data.freeswitch_domain.main.name
  context  = "company"
  priority = 90

  condition {
    field      = "destination_number"
    expression = "^(9100)$"

    action { application = "answer" }
    action {
      application = "speak"
      data        = "flite|${var.voice}|${var.ivr_text}"
    }
  }
}

resource "freeswitch_reloadxml" "apply" {
  triggers = {
    ext = freeswitch_dialplan_extension.tts_menu.id
  }
}
