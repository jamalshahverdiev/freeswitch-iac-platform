output "realm" {
  value = keycloak_realm.fs.realm
}

output "issuer" {
  value       = "${var.keycloak_url}/realms/${keycloak_realm.fs.realm}"
  description = "OIDC issuer — the BFF validates JWTs against <issuer>/protocol/openid-connect/certs."
}

output "webphone_client_id" {
  value = keycloak_openid_client.webphone.client_id
}
