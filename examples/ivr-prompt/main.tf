# Goal A: a PRE-RECORDED prompt played by an IVR, managed via Terraform.
# The .wav was generated with flite and shipped to the FreeSWITCH server at
#   /usr/share/freeswitch/sounds/en/us/callie/ivr/8000/tf-custom-prompt.wav
# (source kept in deploy/freeswitch/sounds/). Terraform just references it.
#
#   tofu -chdir=examples/ivr-prompt apply   # dial 9200 -> the file is played

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

data "freeswitch_domain" "main" {
  name = "192.168.48.143"
}

resource "freeswitch_dialplan_extension" "prompt" {
  name     = "custom-prompt"
  domain   = data.freeswitch_domain.main.name
  context  = "company"
  priority = 92

  condition {
    field      = "destination_number"
    expression = "^(9200)$"

    action { application = "answer" }
    action {
      application = "playback"
      data        = "ivr/tf-custom-prompt.wav"
    }
  }
}

resource "freeswitch_reloadxml" "apply" {
  triggers = {
    ext = freeswitch_dialplan_extension.prompt.id
  }
}
