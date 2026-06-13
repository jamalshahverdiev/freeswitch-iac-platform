# Video echo test extension: dial 9196 and FreeSWITCH mirrors your audio AND
# video back to you (the `echo` application echoes all media). The standard
# single-client WebRTC video test — pair with the self-hosted webphone
# (https://172.31.30.216:8443).
#
#   tofu -chdir=examples/video-echo apply

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

resource "freeswitch_dialplan_extension" "video_echo" {
  name     = "video-echo"
  domain   = data.freeswitch_domain.main.name
  context  = "company"
  priority = 95

  condition {
    field      = "destination_number"
    expression = "^(9196)$"

    action { application = "answer" }
    action { application = "echo" }
  }
}

resource "freeswitch_reloadxml" "apply" {
  triggers = {
    ext = freeswitch_dialplan_extension.video_echo.id
  }
}
