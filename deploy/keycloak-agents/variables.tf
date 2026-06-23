variable "keycloak_url" {
  type    = string
  default = "http://localhost:8081"
}
variable "keycloak_admin" {
  type    = string
  default = "admin"
}
variable "keycloak_admin_password" {
  type      = string
  sensitive = true
}
variable "control_plane_url" {
  type    = string
  default = "https://localhost:8080"
}
variable "control_plane_token" {
  type      = string
  default   = "dev-token"
  sensitive = true
}
variable "sip_domain" {
  type    = string
  default = "192.168.48.143"
}
variable "agent2_password" {
  type      = string
  default   = "agent2"
  sensitive = true
}
variable "super1_password" {
  type      = string
  default   = "super1"
  sensitive = true
}
