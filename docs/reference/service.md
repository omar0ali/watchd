# Running as a service

For development, you don't need to set anything up: the daemon starts automatically when
you need it.

To keep projects running across reboots, or on a small server, run the daemon under your
system's service manager with `watchd daemon`, which runs it in the foreground.

## systemd (Linux)

Create a **user** service. No root is needed, and apps run as you:

```ini
# ~/.config/systemd/user/watchd.service
[Unit]
Description=watchd process supervisor

[Service]
ExecStart=%h/.local/bin/watchd daemon
Restart=on-failure

[Install]
WantedBy=default.target
```

Adjust the `ExecStart` path to wherever `command -v watchd` says watchd is installed, then:

```sh
systemctl --user daemon-reload
systemctl --user enable --now watchd
loginctl enable-linger "$USER"     # keep it running when you're logged out
```

Check it with `systemctl --user status watchd`, or with `journalctl --user -u watchd`, which
shows the daemon's log.

### Starting projects with the service

Projects aren't remembered across daemon restarts yet, so start them after the daemon is
up. Add one `ExecStartPost=` line per project:

```ini
[Service]
ExecStart=%h/.local/bin/watchd daemon
ExecStartPost=%h/.local/bin/watchd up -c %h/work/shop/watchd.lua
ExecStartPost=%h/.local/bin/watchd up -c %h/work/blog/watchd.lua
Restart=on-failure
```

::: tip The environment under systemd
Apps get the environment of whoever runs `watchd up`. Under systemd, that's the service's
environment, which has a minimal `PATH`. Set what your apps need in the unit
(`Environment=PATH=...`) or in the project's `env`/`env_file`.
:::

## launchd (macOS)

```xml
<!-- ~/Library/LaunchAgents/com.github.omar0ali.watchd.plist -->
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.github.omar0ali.watchd</string>
  <key>ProgramArguments</key>
  <array>
    <string>/usr/local/bin/watchd</string>
    <string>daemon</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
</dict>
</plist>
```

```sh
launchctl load ~/Library/LaunchAgents/com.github.omar0ali.watchd.plist
```

Then run `watchd up` in your projects, or from a login script.
