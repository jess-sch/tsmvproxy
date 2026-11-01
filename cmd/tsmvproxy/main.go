package main

import (
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jess-sch/tsmvproxy/internal/tsmvproxy"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var (
	stateDir     string
	serverPrefix string
	localAddress string
	tsProxyPort  uint16
	rootCmd      = &cobra.Command{
		Use: "tsmvproxy",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{})))
			ctx, _ := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			cmd.SetContext(ctx)

		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			proxy, err := tsmvproxy.NewTSMVProxy(ctx, stateDir)
			if err != nil {
				slog.Error(err.Error())
				os.Exit(1)
			}
			defer proxy.Close()
			wg, ctx := errgroup.WithContext(ctx)
			wg.Go(func() error {
				return proxy.WatchPeersAndSelectExit(ctx, serverPrefix)
			})
			if tsProxyPort != 0 {
				wg.Go(func() error {
					return proxy.ListenAndServeTS(ctx, tsProxyPort)
				})
			}
			wg.Go(func() error {
				return proxy.ListenAndServeLocal(ctx, localAddress)
			})
			if err := wg.Wait(); err != nil {
				log.Fatalln(err)
			}
		},
	}
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&serverPrefix, "server", "s", "", "mullvad server name or prefix (e.g. \"ch-zrh\", \"ch-zrh-wg-001\")")
	rootCmd.PersistentFlags().StringVarP(&localAddress, "listen", "l", "[::]:1080", "which port to run the proxy on")
	rootCmd.PersistentFlags().StringVar(&stateDir, "state-dir", "", "tailscale state directory")
	rootCmd.PersistentFlags().Uint16VarP(&tsProxyPort, "ts-port", "p", 0, "if set, also provide the service on this tailscale port")
}
