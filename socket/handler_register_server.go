package socket

import (
	"github.com/paroxity/portal/server"
	"github.com/paroxity/portal/session"
	"github.com/paroxity/portal/socket/packet"
)

// RegisterServerHandler is responsible for handling the RegisterServer packet sent by servers.
type RegisterServerHandler struct{ requireAuth }

// Handle ...
func (*RegisterServerHandler) Handle(p packet.Packet, srv Server, c *Client) error {
	pk := p.(*packet.RegisterServer)
	reg := server.New(c.Name(), pk.Address)
	srv.ServerRegistry().AddServer(reg)
	srv.Logger().Debugf("socket connection \"%s\" has registered itself as a server with the address \"%s\"", c.Name(), pk.Address)

	session.HibernatersMu.Lock()
	defer session.HibernatersMu.Unlock()
	for s := range session.Hibernaters {
		s.Transfer(reg)
	}
	session.Hibernaters = make(map[*session.Session]struct{})

	return nil
}
