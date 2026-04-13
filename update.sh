#!/bin/bash
# =============================================================
# TTNFlow — Update all services
# Run on the server: bash /opt/ttnflow/backend/update.sh
# Optional flags:
#   --api       update API only
#   --web       update Vue only
#   --landing   update Astro landing only
# No flags = update everything
# =============================================================
set -e

APP_DIR="/opt/ttnflow"
COMPOSE="docker compose -f $APP_DIR/backend/docker-compose.prod.yml"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()  { echo -e "${GREEN}[✓]${NC} $1"; }
warn()  { echo -e "${YELLOW}[!]${NC} $1"; }
error() { echo -e "${RED}[✗]${NC} $1"; exit 1; }

[ "$EUID" -ne 0 ] && error "Run as root"

# Parse flags
DO_API=false; DO_WEB=false; DO_LANDING=false
if [ $# -eq 0 ]; then
    DO_API=true; DO_WEB=true; DO_LANDING=true
else
    for arg in "$@"; do
        case $arg in
            --api)     DO_API=true ;;
            --web)     DO_WEB=true ;;
            --landing) DO_LANDING=true ;;
            *) error "Unknown flag: $arg. Use --api, --web, --landing" ;;
        esac
    done
fi

# ── API ──────────────────────────────────────────────────────
if $DO_API; then
    info "Updating API..."
    git -C "$APP_DIR/backend" pull
    $COMPOSE up --build -d api
    info "API updated and restarted"
fi

# ── Vue SPA ──────────────────────────────────────────────────
if $DO_WEB; then
    info "Updating Vue web app..."
    git -C "$APP_DIR/web" pull
    cd "$APP_DIR/web"
    npm ci --silent
    npm run build
    rm -rf /var/www/ttnflow/app/*
    cp -r dist/* /var/www/ttnflow/app/
    info "Vue app updated"
fi

# ── Astro landing ─────────────────────────────────────────────
if $DO_LANDING; then
    info "Updating Astro landing..."
    git -C "$APP_DIR/landing" pull
    cd "$APP_DIR/landing"
    npm ci --silent
    npm run build
    rm -rf /var/www/ttnflow/landing/*
    cp -r dist/* /var/www/ttnflow/landing/
    info "Landing updated"
fi

# ── Nginx reload ─────────────────────────────────────────────
if $DO_WEB || $DO_LANDING; then
    nginx -t && systemctl reload nginx
    info "Nginx reloaded"
fi

echo ""
info "Done! https://ttnflow.com"
