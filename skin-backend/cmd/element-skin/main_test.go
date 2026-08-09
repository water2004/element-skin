package main

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"element-skin/backend/internal/config"
)

type fakeApplication struct {
	handler    http.Handler
	closeCalls int
}

func (a *fakeApplication) Handler() http.Handler { return a.handler }

func (a *fakeApplication) Close() { a.closeCalls++ }

func TestNewHTTPServerUsesConfiguredAddressHandlerAndTimeout(t *testing.T) {
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	cfg := config.Config{ServerHost: "127.0.0.1", ServerPort: "18080"}

	server := newHTTPServer(cfg, handler)

	if server.Addr != "127.0.0.1:18080" {
		t.Fatalf("server address should come from config, got %q", server.Addr)
	}
	if server.Handler == nil {
		t.Fatal("server handler should be installed")
	}
	if server.ReadHeaderTimeout != 10*time.Second {
		t.Fatalf("unexpected read header timeout: %s", server.ReadHeaderTimeout)
	}
	if server.ReadTimeout != 30*time.Second {
		t.Fatalf("unexpected read timeout: %s", server.ReadTimeout)
	}
	if server.WriteTimeout != 30*time.Second {
		t.Fatalf("unexpected write timeout: %s", server.WriteTimeout)
	}
	if server.IdleTimeout != 120*time.Second {
		t.Fatalf("unexpected idle timeout: %s", server.IdleTimeout)
	}
	if server.MaxHeaderBytes != 1<<20 {
		t.Fatalf("unexpected max header bytes: %d", server.MaxHeaderBytes)
	}
}

func TestRunServerPropagatesStartupAndListenerFailuresAndClosesApplicationExactly(t *testing.T) {
	wantStartupErr := errors.New("application startup failed")
	startupRuntime := serverRuntime{
		newApplication: func(context.Context, config.Config) (application, error) { return nil, wantStartupErr },
		listen: func(*http.Server) error {
			t.Fatal("listener should not start after application failure")
			return nil
		},
		shutdown: func(*http.Server, context.Context) error {
			t.Fatal("shutdown should not run after application failure")
			return nil
		},
	}
	if err := runServer(t.Context(), config.Config{}, startupRuntime); !errors.Is(err, wantStartupErr) {
		t.Fatalf("startup error=%v", err)
	}

	wantListenerErr := errors.New("HTTP listener failed")
	appInstance := &fakeApplication{handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})}
	listenerRuntime := serverRuntime{
		newApplication: func(context.Context, config.Config) (application, error) { return appInstance, nil },
		listen:         func(*http.Server) error { return wantListenerErr },
		shutdown: func(*http.Server, context.Context) error {
			t.Fatal("shutdown should not run after listener failure")
			return nil
		},
	}
	if err := runServer(t.Context(), config.Config{}, listenerRuntime); !errors.Is(err, wantListenerErr) {
		t.Fatalf("listener error=%v", err)
	}
	if appInstance.closeCalls != 1 {
		t.Fatalf("listener failure application close calls=%d want=1", appInstance.closeCalls)
	}

	closedApplication := &fakeApplication{handler: appInstance.handler}
	closedRuntime := listenerRuntime
	closedRuntime.newApplication = func(context.Context, config.Config) (application, error) { return closedApplication, nil }
	closedRuntime.listen = func(*http.Server) error { return http.ErrServerClosed }
	if err := runServer(t.Context(), config.Config{}, closedRuntime); err != nil {
		t.Fatalf("normal server close error=%v", err)
	}
	if closedApplication.closeCalls != 1 {
		t.Fatalf("normal close application calls=%d want=1", closedApplication.closeCalls)
	}
}

func TestRunServerCancellationUsesConfiguredServerAndReturnsShutdownResultExactly(t *testing.T) {
	for _, tc := range []struct {
		name        string
		shutdownErr error
	}{
		{name: "successful shutdown"},
		{name: "failed shutdown", shutdownErr: errors.New("HTTP shutdown failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			handler := http.NewServeMux()
			appInstance := &fakeApplication{handler: handler}
			listenerRelease := make(chan struct{})
			var shutdownCalls int
			runtime := serverRuntime{
				newApplication: func(context.Context, config.Config) (application, error) { return appInstance, nil },
				listen: func(*http.Server) error {
					<-listenerRelease
					return http.ErrServerClosed
				},
				shutdown: func(server *http.Server, shutdownContext context.Context) error {
					shutdownCalls++
					close(listenerRelease)
					if server.Addr != "127.0.0.1:18081" || server.Handler != handler || shutdownContext.Err() != nil {
						t.Fatalf("shutdown server=%#v context_error=%v", server, shutdownContext.Err())
					}
					return tc.shutdownErr
				},
			}
			err := runServer(ctx, config.Config{ServerHost: "127.0.0.1", ServerPort: "18081"}, runtime)
			if !errors.Is(err, tc.shutdownErr) || shutdownCalls != 1 || appInstance.closeCalls != 1 {
				t.Fatalf("shutdown err=%v calls=%d app_closes=%d", err, shutdownCalls, appInstance.closeCalls)
			}
		})
	}
}
