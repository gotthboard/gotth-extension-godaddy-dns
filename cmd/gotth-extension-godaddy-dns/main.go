package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/gotthboard/gotth-extension-godaddy-dns/internal/godaddydns"
	dnsv1 "github.com/gotthboard/gotth-extension-godaddy-dns/proto/gotth/dns/provider/v1"
	extensionsv1 "github.com/gotthboard/gotth-extension-godaddy-dns/proto/gotth/extensions/v1"
	"google.golang.org/grpc"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println(godaddydns.BuildVersion)
		return
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gotth-extension-godaddy-dns: startup failed")
		os.Exit(1)
	}
}

func run() error {
	runtime, err := godaddydns.LoadRuntimeFromEnv()
	if err != nil {
		return err
	}
	service, err := godaddydns.NewService(runtime)
	clear(runtime.PAT)
	if err != nil {
		return err
	}
	defer service.Close()
	listener, err := net.Listen("unix", runtime.SocketPath)
	if err != nil {
		return err
	}
	defer listener.Close()
	if err := os.Chmod(runtime.SocketPath, 0o600); err != nil {
		return err
	}
	server := grpc.NewServer()
	extensionsv1.RegisterExtensionControlServer(server, service)
	dnsv1.RegisterDNSRecordsServer(server, service)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() { <-signals; server.GracefulStop() }()
	if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return err
	}
	return nil
}
