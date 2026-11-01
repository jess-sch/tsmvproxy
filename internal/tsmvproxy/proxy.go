package tsmvproxy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"strings"

	"tailscale.com/ipn"
	"tailscale.com/tailcfg"
	"tailscale.com/tsnet"
)

const MULLVAD_TAILNET = ".mullvad.ts.net."
const MULLVAD_SOCKS_ENDPOINT = "10.64.0.1:1080"

type TSMVProxy struct {
	srv *tsnet.Server
	ips []netip.Addr
}

func NewTSMVProxy(ctx context.Context, stateDir string) (_ *TSMVProxy, retErr error) {
	srv := &tsnet.Server{
		Hostname: "tsmvproxy",
		AuthKey:  os.Getenv("TS_AUTHKEY"),
		Dir:      stateDir,
	}
	defer func() {
		if retErr != nil {
			defer srv.Close()
		}
	}()
	status, err := srv.Up(ctx)
	if err != nil {
		return nil, err
	}
	return &TSMVProxy{
		srv: srv,
		ips: status.TailscaleIPs,
	}, nil
}

func (p *TSMVProxy) Close() error {
	return p.srv.Close()
}

func (p *TSMVProxy) WatchPeersAndSelectExit(ctx context.Context, prefix string) error {
	lc, err := p.srv.LocalClient()
	if err != nil {
		return err
	}
	watcher, err := lc.WatchIPNBus(ctx, ipn.NotifyPeerChanges|ipn.NotifyInitialStatus)
	if err != nil {
		return err
	}
	defer watcher.Close()

	type peerinfo struct {
		nodeid tailcfg.NodeID
		name   string
		ip     netip.Addr
	}

	peermap := map[tailcfg.NodeID]peerinfo{}
	var lastChoice *tailcfg.NodeID
	for {
		ntf, err := watcher.Next()
		if err != nil {
			return err
		}
		oldCount := len(peermap)
		if ntf.InitialStatus != nil {
			for _, peer := range ntf.InitialStatus.Peer {
				if strings.HasSuffix(peer.DNSName, MULLVAD_TAILNET) && strings.HasPrefix(peer.DNSName, prefix) {
					nodeid := peer.NodeID
					name := peer.HostName
					ip := peer.TailscaleIPs[0]
					peermap[nodeid] = peerinfo{nodeid, name, ip}
					slog.Info("peer changed", "server", name, "online", true)
				}
			}
		}
		for _, peer := range ntf.PeersChanged {
			if strings.HasSuffix(peer.Name, MULLVAD_TAILNET) && strings.HasPrefix(peer.Name, prefix) {
				nodeid := peer.ID
				name := peer.Hostinfo.Hostname()
				ip := peer.Addresses[0].Addr()
				peermap[peer.ID] = peerinfo{nodeid, name, ip}
				slog.Info("peer changed", "server", name, "online", true)
			}
		}
		for _, peer := range ntf.PeersRemoved {
			entry, ok := peermap[peer]
			if ok {
				delete(peermap, peer)
				slog.Info("peer changed", "server", entry.name, "online", false)
			}
		}
		if len(peermap) != oldCount {
			slog.Info("available servers", "count", len(peermap))
		} else if len(peermap) == 0 {
			slog.Warn("No servers found, please ensure this node has a Mullvad Add-On license")
		}
		needsUpdate := lastChoice == nil
		if lastChoice != nil {
			_, stillValid := peermap[*lastChoice]
			needsUpdate = !stillValid
		}
		if needsUpdate {
			for id, peer := range peermap {
				slog.Info("setting exit node", "name", peer.name)
				if err := p.setExitNode(ctx, peer.ip); err != nil {
					slog.Error("failed to set exit node", "err", err.Error())
				}
				lastChoice = &id
				break
			}
		}
	}
}

func (p *TSMVProxy) setExitNode(ctx context.Context, addr netip.Addr) error {
	lc, err := p.srv.LocalClient()
	if err != nil {
		return err
	}
	_, err = lc.EditPrefs(ctx, &ipn.MaskedPrefs{
		Prefs:         ipn.Prefs{ExitNodeIP: addr},
		ExitNodeIPSet: true,
	})
	return err
}

func (p *TSMVProxy) dialSocks(ctx context.Context) (net.Conn, error) {
	return p.srv.Dial(ctx, "tcp", MULLVAD_SOCKS_ENDPOINT)
}

func (p *TSMVProxy) ListenAndServeLocal(ctx context.Context, addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	slog.Info("listening on node", "addr", addr)
	return p.listenAndServe(ctx, l)
}

func (p *TSMVProxy) ListenAndServeTS(ctx context.Context, port uint16) error {
	l, err := p.srv.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	for _, ip := range p.ips {
		slog.Info("listening on tailscale", "addr", netip.AddrPortFrom(ip, port))
	}
	return p.listenAndServe(ctx, l)
}

func (p *TSMVProxy) listenAndServe(ctx context.Context, l net.Listener) error {
	defer l.Close()
	go func() {
		<-ctx.Done()
		l.Close()
	}()
	for {
		c, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return err
			}
			slog.Error(err.Error())
			continue
		}
		go p.serve(ctx, c)
	}
}

func (p *TSMVProxy) serve(ctx context.Context, c net.Conn) {
	defer c.Close()
	rc, err := p.dialSocks(ctx)
	if err != nil {
		slog.Error(err.Error())
		return
	}
	defer rc.Close()
	if err := bidiPipe(c, rc); err != nil {
		slog.Error(err.Error())
	}
}
