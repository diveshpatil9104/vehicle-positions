#!/usr/bin/env bash
# Seeds a fresh benchmark server with the driver the benchmarks sign in as and the six buses they pick from.
# Run it once, after the server has started with ADMIN_BOOTSTRAP_EMAIL and ADMIN_BOOTSTRAP_PASSWORD set.
set -euo pipefail

SERVER=${SERVER:-http://127.0.0.1:8080}
: "${ADMIN_EMAIL:?set ADMIN_EMAIL to the server's ADMIN_BOOTSTRAP_EMAIL}"
: "${ADMIN_PASSWORD:?set ADMIN_PASSWORD to the server's ADMIN_BOOTSTRAP_PASSWORD}"
DRIVER_EMAIL=${DRIVER_EMAIL:-driver@bench.test}
DRIVER_PASSWORD=${DRIVER_PASSWORD:-bench-driver-1}

# The value of a top-level JSON field, string or number, from a one-line response.
field() { sed -n "s/.*\"$1\":\"\{0,1\}\([^\",}]*\).*/\1/p"; }

post() {
    curl -sSf -X POST "$SERVER$1" -H 'Content-Type: application/json' ${token:+-H "Authorization: Bearer $token"} -d "$2"
}

token=$(post /api/v1/auth/login "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}" | field token)
driver=$(post /api/v1/admin/users \
    "{\"name\":\"Bench Driver\",\"email\":\"$DRIVER_EMAIL\",\"password\":\"$DRIVER_PASSWORD\",\"role\":\"driver\"}" | field id)
for bus in 4317 4318 4322 4401 4523 4567; do
    post /api/v1/admin/vehicles "{\"id\":\"$bus\",\"label\":\"Bus $bus\"}" >/dev/null
    post /api/v1/admin/assignments "{\"user_id\":$driver,\"vehicle_id\":\"$bus\"}" >/dev/null
done
echo "Seeded $DRIVER_EMAIL (user $driver) with buses 4317 4318 4322 4401 4523 4567"
