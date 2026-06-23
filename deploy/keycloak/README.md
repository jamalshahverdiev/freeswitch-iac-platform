# Keycloak realm (webphone auth + RBAC)

Declarative Keycloak config for the webphone, managed with
[`terraform-provider-keycloak`](https://registry.terraform.io/providers/keycloak/keycloak/latest).
Defines the `freeswitch` realm, the `webphone` public SPA client (Auth Code +
PKCE), and the RBAC realm roles `agent` / `supervisor` / `admin`.

Keycloak itself runs as the `keycloak` service in the root `docker-compose.yml`
(persistent store in the `keycloak` Postgres DB; admin console on
<http://localhost:8081>).

## Apply

```bash
docker compose up -d keycloak          # from the repo root
export KEYCLOAK_URL=http://localhost:8081
export TF_VAR_keycloak_admin_password="$KEYCLOAK_ADMIN_PASSWORD"
tofu init
tofu apply
```

Outputs the OIDC **issuer** (`$KEYCLOAK_URL/realms/freeswitch`); the BFF validates
JWTs against `<issuer>/protocol/openid-connect/certs`, and the SPA logs in against
this realm/client.

## How it fits

- **Authentication + roles** live here (Keycloak). Realm roles ride in the JWT
  under `realm_access.roles`.
- **identity → SIP extension** binding lives in the control-plane `operators`
  table (Terraform resource `freeswitch_operator`), keyed by the Keycloak user's
  `sub`. End to end you can wire `freeswitch_operator.subject =
  keycloak_user.<u>.id` so both stay as code.
- Add users either in the Keycloak admin console or as `keycloak_user` resources;
  assign them the `agent` / `supervisor` / `admin` role.
