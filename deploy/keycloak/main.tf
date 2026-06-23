# Declarative Keycloak realm for the FreeSWITCH webphone: authentication + RBAC.
# Keeps identities/roles "as code" alongside the rest of the platform.
#
# Apply (with the keycloak service from docker-compose running):
#   export KEYCLOAK_URL=http://localhost:8081
#   export TF_VAR_keycloak_admin_password=<admin pass>
#   tofu init && tofu apply

terraform {
  required_providers {
    keycloak = {
      source  = "keycloak/keycloak"
      version = "~> 5.0"
    }
  }
}

provider "keycloak" {
  client_id = "admin-cli"
  username  = var.keycloak_admin
  password  = var.keycloak_admin_password
  url       = var.keycloak_url
}

resource "keycloak_realm" "fs" {
  realm        = "freeswitch"
  enabled      = true
  display_name = "FreeSWITCH IaC Platform"
}

# Public SPA client — Authorization Code + PKCE (no client secret in the browser).
resource "keycloak_openid_client" "webphone" {
  realm_id  = keycloak_realm.fs.id
  client_id = "webphone"
  name      = "FreeSWITCH Webphone"
  enabled   = true

  access_type                  = "PUBLIC"
  standard_flow_enabled        = true
  direct_access_grants_enabled = false
  pkce_code_challenge_method   = "S256"

  valid_redirect_uris = var.webphone_redirect_uris
  web_origins         = var.webphone_web_origins
}

# RBAC realm roles. Keycloak puts these in the JWT under realm_access.roles,
# which the BFF reads to authorize requests.
resource "keycloak_role" "agent" {
  realm_id    = keycloak_realm.fs.id
  name        = "agent"
  description = "Operator: own softphone, own history/voicemail"
}

resource "keycloak_role" "supervisor" {
  realm_id    = keycloak_realm.fs.id
  name        = "supervisor"
  description = "Wallboard, agent/queue control, listen/whisper/barge, recordings"
}

resource "keycloak_role" "admin" {
  realm_id    = keycloak_realm.fs.id
  name        = "admin"
  description = "Full runtime admin"
}
