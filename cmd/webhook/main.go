package main

import (
	"context"
	"crypto/tls"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/pladd/token-lifetime-filtering-webhook/pkg/policy"
	"github.com/pladd/token-lifetime-filtering-webhook/pkg/policy/informer"
	"github.com/pladd/token-lifetime-filtering-webhook/pkg/webhook"
)

func main() {
	var (
		listenAddr = flag.String("listen", ":8443", "HTTPS listen address")
		tlsCert    = flag.String("tls-cert", "/tls/tls.crt", "Path to TLS certificate")
		tlsKey     = flag.String("tls-key", "/tls/tls.key", "Path to TLS private key")
		kubeconfig = flag.String("kubeconfig", "", "Path to kubeconfig (optional; in-cluster config used if empty)")
	)
	klog.InitFlags(nil)
	flag.Parse()

	cfg, err := buildConfig(*kubeconfig)
	if err != nil {
		klog.Fatalf("failed to build kubeconfig: %v", err)
	}

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("failed to create dynamic client: %v", err)
	}

	store := policy.NewStore()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	hasSynced, err := informer.Run(ctx, dyn, store)
	if err != nil {
		klog.Fatalf("failed to start policy informer: %v", err)
	}

	go waitForSync(ctx, hasSynced)

	handler := &webhook.Handler{
		Store: store,
		Ready: func() bool { return hasSynced() },
	}

	server := &http.Server{
		Addr:              *listenAddr,
		Handler:           handler.NewMux(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	klog.Infof("starting token-lifetime webhook on %s", *listenAddr)
	if err := server.ListenAndServeTLS(*tlsCert, *tlsKey); err != nil && err != http.ErrServerClosed {
		klog.Fatalf("server failed: %v", err)
	}
}

func buildConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	if env := os.Getenv("KUBECONFIG"); env != "" {
		return clientcmd.BuildConfigFromFlags("", env)
	}
	return rest.InClusterConfig()
}

func waitForSync(ctx context.Context, hasSynced cache.InformerSynced) {
	klog.Info("waiting for policy informer sync")
	if !cache.WaitForCacheSync(ctx.Done(), hasSynced) {
		klog.Error("timed out waiting for policy informer sync")
		return
	}
	klog.Info("policy informer synced")
}
