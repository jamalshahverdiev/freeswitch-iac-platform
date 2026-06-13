# A complete, nested IVR defined entirely in Terraform and served to FreeSWITCH
# via mod_xml_curl. Proves IVR is just data: freeswitch_dialplan_extension rows.
#
#   8000 main : 1 -> submenu(8100) | 2 -> call 2001 | 3 -> echo
#   8100 sub  : 1 -> call 2002      | 0 -> back to 8000
#
# Run with OpenTofu + ~/.terraformrc dev_overrides (no init):
#   export TF_CLI_CONFIG_FILE=$HOME/.terraformrc
#   tofu -chdir=examples/ivr apply

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

# Reference the already-existing domain (created out of band) via a data source.
data "freeswitch_domain" "main" {
  name = "192.168.48.143"
}

locals {
  ctx     = "company"
  welcome = "ivr/ivr-welcome_to_freeswitch.wav"
  submenu = "ivr/ivr-please_enter_the_extension.wav"
  invalid = "ivr/ivr-that_was_an_invalid_entry.wav"
}

resource "freeswitch_dialplan_extension" "ivr_main" {
  name     = "tf-ivr-main"
  domain   = data.freeswitch_domain.main.name
  context  = local.ctx
  priority = 200

  condition {
    field      = "destination_number"
    expression = "^(8000)$"
    action { application = "answer" }
    action {
      application = "sleep"
      data        = "500"
    }
    action {
      application = "play_and_get_digits"
      data        = "1 1 3 5000 # ${local.welcome} ${local.invalid} ivr_choice \\d 3000"
    }
    action {
      application = "transfer"
      data        = "tfmain_$${ivr_choice} XML ${local.ctx}"
    }
  }
}

resource "freeswitch_dialplan_extension" "main_1" {
  name     = "tf-main-1"
  domain   = data.freeswitch_domain.main.name
  context  = local.ctx
  priority = 201
  condition {
    field      = "destination_number"
    expression = "^tfmain_1$"
    action {
      application = "transfer"
      data        = "8100 XML ${local.ctx}"
    }
  }
}

resource "freeswitch_dialplan_extension" "main_2" {
  name     = "tf-main-2"
  domain   = data.freeswitch_domain.main.name
  context  = local.ctx
  priority = 202
  condition {
    field      = "destination_number"
    expression = "^tfmain_2$"
    action {
      application = "transfer"
      data        = "2001 XML ${local.ctx}"
    }
  }
}

resource "freeswitch_dialplan_extension" "main_3" {
  name     = "tf-main-3"
  domain   = data.freeswitch_domain.main.name
  context  = local.ctx
  priority = 203
  condition {
    field      = "destination_number"
    expression = "^tfmain_3$"
    action { application = "answer" }
    action { application = "echo" }
  }
}

resource "freeswitch_dialplan_extension" "ivr_sub" {
  name     = "tf-ivr-sub"
  domain   = data.freeswitch_domain.main.name
  context  = local.ctx
  priority = 210

  condition {
    field      = "destination_number"
    expression = "^(8100)$"
    action { application = "answer" }
    action {
      application = "play_and_get_digits"
      data        = "1 1 3 5000 # ${local.submenu} ${local.invalid} sub_choice \\d 3000"
    }
    action {
      application = "transfer"
      data        = "tfsub_$${sub_choice} XML ${local.ctx}"
    }
  }
}

resource "freeswitch_dialplan_extension" "sub_1" {
  name     = "tf-sub-1"
  domain   = data.freeswitch_domain.main.name
  context  = local.ctx
  priority = 211
  condition {
    field      = "destination_number"
    expression = "^tfsub_1$"
    action {
      application = "transfer"
      data        = "2002 XML ${local.ctx}"
    }
  }
}

resource "freeswitch_dialplan_extension" "sub_0" {
  name     = "tf-sub-0"
  domain   = data.freeswitch_domain.main.name
  context  = local.ctx
  priority = 212
  condition {
    field      = "destination_number"
    expression = "^tfsub_0$"
    action {
      application = "transfer"
      data        = "8000 XML ${local.ctx}"
    }
  }
}

# Apply the config in FreeSWITCH whenever any extension id changes.
resource "freeswitch_reloadxml" "apply" {
  triggers = {
    main = freeswitch_dialplan_extension.ivr_main.id
    sub  = freeswitch_dialplan_extension.ivr_sub.id
    r    = join(",", [
      freeswitch_dialplan_extension.main_1.id,
      freeswitch_dialplan_extension.main_2.id,
      freeswitch_dialplan_extension.main_3.id,
      freeswitch_dialplan_extension.sub_1.id,
      freeswitch_dialplan_extension.sub_0.id,
    ])
  }
}
