# tsmvproxy

A basic, smart and **unofficial** SOCKS5 router for Tailscale + Mullvad Add-On.

## Features
* Node Change Streams - when your current Mullvad server goes offline, it picks another.
* Prefix-based selection - Don't care where in Germany? Set the server name to `de`.
* Local listener - make the Mullvad SOCKS5 proxy available via localhost.
* Tailscale listener - make the Mullvad SOCKS5 proxy available to other nodes in the tailnet, without setting Mullvad exit nodes on them
* Userspace only, no networking funkiness to worry about

```
Usage:
  tsmvproxy [flags]

Flags:
  -h, --help               help for tsmvproxy
  -l, --listen string      which port to run the proxy on (default "[::]:1080")
  -s, --server string      mullvad server name or prefix (e.g. "ch-zrh", "ch-zrh-wg-001")
      --state-dir string   tailscale state directory
  -p, --ts-port uint16     if set, also provide the service on this tailscale port
```

## Why not just the official Tailscale container?

* It requires you to set a specific exit node. If you pick one and it goes offline, manual intervention is required.
* In certain conditions, I have observed the exit node turning itself off after a restart.
* Since we're TCP proxying Mullvad's SOCKS5 proxy rather than providing our own SOCKS5 proxy while using a Mullvad exit node, we basically get a "kill switch" for free. 

## Legal Disclaimers

**This project is not affiliated, associated, authorized, endorsed by, or in any way officially connected with Tailscale Inc. or Mullvad VPN AB.**

Also, it's a side project cooked up in an afternoon to solve a problem I had. Absolutely zero warranty. If it breaks, I'll fix it if/when I get around to it.