# habit-tracker

Personal habit & routine tracker. Runs on your home server, data stays local, reachable from your phone over Tailscale.

- **Stack**: Go + SQLite + HTMX. Single static binary, ~18MB, ~20MB RAM at idle.
- **Privacy**: no telemetry, no CDN, no outbound HTTP. HTMX is vendored under `web/static/`.
- **Auth**: none — relies on the tracker being reachable only on your tailnet.

## Run locally

```sh
go build -o ./bin/server ./cmd/server
DB_PATH=./tracker.db ADDR=:8080 TZ=America/Los_Angeles ./bin/server
```

Then open http://localhost:8080.

Tests:

```sh
go test ./...
```

## Deploy on the home server

Prereqs: Docker + Docker Compose + Tailscale installed and connected on the host.

1. Find your host's tailnet IPv4: `tailscale ip -4` (e.g. `100.x.y.z`).
2. Edit `docker-compose.yml`:
   - Replace `100.x.y.z` in the Caddy `ports:` mapping with the value above.
   - Set `TZ` on the `tracker` service to your timezone.
3. Build and run:
   ```sh
   docker compose up -d --build
   docker compose logs -f tracker
   ```
4. Reach the app from any tailnet device at `http://<host-tailscale-name>/` — for example from your phone with the Tailscale app on.

### Why bind Caddy to the tailnet IP

Binding `100.x.y.z:80:80` rather than `80:80` means the host's port 80 is only listening on the Tailscale interface. The tracker is invisible on your LAN and the public internet, with no firewall rules required. Tailscale handles encryption end-to-end, so plain HTTP inside the tailnet is fine.

If you later want the URL bar to show HTTPS, run Tailscale as a sidecar with the [`caddy-tailscale`](https://github.com/tailscale/caddy-tailscale) plugin and use a `*.ts.net` hostname.

## Adding more side projects to the same Caddy

When you add the next service to this Compose file, give it a block in the `Caddyfile`:

```
:80 {
    @habits host habits.tail-xxxxx.ts.net
    handle @habits {
        reverse_proxy tracker:8080
    }

    @other host other.tail-xxxxx.ts.net
    handle @other {
        reverse_proxy other-app:3000
    }
}
```

(Or use one Caddy site per port if you'd rather not bother with hostnames yet.)

## Backup

Everything is in the `tracker-data` Docker volume. Back it up at the host level:

```sh
docker run --rm -v habit-tracker_tracker-data:/data -v "$PWD":/backup alpine \
    tar czf /backup/tracker-$(date +%F).tar.gz -C /data .
```

Or just `restic`/`rsync` the volume directory.

## Layout

```
cmd/server/      entrypoint
internal/store/  SQLite + business logic + tests
internal/httpx/  chi router, handlers, template wiring
internal/migrations/  embedded *.sql + apply-on-startup
web/             package web — embeds templates/ and static/
```

The `httpx` package is named to avoid colliding with stdlib `net/http`.
