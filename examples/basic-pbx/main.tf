terraform {
  required_providers {
    freeswitch = {
      source  = "local/freeswitch"
      version = "0.1.0"
    }
  }
}

provider "freeswitch" {
  endpoint = var.freeswitch_endpoint
  token    = var.freeswitch_token
}

resource "freeswitch_domain" "main" {
  name        = "company.local"
  description = "MVP test domain"

  variables = {
    timezone         = "Europe/Minsk"
    default_language = "ru"
  }
}

resource "freeswitch_user" "user_1001" {
  domain = freeswitch_domain.main.name
  number = "1001"

  params = {
    password    = var.user_1001_password
    vm-password = "1001"
  }

  variables = {
    effective_caller_id_name   = "User 1001"
    effective_caller_id_number = "1001"
    user_context               = "default"
  }
}

resource "freeswitch_user" "user_1002" {
  domain = freeswitch_domain.main.name
  number = "1002"

  params = {
    password    = var.user_1002_password
    vm-password = "1002"
  }

  variables = {
    effective_caller_id_name   = "User 1002"
    effective_caller_id_number = "1002"
    user_context               = "default"
  }
}

resource "freeswitch_dialplan_extension" "internal_users" {
  name     = "internal-users"
  domain   = freeswitch_domain.main.name
  context  = "default"
  priority = 10

  condition {
    field      = "destination_number"
    expression = "^(10[0-9][0-9])$"

    action {
      application = "bridge"
      data        = "user/$1@${freeswitch_domain.main.name}"
    }
  }
}
