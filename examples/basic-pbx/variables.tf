variable "freeswitch_endpoint" {
  type    = string
  default = "http://localhost:8080"
}

variable "freeswitch_token" {
  type      = string
  sensitive = true
  default   = "dev-token"
}

variable "user_1001_password" {
  type      = string
  sensitive = true
  default   = "1234"
}

variable "user_1002_password" {
  type      = string
  sensitive = true
  default   = "1234"
}
