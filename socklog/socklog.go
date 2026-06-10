package socklog

import (
	"fmt"
	"net"
	"strings"

	"github.com/asciimoth/gonnect-vpn-example/logger"
	"github.com/asciimoth/gonnect/sockowner"
)

func LogIncomingConnOwner(log logger.Logger, label string, conn net.Conn) {
	if log == nil {
		return
	}

	owner, err := sockowner.GetIncomingConnOwner(conn)
	if err != nil {
		log.Printf(
			"%s sockowner unavailable local=%s remote=%s: %v",
			label,
			addrString(conn.LocalAddr()),
			addrString(conn.RemoteAddr()),
			err,
		)
		return
	}

	log.Printf(
		"%s sockowner local=%s remote=%s owner=%s",
		label,
		addrString(conn.LocalAddr()),
		addrString(conn.RemoteAddr()),
		FormatOwner(owner),
	)
}

func LogOutgoingPacketOwner(log logger.Logger, label string, packet []byte) {
	if log == nil {
		return
	}

	flow, err := sockowner.FlowTupleFromOutgoingIPPacket(packet)
	if err != nil {
		log.Printf("%s sockowner unavailable: %v", label, err)
		return
	}

	owner, err := sockowner.GetSockOwner(flow)
	if err != nil {
		log.Printf(
			"%s sockowner unavailable flow=%s: %v",
			label,
			formatFlow(flow),
			err,
		)
		return
	}

	log.Printf(
		"%s sockowner flow=%s owner=%s",
		label,
		formatFlow(flow),
		FormatOwner(owner),
	)
}

func FormatOwner(owner *sockowner.SocketOwner) string {
	if owner == nil {
		return "<nil>"
	}

	parts := make([]string, 0, 5)
	if len(owner.PIDs) > 0 {
		pids := make([]string, 0, len(owner.PIDs))
		for _, pid := range owner.PIDs {
			pids = append(pids, fmt.Sprint(pid))
		}
		parts = append(parts, "pid="+strings.Join(pids, ","))
	}
	if owner.UID != nil {
		parts = append(parts, fmt.Sprintf("uid=%d", *owner.UID))
	}
	if owner.GID != nil {
		parts = append(parts, fmt.Sprintf("gid=%d", *owner.GID))
	}
	if owner.Comm != "" {
		parts = append(parts, "comm="+owner.Comm)
	}
	if owner.ProcName != "" {
		parts = append(parts, "proc="+owner.ProcName)
	}
	if len(parts) == 0 {
		return "<unknown>"
	}
	return strings.Join(parts, " ")
}

func formatFlow(flow sockowner.FlowTuple) string {
	return fmt.Sprintf(
		"%s %s:%d -> %s:%d",
		flow.Proto,
		flow.LocalIP,
		flow.LocalPort,
		flow.RemoteIP,
		flow.RemotePort,
	)
}

func addrString(addr net.Addr) string {
	if addr == nil {
		return "<nil>"
	}
	return addr.String()
}
