package spectrum

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"github.com/cooldogedev/spectrum/server"
	"github.com/cooldogedev/spectrum/session"
	tr "github.com/cooldogedev/spectrum/transport"
	"github.com/cooldogedev/spectrum/util"
	"github.com/sandertv/gophertunnel/minecraft"
)

// Spectrum represents a proxy server managing server discovery,
// network transport, and connections through an underlying minecraft.Listener.
type Spectrum struct {
	discovery server.Discovery
	transport tr.Transport

	listener  *minecraft.Listener
	listeners []*minecraft.Listener
	accepted  chan acceptResult
	closeOnce sync.Once
	closed    chan struct{}
	registry  *session.Registry

	logger *slog.Logger
	opts   util.Opts
}

// NewSpectrum creates a new Spectrum instance using the provided server.Discovery.
// It initializes opts with default options from util.DefaultOpts() if opts is nil,
// and defaults to TCP transport if transport is nil transport.TCP.
func NewSpectrum(discovery server.Discovery, logger *slog.Logger, opts *util.Opts, transport tr.Transport) *Spectrum {
	if opts == nil {
		opts = util.DefaultOpts()
	}

	if transport == nil {
		transport = tr.NewSpectral(logger)
	}
	return &Spectrum{
		discovery: discovery,
		transport: transport,

		accepted: make(chan acceptResult),
		closed:   make(chan struct{}),
		registry: session.NewRegistry(),

		logger: logger,
		opts:   *opts,
	}
}

// acceptResult carries a connection accepted by one of the listeners, or the error that ended it.
type acceptResult struct {
	conn net.Conn
	err  error
}

// Listen sets up a minecraft.Listener for incoming connections based on the provided minecraft.ListenConfig.
// The listener is then used by the Accept() method for accepting incoming connections.
func (s *Spectrum) Listen(config minecraft.ListenConfig) (err error) {
	listener, err := config.Listen("raknet", s.opts.Addr)
	if err != nil {
		s.logger.Error("failed to listen", "err", err)
		return err
	}
	s.addListener(listener)
	s.logger.Info("started listening", "addr", listener.Addr())
	return nil
}

// ListenNetwork sets up an additional minecraft.Listener on the given minecraft.Network, such as a
// NetherNet network, next to the RakNet listener created by Listen. Connections accepted by every
// listener are handed out by Accept in the order they arrive.
func (s *Spectrum) ListenNetwork(config minecraft.ListenConfig, network minecraft.Network, address string) (err error) {
	listener, err := config.ListenNetwork(network, address)
	if err != nil {
		s.logger.Error("failed to listen", "network", fmt.Sprintf("%T", network), "err", err)
		return err
	}
	s.addListener(listener)
	s.logger.Info("started listening", "network", fmt.Sprintf("%T", network), "addr", listener.Addr())
	return nil
}

// addListener registers the listener and starts forwarding the connections it accepts to Accept. The first
// listener registered is the one returned by Listener.
func (s *Spectrum) addListener(listener *minecraft.Listener) {
	if s.listener == nil {
		s.listener = listener
	}
	s.listeners = append(s.listeners, listener)
	go func() {
		for {
			c, err := listener.Accept()
			select {
			case s.accepted <- acceptResult{conn: c, err: err}:
			case <-s.closed:
				return
			}
			if err != nil {
				return
			}
		}
	}()
}

// Accept accepts an incoming minecraft.Conn and creates a new session for it.
// This method should be called in a loop to continuously accept new connections.
func (s *Spectrum) Accept() (*session.Session, error) {
	var result acceptResult
	select {
	case result = <-s.accepted:
	case <-s.closed:
		return nil, net.ErrClosed
	}
	if result.err != nil {
		s.logger.Error("failed to accept session", "err", result.err)
		return nil, result.err
	}

	conn := result.conn.(*minecraft.Conn)
	identityData := conn.IdentityData()
	logger := s.logger.With("username", identityData.DisplayName)
	newSession := session.NewSession(conn, logger, s.registry, s.discovery, s.opts, s.transport)
	if s.opts.AutoLogin {
		go func() {
			if err := newSession.Login(); err != nil {
				newSession.Disconnect(err.Error())
				if !errors.Is(err, context.Canceled) {
					logger.Error("failed to login session", "err", err)
				}
			}
		}()
	}
	logger.Info("accepted session")
	return newSession, nil
}

// Discovery returns the server discovery instance.
func (s *Spectrum) Discovery() server.Discovery {
	return s.discovery
}

// Opts returns the configuration options.
func (s *Spectrum) Opts() util.Opts {
	return s.opts
}

// Listener returns the first listener instance, the RakNet one when Listen was called first.
func (s *Spectrum) Listener() *minecraft.Listener {
	return s.listener
}

// Listeners returns every listener instance, in the order they were registered.
func (s *Spectrum) Listeners() []*minecraft.Listener {
	return s.listeners
}

// Registry returns the session registry instance.
func (s *Spectrum) Registry() *session.Registry {
	return s.registry
}

// Transport returns the transport instance.
func (s *Spectrum) Transport() tr.Transport {
	return s.transport
}

// Close closes every listener and stops listening for incoming connections.
func (s *Spectrum) Close() (err error) {
	for _, activeSession := range s.registry.GetSessions() {
		activeSession.Disconnect(s.opts.ShutdownMessage)
	}
	s.closeOnce.Do(func() { close(s.closed) })
	for _, listener := range s.listeners {
		if closeErr := listener.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
}
