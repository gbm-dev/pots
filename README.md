# POTS - OOB Console Hub

Dockerized out-of-band console access over PSTN. Admins SSH in, pick a site from a modern TUI menu, and get dropped into a live modem session routed through D-Modem + Telnyx SIP.

Built with Go using the [Charm](https://charm.sh) ecosystem (Wish + Bubble Tea + Lip Gloss) for a single-binary SSH server with native modem handling and built-in user management.

## Quick Start

```bash
git clone https://github.com/gbm-dev/pots.git /opt/pots
cd /opt/pots
sudo bash scripts/host-setup.sh
```

Edit credentials and sites:

```bash
nano .env                       # Telnyx SIP creds + outbound caller ID
nano config/oob-sites.conf      # Remote sites
```

Build and start:

```bash
docker compose build
sudo systemctl start oob-hub
sudo systemctl start oob-watchdog.timer
```

## Updating

Pull latest code and rebuild image (only when code/image changes):

```bash
cd /opt/pots
git pull
docker compose build
sudo systemctl reload oob-hub
```

Pin a specific release version in the build:

```bash
git checkout v1.0.0
docker compose build
```

Go binaries are compiled from local source during `docker compose build`.

## Site Configuration

Edit `config/oob-sites.conf` — one line per remote device:

```
# name|phone_number|description|baud_rate
2broadway|14105551234|2 Broadway Terminal Server|19200
router1|13125559876|Chicago Core Router|9600
```

The phone number is the PSTN line connected to the modem/console server at the remote site.

## Modem Backend

This image is `dmodem`-only (no Asterisk/IAX runtime path).

For `dmodem`, `SIP_LOGIN` is generated from `TELNYX_SIP_USER`, `TELNYX_SIP_PASS`, and `TELNYX_SIP_DOMAIN`.
`DMODEM_AT_MS` controls the modulation command sent during modem init (default: `AT+MS=132,0,4800,9600`).
`MODEM_DIAL_PREFIX` controls the outbound AT dial command prefix (default: `ATD`).

## Faster Rebuilds

You can prebuild D-Modem binaries once and reuse them in future image builds:

```bash
./scripts/build-dmodem-artifacts.sh
echo "DMODEM_SOURCE=prebuilt" >> .env
docker compose build
```

The binaries are written to `third_party/dmodem/`.
The bundled build defaults to PJSIP epoll mode (`--enable-epoll`) for better stability.

For normal restarts/deploys after that, no rebuild is needed:

```bash
sudo systemctl restart oob-hub
```

## User Management

Run from the host (wrapper delegates to container):

```bash
oob-user-manage add first.last       # Create user (prints temp password)
oob-user-manage list                  # Show all users + status
oob-user-manage reset first.last      # New temp password
oob-user-manage lock first.last       # Disable account
oob-user-manage unlock first.last     # Re-enable account
oob-user-manage remove first.last     # Delete account
```

Or directly inside the container:

```bash
docker exec oob-console-hub oob-manage add first.last
docker exec oob-console-hub oob-manage list
```

Users get a temporary password and must change it on first login. The SSH server drops them directly into the TUI — no shell access.

## Connecting

```bash
ssh first.last@<server-ip> -p 2222
```

Select a site from the menu, auto-dials via modem, live session begins. Press Enter then `~.` to disconnect (same as SSH escape). Session logs are saved to `logs/`.

## Monitoring

```bash
systemctl status oob-hub
systemctl status oob-watchdog.timer
journalctl -u oob-watchdog -f
docker exec oob-console-hub oob-healthcheck.sh --verbose
```

The watchdog checks health every 2 minutes and auto-restarts on critical failures (max 3/hour).

## Architecture

```
Admin SSH (:2222) → Wish/Bubble Tea TUI → free tty device → D-Modem (`slmodemd` + `d-modem`) → Telnyx SIP/PSTN → Remote Device
```

- **oob-hub**: Go binary — Wish SSH server + Bubble Tea TUI + modem pool + user store
- **oob-manage**: Go binary — CLI for user management (add/remove/list/lock/unlock/reset)
- **D-Modem**: Default software modem backend (`slmodemd` + `d-modem`) exposing `/dev/ttyIAX0-7` symlinks.
- **User store**: `users.json` with bcrypt hashing, atomic writes, file locking
- **systemd**: `oob-hub.service` + `oob-watchdog.timer`

## Development

```bash
go test ./internal/...    # Run all unit tests
go build ./cmd/...        # Build binaries
```
