# End-to-end "operator as code": create a Keycloak user AND bind its identity to
# a SIP extension in one apply, across two providers. This is the declarative
# version of what the BFF resolves at login (sub -> extension).
#
#   cd deploy/keycloak-agents
#   export KEYCLOAK_URL=http://localhost:8081
#   export TF_VAR_keycloak_admin_password=<admin pass>
#   tofu init && tofu apply

terraform {
  required_providers {
    keycloak = {
      source  = "keycloak/keycloak"
      version = "~> 5.0"
    }
    freeswitch = {
      source = "local/freeswitch" # dev_overrides → locally built provider
    }
  }
}

provider "keycloak" {
  client_id = "admin-cli"
  username  = var.keycloak_admin
  password  = var.keycloak_admin_password
  url       = var.keycloak_url
}

provider "freeswitch" {
  endpoint = var.control_plane_url
  token    = var.control_plane_token
  insecure = true # dev self-signed CA
}

# Reference the realm/role created by ../keycloak.
data "keycloak_realm" "fs" {
  realm = "freeswitch"
}
data "keycloak_role" "agent" {
  realm_id = data.keycloak_realm.fs.id
  name     = "agent"
}

# 1) The login identity in Keycloak.
resource "keycloak_user" "agent2" {
  realm_id       = data.keycloak_realm.fs.id
  username       = "agent2"
  enabled        = true
  email          = "agent2@example.com"
  email_verified = true
  first_name     = "Agent"
  last_name      = "Two"

  initial_password {
    value     = var.agent2_password
    temporary = false
  }
}

resource "keycloak_user_roles" "agent2" {
  realm_id = data.keycloak_realm.fs.id
  user_id  = keycloak_user.agent2.id
  role_ids = [data.keycloak_role.agent.id]
}

# 2) Bind that identity (its `sub` == keycloak_user.id) to extension 4202.
resource "freeswitch_operator" "agent2" {
  subject      = keycloak_user.agent2.id
  domain       = var.sip_domain
  number       = "4202"
  display_name = "Agent Two"
}

output "agent2_subject" {
  value = keycloak_user.agent2.id
}
output "agent2_extension" {
  value = freeswitch_operator.agent2.number
}
