variable "keycloak_url" {
  type        = string
  default     = "http://localhost:8081"
  description = "Base URL of the Keycloak server (the docker-compose service)."
}

variable "keycloak_admin" {
  type        = string
  default     = "admin"
  description = "Keycloak bootstrap admin username."
}

variable "keycloak_admin_password" {
  type        = string
  sensitive   = true
  description = "Keycloak bootstrap admin password (set via TF_VAR_keycloak_admin_password)."
}

variable "webphone_redirect_uris" {
  type        = list(string)
  default     = ["http://localhost:5173/*", "http://localhost:5174/*"]
  description = "Allowed OIDC redirect URIs for the webphone SPA."
}

variable "webphone_web_origins" {
  type        = list(string)
  default     = ["http://localhost:5173", "http://localhost:5174"]
  description = "Allowed CORS web origins for the webphone SPA."
}
