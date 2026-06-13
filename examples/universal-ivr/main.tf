# Universal Terraform IVR — fully TTS (female voice slt), nested submenus, plus a
# SIP user to test it from a softphone. Everything declared in Terraform.
#
#   user 4100 (password: TF_VAR_sip_password; register from Windows softphone, domain 192.168.48.143)
#
#   3333 main: "Welcome to Universal Terraform IVR.
#               To ask question SRE press 1. To ask Developers press 2."
#     1 -> SRE submenu : Platform(1) Cloudservices(2) DevSupport(3) DevPortal(4)
#     2 -> Dev submenu : Golang(1) Kotlin(2)
#
# Menu pattern: speak (TTS prompt) + play_and_get_digits (silence prompt collects
# the digit) + transfer to a synthetic number routed by the next extension.
#
#   tofu -chdir=examples/universal-ivr apply

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
  engine = "tts_commandline"     # piper via mod_tts_commandline ("flite" = fallback)
  voice  = "en_US-ryan-medium"   # piper male voice; female: en_US-amy-medium (flite: slt/rms/awb/kal)
  # play_and_get_digits args prefix: min max tries timeout terminators prompt invalid
  getdig = "1 1 3 7000 # silence_stream://500 silence_stream://500"
}

# ---- SIP user to test the IVR from a softphone ----
variable "sip_password" {
  type      = string
  sensitive = true
  # set via TF_VAR_sip_password (see deploy/SECRETS.md)
}

resource "freeswitch_user" "u4100" {
  domain = local.domain
  number = "4100"
  params = {
    password    = var.sip_password
    vm-password = "4100"
  }
  variables = {
    effective_caller_id_name   = "Test 4100"
    effective_caller_id_number = "4100"
    user_context               = local.ctx
  }
}

# ---- Main menu: 3333 ----
resource "freeswitch_dialplan_extension" "main" {
  name     = "uivr-main"
  domain   = local.domain
  context  = local.ctx
  priority = 300
  condition {
    field      = "destination_number"
    expression = "^(3333)$"
    action {
      application = "answer"
    }
    action {
      application = "sleep"
      data        = "500"
    }
    action {
      application = "speak"
      data        = "${local.engine}|${local.voice}|Welcome to Universal Terraform I V R. To ask question S R E press 1. To ask Developers press 2."
    }
    action {
      application = "play_and_get_digits"
      data        = "${local.getdig} umain \\d 3000"
    }
    action {
      application = "transfer"
      data        = "u_main_$${umain} XML ${local.ctx}"
    }
  }
}

resource "freeswitch_dialplan_extension" "main_1" {
  name     = "uivr-main-1"
  domain   = local.domain
  context  = local.ctx
  priority = 301
  condition {
    field      = "destination_number"
    expression = "^u_main_1$"
    action {
      application = "transfer"
      data        = "u_sre XML ${local.ctx}"
    }
  }
}

resource "freeswitch_dialplan_extension" "main_2" {
  name     = "uivr-main-2"
  domain   = local.domain
  context  = local.ctx
  priority = 302
  condition {
    field      = "destination_number"
    expression = "^u_main_2$"
    action {
      application = "transfer"
      data        = "u_dev XML ${local.ctx}"
    }
  }
}

# ---- SRE submenu ----
resource "freeswitch_dialplan_extension" "sre" {
  name     = "uivr-sre"
  domain   = local.domain
  context  = local.ctx
  priority = 310
  condition {
    field      = "destination_number"
    expression = "^u_sre$"
    action {
      application = "speak"
      data        = "${local.engine}|${local.voice}|To ask Platform press 1. To ask Cloudservices press 2. To ask Dev Support press 3. To ask Dev Portal press 4."
    }
    action {
      application = "play_and_get_digits"
      data        = "${local.getdig} usre \\d 3000"
    }
    action {
      application = "transfer"
      data        = "u_sre_$${usre} XML ${local.ctx}"
    }
  }
}

resource "freeswitch_dialplan_extension" "sre_1" {
  name     = "uivr-sre-1"
  domain   = local.domain
  context  = local.ctx
  priority = 311
  condition {
    field      = "destination_number"
    expression = "^u_sre_1$"
    action {
      application = "speak"
      data        = "${local.engine}|${local.voice}|You selected Platform team. Goodbye."
    }
    action {
      application = "hangup"
    }
  }
}

resource "freeswitch_dialplan_extension" "sre_2" {
  name     = "uivr-sre-2"
  domain   = local.domain
  context  = local.ctx
  priority = 312
  condition {
    field      = "destination_number"
    expression = "^u_sre_2$"
    action {
      application = "speak"
      data        = "${local.engine}|${local.voice}|You selected Cloud Services team. Goodbye."
    }
    action {
      application = "hangup"
    }
  }
}

resource "freeswitch_dialplan_extension" "sre_3" {
  name     = "uivr-sre-3"
  domain   = local.domain
  context  = local.ctx
  priority = 313
  condition {
    field      = "destination_number"
    expression = "^u_sre_3$"
    action {
      application = "speak"
      data        = "${local.engine}|${local.voice}|You selected Dev Support team. Goodbye."
    }
    action {
      application = "hangup"
    }
  }
}

resource "freeswitch_dialplan_extension" "sre_4" {
  name     = "uivr-sre-4"
  domain   = local.domain
  context  = local.ctx
  priority = 314
  condition {
    field      = "destination_number"
    expression = "^u_sre_4$"
    action {
      application = "speak"
      data        = "${local.engine}|${local.voice}|You selected Dev Portal team. Goodbye."
    }
    action {
      application = "hangup"
    }
  }
}

# ---- Developers submenu ----
resource "freeswitch_dialplan_extension" "dev" {
  name     = "uivr-dev"
  domain   = local.domain
  context  = local.ctx
  priority = 320
  condition {
    field      = "destination_number"
    expression = "^u_dev$"
    action {
      application = "speak"
      data        = "${local.engine}|${local.voice}|To ask Golang developers press 1. To ask Kotlin developers press 2."
    }
    action {
      application = "play_and_get_digits"
      data        = "${local.getdig} udev \\d 3000"
    }
    action {
      application = "transfer"
      data        = "u_dev_$${udev} XML ${local.ctx}"
    }
  }
}

resource "freeswitch_dialplan_extension" "dev_1" {
  name     = "uivr-dev-1"
  domain   = local.domain
  context  = local.ctx
  priority = 321
  condition {
    field      = "destination_number"
    expression = "^u_dev_1$"
    action {
      application = "speak"
      data        = "${local.engine}|${local.voice}|You selected Golang developers. Goodbye."
    }
    action {
      application = "hangup"
    }
  }
}

resource "freeswitch_dialplan_extension" "dev_2" {
  name     = "uivr-dev-2"
  domain   = local.domain
  context  = local.ctx
  priority = 322
  condition {
    field      = "destination_number"
    expression = "^u_dev_2$"
    action {
      application = "speak"
      data        = "${local.engine}|${local.voice}|You selected Kotlin developers. Goodbye."
    }
    action {
      application = "hangup"
    }
  }
}

resource "freeswitch_reloadxml" "apply" {
  triggers = {
    main = freeswitch_dialplan_extension.main.id
    sre  = freeswitch_dialplan_extension.sre.id
    dev  = freeswitch_dialplan_extension.dev.id
    user = freeswitch_user.u4100.id
    leaves = join(",", [
      freeswitch_dialplan_extension.main_1.id, freeswitch_dialplan_extension.main_2.id,
      freeswitch_dialplan_extension.sre_1.id, freeswitch_dialplan_extension.sre_2.id,
      freeswitch_dialplan_extension.sre_3.id, freeswitch_dialplan_extension.sre_4.id,
      freeswitch_dialplan_extension.dev_1.id, freeswitch_dialplan_extension.dev_2.id,
    ])
  }
}
