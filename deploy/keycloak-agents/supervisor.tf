# A supervisor identity, as code: Keycloak user with the `supervisor` role, bound
# to extension 4100. The webphone shows the supervisor wallboard for this role.

data "keycloak_role" "supervisor" {
  realm_id = data.keycloak_realm.fs.id
  name     = "supervisor"
}

resource "keycloak_user" "super1" {
  realm_id       = data.keycloak_realm.fs.id
  username       = "super1"
  enabled        = true
  email          = "super1@example.com"
  email_verified = true
  first_name     = "Super"
  last_name      = "One"

  initial_password {
    value     = var.super1_password
    temporary = false
  }
}

resource "keycloak_user_roles" "super1" {
  realm_id = data.keycloak_realm.fs.id
  user_id  = keycloak_user.super1.id
  role_ids = [data.keycloak_role.supervisor.id]
}

resource "freeswitch_operator" "super1" {
  subject      = keycloak_user.super1.id
  domain       = var.sip_domain
  number       = "4100"
  display_name = "Super One"
}

output "super1_subject" {
  value = keycloak_user.super1.id
}
