#!/bin/bash
# =============================================================
# TTNFlow — Initial VPS Deploy
# Run once on a fresh Ubuntu 22.04/24.04 server as root
# =============================================================
set -e

DOMAIN="ttnflow.com"
EMAIL="pysarenkoigor@gmail.com"
BACKEND_REPO="https://github.com/NothingToSayLGFM/ttnbackend.git"
WEB_REPO="https://github.com/NothingToSayLGFM/ttnweb.git"
LANDING_REPO="https://github.com/NothingToSayLGFM/ttnlanding.git"
APP_DIR="/opt/ttnflow"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()  { echo -e "${GREEN}[✓]${NC} $1"; }
warn()  { echo -e "${YELLOW}[!]${NC} $1"; }
error() { echo -e "${RED}[✗]${NC} $1"; exit 1; }

[ "$EUID" -ne 0 ] && error "Run as root: sudo bash deploy.sh"

# ── System ───────────────────────────────────────────────────
info "Updating system..."
apt-get update -qq && apt-get upgrade -y -qq

info "Installing dependencies..."
apt-get install -y -qq git nginx certbot python3-certbot-nginx curl

info "Installing Node.js 22..."
curl -fsSL https://deb.nodesource.com/setup_22.x | bash -
apt-get install -y -qq nodejs

info "Installing Docker..."
if ! command -v docker &>/dev/null; then
    curl -fsSL https://get.docker.com | sh
    apt-get install -y -qq docker-compose-plugin
fi

# ── Clone repos ──────────────────────────────────────────────
info "Cloning repositories..."
mkdir -p "$APP_DIR"

clone_or_pull() {
    local repo=$1 dir=$2
    if [ -d "$dir/.git" ]; then
        git -C "$dir" pull
    else
        git clone "$repo" "$dir"
    fi
}

clone_or_pull "$BACKEND_REPO"  "$APP_DIR/backend"
clone_or_pull "$WEB_REPO"      "$APP_DIR/web"
clone_or_pull "$LANDING_REPO"  "$APP_DIR/landing"

# ── Environment ──────────────────────────────────────────────
if [ ! -f "$APP_DIR/backend/.env" ]; then
    cat > "$APP_DIR/backend/.env" <<'ENVEOF'
# Generate strong passwords with: openssl rand -hex 32
POSTGRES_PASSWORD=CHANGE_ME
JWT_SECRET=CHANGE_ME
TELEGRAM_URL=https://t.me/your_telegram
ENVEOF
    warn ".env created. Fill in real values:"
    echo ""
    echo "  nano $APP_DIR/backend/.env"
    echo ""
    warn "Then re-run: bash deploy.sh"
    exit 0
fi

if grep -q "CHANGE_ME" "$APP_DIR/backend/.env"; then
    error "Edit $APP_DIR/backend/.env — still has placeholder values!"
fi

# ── Docker Compose ───────────────────────────────────────────
info "Creating docker-compose.prod.yml..."
source "$APP_DIR/backend/.env"

cat > "$APP_DIR/backend/docker-compose.prod.yml" <<COMPOSEEOF
services:
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER: ttnflow
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: ttnflow
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./internal/db/migrations/001_init.up.sql:/docker-entrypoint-initdb.d/001_init.sql:ro
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ttnflow -d ttnflow"]
      interval: 5s
      timeout: 5s
      retries: 10

  api:
    build: .
    restart: unless-stopped
    environment:
      DATABASE_URL: postgres://ttnflow:${POSTGRES_PASSWORD}@postgres:5432/ttnflow?sslmode=disable
      JWT_SECRET: ${JWT_SECRET}
      PORT: 8080
      NP_API_URL: https://api.novaposhta.ua/v2.0/json/
      TELEGRAM_URL: ${TELEGRAM_URL}
    depends_on:
      postgres:
        condition: service_healthy
    ports:
      - "127.0.0.1:8080:8080"

volumes:
  postgres_data:
COMPOSEEOF

# ── Build frontends ──────────────────────────────────────────
info "Building Vue web app..."
cd "$APP_DIR/web"
npm ci --silent
npm run build
mkdir -p /var/www/ttnflow/app
rm -rf /var/www/ttnflow/app/*
cp -r dist/* /var/www/ttnflow/app/

info "Building Astro landing..."
cd "$APP_DIR/landing"
npm ci --silent
npm run build
mkdir -p /var/www/ttnflow/landing
rm -rf /var/www/ttnflow/landing/*
# Astro outputs to dist/ by default
cp -r dist/* /var/www/ttnflow/landing/

# ── nginx config ─────────────────────────────────────────────
info "Configuring nginx..."
cat > /etc/nginx/sites-available/ttnflow <<NGINXEOF
server {
    listen 80;
    server_name $DOMAIN www.$DOMAIN;

    # Vue SPA at /app/
    location /app/ {
        root /var/www/ttnflow;
        try_files \$uri \$uri/ /app/index.html;
    }

    # Go API proxy
    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_read_timeout 30s;
    }

    # Astro landing (catch-all)
    location / {
        root /var/www/ttnflow/landing;
        try_files \$uri \$uri/ /index.html;
    }
}
NGINXEOF

ln -sf /etc/nginx/sites-available/ttnflow /etc/nginx/sites-enabled/ttnflow
rm -f /etc/nginx/sites-enabled/default

nginx -t || error "nginx config error"
systemctl enable nginx
systemctl reload nginx

# ── SSL ──────────────────────────────────────────────────────
info "Getting SSL certificate..."
certbot --nginx -d "$DOMAIN" -d "www.$DOMAIN" \
    --non-interactive --agree-tos -m "$EMAIL" --redirect

systemctl reload nginx

# ── Start API ────────────────────────────────────────────────
info "Starting API + PostgreSQL..."
cd "$APP_DIR/backend"
docker compose -f docker-compose.prod.yml up -d --build

# ── Done ─────────────────────────────────────────────────────
echo ""
info "Deploy complete!"
echo ""
echo "  Landing:  https://$DOMAIN/"
echo "  App:      https://$DOMAIN/app/"
echo "  API:      https://$DOMAIN/api/"
echo ""
echo "Logs:   docker compose -f $APP_DIR/backend/docker-compose.prod.yml logs -f"
echo "Update: bash $APP_DIR/backend/update.sh"
